package singledb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// TransactionRepo implements repository.TransactionRepository for single-DB mode.
type TransactionRepo struct {
	db *pgxpool.Pool
}

// NewTransactionRepo creates a new TransactionRepo backed by the shared pool.
func NewTransactionRepo(db *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{db: db}
}

// ExecuteSaleTx executes the entire SALE flow inside a postgres transaction.
// It uses a Closure pattern (UpdateFn) coupled with Bulk Querying and Bulk Pipelining (pgx.Batch)
func (r *TransactionRepo) ExecuteSaleTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateSaleRequest,
	processFn func(loadedItems map[int]model.ProcessSaleItem) (model.FinalSaleAggregate, error),
) (*model.Transaction, error) {

	// Start DB Transaction
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. PHASE ONE: BULK READ (Avoiding N+1)
	branchItemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		branchItemIDs = append(branchItemIDs, item.BranchItemID)
	}

	loadedDbItems := make(map[int]model.ProcessSaleItem)

	// Single query to lookup all branch items and their master pricing info!
	query := `
		SELECT b.id, b.stock, i.cost, i.price 
		FROM branch_items b
		JOIN items i ON b.item_id = i.id
		WHERE b.id = ANY($1) AND b.branch_id = $2 AND b.tenant_id = $3 
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, branchItemIDs, req.BranchID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk lookup branch items: %w", err)
	}

	for rows.Next() {
		var (
			bID   int
			stock int
			cost  float64
			price float64
		)
		if err := rows.Scan(&bID, &stock, &cost, &price); err != nil {
			rows.Close() // ALWAYS close early if manually returning
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		loadedDbItems[bID] = model.ProcessSaleItem{
			BranchItemID: bID,
			AvailableQty: stock,
			COGS:         cost,
			Price:        price,
		}
	}
	rows.Close()

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC CLOSURE
	finalAggregate, err := processFn(loadedDbItems)
	if err != nil {
		return nil, err
	}

	// 3. PHASE THREE: WRITE & PIPELINING (Avoiding N+1 Insert & Update)
	trxNo := fmt.Sprintf("SALE-%s", time.Now().Format("20060102150405"))

	// Insert Header `transactions`
	var trxID int
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (tenant_id, trxno, branch_id, customer_id, user_id, trans_type, total_amount, tax, discount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		tenantID, trxNo, req.BranchID, req.CustomerID, userID, model.TxSale, finalAggregate.TotalAmount, req.Tax, req.Discount, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert transaction header: %w", err)
	}

	// Start pipelining! Prepare a massive block of commands.
	batch := &pgx.Batch{}

	// Insert Details & Record Stock deductions in memory buffer first (Batching)
	for _, detail := range finalAggregate.Details {
		batch.Queue(
			`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			trxID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
		)

		batch.Queue(
			`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			detail.Quantity, *detail.BranchItemID, tenantID,
		)
	}

	// Insert the branch Cashflow
	batch.Queue(
		`INSERT INTO branch_cashflow (tenant_id, branch_id, transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tenantID, req.BranchID, trxID, model.CflowSale, "IN", finalAggregate.TotalAmount,
	)

	// Send everything over the network to PostgreSQL IN ONE GO! 🏎️
	batchResults := tx.SendBatch(ctx, batch)

	// Read back execution responses to ensure NO errors matched our queue length
	for i := 0; i < batch.Len(); i++ {
		_, err := batchResults.Exec()
		if err != nil {
			batchResults.Close()
			return nil, fmt.Errorf("bulk execution failed at pipelined query %d: %w", i, err)
		}
	}
	batchResults.Close() // Mandatory cleanup

	// Commit Transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Since we used Batch to insert details, we don't have detail DB IDs generated back internally here.
	// But usually, frontends DO NOT need the newly generated transaction_detail.id upon a successful checkout,
	// so returning the generic detail breakdown is perfectly acceptable.
	branchIDProxy := req.BranchID
	return &model.Transaction{
		ID:          trxID,
		TenantID:    tenantID,
		TrxNo:       trxNo,
		BranchID:    &branchIDProxy,
		CustomerID:  req.CustomerID,
		UserID:      &userID,
		TransType:   model.TxSale,
		TotalAmount: finalAggregate.TotalAmount,
		Tax:         req.Tax,
		Discount:    req.Discount,
		Note:        req.Note,
		CreatedAt:   time.Now(),
		Details:     finalAggregate.Details,
	}, nil
}

// ExecutePurchaseTx handles the DB transaction and coordination for PURCHASES.
func (r *TransactionRepo) ExecutePurchaseTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreatePurchaseRequest,
	processFn func(loadedItems map[int]model.ProcessPurchaseItem) (model.FinalPurchaseAggregate, error),
) (*model.Transaction, error) {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. PHASE ONE: BULK READ & GLOBAL STOCK CALCULATION
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	loadedDbItems := make(map[int]model.ProcessPurchaseItem)

	// A. Lock and Load Master Items (for current cost)
	rows, err := tx.Query(ctx, `SELECT id, cost FROM items WHERE id = ANY($1) AND tenant_id = $2 FOR UPDATE`, itemIDs, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock master items: %w", err)
	}
	for rows.Next() {
		var id int
		var cost float64
		if err := rows.Scan(&id, &cost); err != nil {
			rows.Close()
			return nil, err
		}
		loadedDbItems[id] = model.ProcessPurchaseItem{ItemID: id, ExistingCost: cost, GlobalStock: 0}
	}
	rows.Close()

	// B. Calculate Global Stock (Branch + Warehouse)
	branchRows, err := tx.Query(ctx, `SELECT item_id, SUM(stock) FROM branch_items WHERE item_id = ANY($1) AND tenant_id = $2 GROUP BY item_id`, itemIDs, tenantID)
	if err != nil {
		return nil, err
	}
	for branchRows.Next() {
		var id, stock int
		if err := branchRows.Scan(&id, &stock); err != nil {
			branchRows.Close()
			return nil, err
		}
		item := loadedDbItems[id]
		item.GlobalStock += stock
		loadedDbItems[id] = item
	}
	branchRows.Close()

	whRows, err := tx.Query(ctx, `SELECT item_id, SUM(stock) FROM warehouse_items WHERE item_id = ANY($1) AND tenant_id = $2 GROUP BY item_id`, itemIDs, tenantID)
	if err != nil {
		return nil, err
	}
	for whRows.Next() {
		var id, stock int
		if err := whRows.Scan(&id, &stock); err != nil {
			whRows.Close()
			return nil, err
		}
		item := loadedDbItems[id]
		item.GlobalStock += stock
		loadedDbItems[id] = item
	}
	whRows.Close()

	// C. Resolve Target Location IDs (Upsert records so we have IDs for transaction_detail)
	locationItemIDs := make(map[int]int) // item_id -> branch_item_id or warehouse_item_id
	for _, id := range itemIDs {
		var locID int
		if req.BranchID != nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO branch_items (tenant_id, branch_id, item_id, stock)
				VALUES ($1, $2, $3, 0)
				ON CONFLICT (tenant_id, branch_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id`,
				tenantID, *req.BranchID, id,
			).Scan(&locID)
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO warehouse_items (tenant_id, warehouse_id, item_id, stock)
				VALUES ($1, $2, $3, 0)
				ON CONFLICT (tenant_id, warehouse_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id`,
				tenantID, *req.WarehouseID, id,
			).Scan(&locID)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to ensure location item record: %w", err)
		}
		locationItemIDs[id] = locID
	}

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC
	finalAggregate, err := processFn(loadedDbItems)
	if err != nil {
		return nil, err
	}

	// 3. PHASE THREE: BULK WRITE
	trxNo := fmt.Sprintf("PURC-%s", time.Now().Format("20060102150405"))

	// Insert Header (Explicitly ensure mutual exclusivity for safety)
	var finalBranchID, finalWarehouseID *int
	if req.BranchID != nil {
		finalBranchID = req.BranchID
	} else {
		finalWarehouseID = req.WarehouseID
	}

	var trxID int
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (tenant_id, trxno, branch_id, warehouse_id, supplier_id, user_id, trans_type, total_amount, tax, discount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		tenantID, trxNo, finalBranchID, finalWarehouseID, req.SupplierID, userID, model.TxPurchase, finalAggregate.TotalAmount, req.Tax, req.Discount, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert purchase header: %w", err)
	}

	batch := &pgx.Batch{}

	// A. Update Master Costs
	for itemID, newCost := range finalAggregate.NewCosts {
		batch.Queue(`UPDATE items SET cost = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`, newCost, itemID, tenantID)
	}

	// B. Batch Insert Details and Update Stocks
	for _, detail := range finalAggregate.Details {
		// Because we resolved locationItemIDs in Phase 1, we can now use them.
		
		var locID int
		if req.BranchID != nil {
			locID = locationItemIDs[*detail.BranchItemID] // Service stored item_id here
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, locID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
				detail.Quantity, locID, tenantID,
			)
		} else {
			locID = locationItemIDs[*detail.WarehouseItemID]
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, warehouse_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, locID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
				detail.Quantity, locID, tenantID,
			)
		}
	}

	// C. Insert Tenant Cashflow (OUT)
	batch.Queue(
		`INSERT INTO tenant_cashflow (tenant_id, transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4, $5)`,
		tenantID, trxID, model.CflowPurch, "OUT", finalAggregate.TotalAmount,
	)

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return nil, fmt.Errorf("failed to execute batch at index %d: %w", i, err)
		}
	}
	br.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit purchase transaction: %w", err)
	}

	return &model.Transaction{
		ID:          trxID,
		TrxNo:       trxNo,
		TenantID:    tenantID,
		BranchID:    req.BranchID,
		WarehouseID: req.WarehouseID,
		SupplierID:  &req.SupplierID,
		UserID:      &userID,
		TransType:   model.TxPurchase,
		TotalAmount: finalAggregate.TotalAmount,
		Tax:         req.Tax,
		Discount:    req.Discount,
		Note:        req.Note,
		CreatedAt:   time.Now(),
		Details:     finalAggregate.Details,
	}, nil
}

// ExecuteTransferTx handles the DB transaction for OMNI-DIRECTIONAL transfer (Double-Entry).
func (r *TransactionRepo) ExecuteTransferTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateTransferRequest,
	processFn func(loadedItems map[int]model.ProcessTransferItem) (model.FinalTransferAggregate, error),
) (*model.Transaction, error) {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. PHASE ONE: BULK READ & LOCK (SOURCE & DEST)
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	loadedDbItems := make(map[int]model.ProcessTransferItem)
	
	// A. Load Master Item Info (Existing Cost)
	rows, err := tx.Query(ctx, `SELECT id, cost FROM items WHERE id = ANY($1) AND tenant_id = $2`, itemIDs, tenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int
		var cost float64
		if err := rows.Scan(&id, &cost); err != nil {
			rows.Close()
			return nil, err
		}
		loadedDbItems[id] = model.ProcessTransferItem{ItemID: id, ExistingCost: cost}
	}
	rows.Close()

	// B. Lock Source and Load Source Stock
	var sourceQuery string
	if req.SourceType == "branch" {
		sourceQuery = `SELECT id, item_id, stock FROM branch_items WHERE item_id = ANY($1) AND branch_id = $2 AND tenant_id = $3 FOR UPDATE`
	} else {
		sourceQuery = `SELECT id, item_id, stock FROM warehouse_items WHERE item_id = ANY($1) AND warehouse_id = $2 AND tenant_id = $3 FOR UPDATE`
	}

	rows, err = tx.Query(ctx, sourceQuery, itemIDs, req.SourceID, tenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var locID, itemID, stock int
		if err := rows.Scan(&locID, &itemID, &stock); err != nil {
			rows.Close()
			return nil, err
		}
		ptr := loadedDbItems[itemID]
		ptr.SourceStock = stock
		ptr.SourceItemLocID = locID
		loadedDbItems[itemID] = ptr
	}
	rows.Close()

	// C. Resolve Dest (Upsert) & Lock Dest
	for _, id := range itemIDs {
		var destLocID, destStock int
		if req.DestType == "branch" {
			err = tx.QueryRow(ctx, `
				INSERT INTO branch_items (tenant_id, branch_id, item_id, stock)
				VALUES ($1, $2, $3, 0)
				ON CONFLICT (tenant_id, branch_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id, stock`,
				tenantID, req.DestID, id,
			).Scan(&destLocID, &destStock)
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO warehouse_items (tenant_id, warehouse_id, item_id, stock)
				VALUES ($1, $2, $3, 0)
				ON CONFLICT (tenant_id, warehouse_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id, stock`,
				tenantID, req.DestID, id,
			).Scan(&destLocID, &destStock)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to ensure dest location record: %w", err)
		}
		
		ptr := loadedDbItems[id]
		ptr.DestStock = destStock
		ptr.DestItemLocID = destLocID
		loadedDbItems[id] = ptr
	}

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC
	finalAggregate, err := processFn(loadedDbItems)
	if err != nil {
		return nil, err
	}

	// 3. PHASE THREE: DOUBLE-ENTRY WRITE
	timestamp := time.Now().Format("20060102150405")
	trxNoBase := timestamp
	trfoNo := fmt.Sprintf("TRFO-%s", trxNoBase)
	trfiNo := fmt.Sprintf("TRFI-%s", trxNoBase)

	// A. Insert Header TRFO (Source)
	var trfoID int
	var sourceBranchID, sourceWarehouseID *int
	if req.SourceType == "branch" {
		sourceBranchID = &req.SourceID
	} else {
		sourceWarehouseID = &req.SourceID
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (tenant_id, trxno, branch_id, warehouse_id, user_id, trans_type, total_amount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		tenantID, trfoNo, sourceBranchID, sourceWarehouseID, userID, model.TxTransfer, 0, req.Note,
	).Scan(&trfoID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert TRFO header: %w", err)
	}

	// B. Insert Header TRFI (Dest)
	var trfiID int
	var destBranchID, destWarehouseID *int
	if req.DestType == "branch" {
		destBranchID = &req.DestID
	} else {
		destWarehouseID = &req.DestID
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (tenant_id, trxno, branch_id, warehouse_id, user_id, trans_type, total_amount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		tenantID, trfiNo, destBranchID, destWarehouseID, userID, model.TxTransfer, 0, req.Note,
	).Scan(&trfiID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert TRFI header: %w", err)
	}

	batch := &pgx.Batch{}

	// C. Queue TRFO Details & Stock Deduction
	for _, detail := range finalAggregate.SourceDetails {
		if req.SourceType == "branch" {
			batch.Queue(`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal) VALUES ($1, $2, $3, $4, $5, $6)`,
				trfoID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal)
			batch.Queue(`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`, detail.Quantity, detail.BranchItemID)
		} else {
			batch.Queue(`INSERT INTO transaction_detail (transaction_id, warehouse_item_id, quantity, cogs, price, subtotal) VALUES ($1, $2, $3, $4, $5, $6)`,
				trfoID, detail.WarehouseItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal)
			batch.Queue(`UPDATE warehouse_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`, detail.Quantity, detail.WarehouseItemID)
		}
	}

	// D. Queue TRFI Details & Stock Addition
	for _, detail := range finalAggregate.DestDetails {
		if req.DestType == "branch" {
			batch.Queue(`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal) VALUES ($1, $2, $3, $4, $5, $6)`,
				trfiID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal)
			batch.Queue(`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`, detail.Quantity, detail.BranchItemID)
		} else {
			batch.Queue(`INSERT INTO transaction_detail (transaction_id, warehouse_item_id, quantity, cogs, price, subtotal) VALUES ($1, $2, $3, $4, $5, $6)`,
				trfiID, detail.WarehouseItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal)
			batch.Queue(`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`, detail.Quantity, detail.WarehouseItemID)
		}
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return nil, fmt.Errorf("batch execution failed at index %d: %w", i, err)
		}
	}
	br.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the TRFI (Transfer In) record as the representative result.
	return &model.Transaction{
		ID:          trfiID,
		TrxNo:       trfiNo,
		TenantID:    tenantID,
		BranchID:    destBranchID,
		WarehouseID: destWarehouseID,
		UserID:      &userID,
		TransType:   model.TxTransfer,
		Note:        req.Note,
		CreatedAt:   time.Now(),
		Details:     finalAggregate.DestDetails,
	}, nil
}
