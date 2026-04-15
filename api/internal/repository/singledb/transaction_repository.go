package singledb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/shopspring/decimal"
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

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, req.BranchID, tenantID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found for this tenant", req.BranchID))
		}
		return nil, fmt.Errorf("failed to validate branch identity: %w", err)
	}

	if req.CustomerID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM customers WHERE id = $1 AND tenant_id = $2`, *req.CustomerID, tenantID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("customer id %d not found for this tenant", *req.CustomerID))
			}
			return nil, fmt.Errorf("failed to validate customer identity: %w", err)
		}
	}

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
			cost  decimal.Decimal
			price decimal.Decimal
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

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.BranchID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, *req.BranchID, tenantID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found for this tenant", *req.BranchID))
			}
			return nil, fmt.Errorf("failed to validate branch identity: %w", err)
		}
	} else if req.WarehouseID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND tenant_id = $2`, *req.WarehouseID, tenantID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("warehouse id %d not found for this tenant", *req.WarehouseID))
			}
			return nil, fmt.Errorf("failed to validate warehouse identity: %w", err)
		}
	}

	err = tx.QueryRow(ctx, `SELECT 1 FROM suppliers WHERE id = $1 AND tenant_id = $2`, req.SupplierID, tenantID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("supplier id %d not found for this tenant", req.SupplierID))
		}
		return nil, fmt.Errorf("failed to validate supplier identity: %w", err)
	}

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
		var cost decimal.Decimal
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

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.SourceType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, req.SourceID, tenantID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND tenant_id = $2`, req.SourceID, tenantID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("source %s id %d not found for this tenant", req.SourceType, req.SourceID))
		}
		return nil, fmt.Errorf("failed to validate source identity: %w", err)
	}

	if req.DestType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, req.DestID, tenantID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND tenant_id = $2`, req.DestID, tenantID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("destination %s id %d not found for this tenant", req.DestType, req.DestID))
		}
		return nil, fmt.Errorf("failed to validate destination identity: %w", err)
	}

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
		var cost decimal.Decimal
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

// ExecuteReturnTx handles the DB transaction and coordination for RETURNS (Customer -> Branch).
func (r *TransactionRepo) ExecuteReturnTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateReturnRequest,
	processFn func(loadedItems map[int]model.ProcessReturnItem) (model.FinalReturnAggregate, error),
) (*model.Transaction, error) {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, req.BranchID, tenantID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found for this tenant", req.BranchID))
		}
		return nil, fmt.Errorf("failed to validate branch identity: %w", err)
	}

	if req.CustomerID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM customers WHERE id = $1 AND tenant_id = $2`, *req.CustomerID, tenantID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("customer id %d not found for this tenant", *req.CustomerID))
			}
			return nil, fmt.Errorf("failed to validate customer identity: %w", err)
		}
	}

	// 0.1 RESOLVE ORIGINAL TRANSACTION
	var originalTrxID int
	err = tx.QueryRow(ctx, `SELECT id FROM transactions WHERE trxno = $1 AND tenant_id = $2`, req.OriginalTrxNo, tenantID).Scan(&originalTrxID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("original transaction %s not found", req.OriginalTrxNo))
		}
		return nil, fmt.Errorf("failed to lookup original transaction: %w", err)
	}

	// 1. PHASE ONE: BULK READ
	branchItemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		branchItemIDs = append(branchItemIDs, item.BranchItemID)
	}

	loadedDbItems := make(map[int]model.ProcessReturnItem)
	query := `SELECT id, stock FROM branch_items WHERE id = ANY($1) AND branch_id = $2 AND tenant_id = $3 FOR UPDATE`
	rows, err := tx.Query(ctx, query, branchItemIDs, req.BranchID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk lookup branch items for return: %w", err)
	}
	for rows.Next() {
		var id, stock int
		if err := rows.Scan(&id, &stock); err != nil {
			rows.Close()
			return nil, err
		}
		loadedDbItems[id] = model.ProcessReturnItem{BranchItemID: id, CurrentStock: stock}
	}
	rows.Close()

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC
	finalAggregate, err := processFn(loadedDbItems)
	if err != nil {
		return nil, err
	}
	finalAggregate.ReferenceTransactionID = originalTrxID

	// 3. PHASE THREE: WRITE
	trxNo := fmt.Sprintf("RET-%s", time.Now().Format("20060102150405"))
	var trxID int
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (tenant_id, trxno, branch_id, customer_id, user_id, trans_type, total_amount, reference_transaction_id, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		tenantID, trxNo, req.BranchID, req.CustomerID, userID, model.TxReturn, finalAggregate.TotalAmount, finalAggregate.ReferenceTransactionID, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert return header: %w", err)
	}

	batch := &pgx.Batch{}
	for _, detail := range finalAggregate.Details {
		batch.Queue(
			`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			trxID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
		)
		batch.Queue(
			`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			detail.Quantity, *detail.BranchItemID, tenantID,
		)
	}

	// Record Cashflow (OUT - because returning money to customer)
	batch.Queue(
		`INSERT INTO branch_cashflow (tenant_id, branch_id, transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tenantID, req.BranchID, trxID, model.CflowReturn, "OUT", finalAggregate.TotalAmount,
	)

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return nil, fmt.Errorf("return batch failed at query %d: %w", i, err)
		}
	}
	br.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	branchIDProxy := req.BranchID
	return &model.Transaction{
		ID:                     trxID,
		TrxNo:                  trxNo,
		TenantID:               tenantID,
		BranchID:               &branchIDProxy,
		CustomerID:             req.CustomerID,
		UserID:                 &userID,
		TransType:              model.TxReturn,
		TotalAmount:            finalAggregate.TotalAmount,
		ReferenceTransactionID: &finalAggregate.ReferenceTransactionID,
		Note:                   req.Note,
		CreatedAt:              time.Now(),
		Details:                finalAggregate.Details,
	}, nil
}

// ExecuteAdjustmentTx handles bulk stock adjustments and audit logging.
func (r *TransactionRepo) ExecuteAdjustmentTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.AdjustStockRequest,
	processFn func(currentStocks map[int]int) (map[int]int, error),
) error {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.LocationType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1 AND tenant_id = $2`, req.LocationID, tenantID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND tenant_id = $2`, req.LocationID, tenantID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return apierr.NotFound(fmt.Sprintf("%s id %d not found for this tenant", req.LocationType, req.LocationID))
		}
		return fmt.Errorf("failed to validate location identity: %w", err)
	}

	// 1. PHASE ONE: BULK READ
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	currentStocks := make(map[int]int)
	var query string
	if req.LocationType == "branch" {
		query = `SELECT item_id, stock FROM branch_items WHERE item_id = ANY($1) AND branch_id = $2 AND tenant_id = $3 FOR UPDATE`
	} else {
		query = `SELECT item_id, stock FROM warehouse_items WHERE item_id = ANY($1) AND warehouse_id = $2 AND tenant_id = $3 FOR UPDATE`
	}

	rows, err := tx.Query(ctx, query, itemIDs, req.LocationID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to bulk lookup items for adjustment: %w", err)
	}
	for rows.Next() {
		var itemID, stock int
		if err := rows.Scan(&itemID, &stock); err != nil {
			rows.Close()
			return err
		}
		currentStocks[itemID] = stock
	}
	rows.Close()

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC (Calculate diffs)
	stockChanges, err := processFn(currentStocks)
	if err != nil {
		return err
	}

	// 3. PHASE THREE: WRITE
	batch := &pgx.Batch{}
	for itemID, changeUnit := range stockChanges {
		if changeUnit == 0 {
			continue
		}

		if req.LocationType == "branch" {
			batch.Queue(`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE item_id = $2 AND branch_id = $3 AND tenant_id = $4`,
				changeUnit, itemID, req.LocationID, tenantID)
			batch.Queue(`INSERT INTO audit_stock (tenant_id, branch_item_id, change_unit, reason, user_id) 
				SELECT $1, id, $2, $3, $4 FROM branch_items WHERE item_id = $5 AND branch_id = $6 AND tenant_id = $7`,
				tenantID, changeUnit, req.Reason, userID, itemID, req.LocationID, tenantID)
		} else {
			batch.Queue(`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE item_id = $2 AND warehouse_id = $3 AND tenant_id = $4`,
				changeUnit, itemID, req.LocationID, tenantID)
			batch.Queue(`INSERT INTO audit_stock (tenant_id, warehouse_item_id, change_unit, reason, user_id) 
				SELECT $1, id, $2, $3, $4 FROM warehouse_items WHERE item_id = $5 AND warehouse_id = $6 AND tenant_id = $7`,
				tenantID, changeUnit, req.Reason, userID, itemID, req.LocationID, tenantID)
		}
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return fmt.Errorf("adjustment batch failed at index %d: %w", i, err)
		}
	}
	br.Close()

	return tx.Commit(ctx)
}

// List returns a paginated, filtered list of transactions for the tenant.
func (r *TransactionRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.TransactionFilter) ([]model.Transaction, int, error) {
	args := []any{tenantID}
	where := "WHERE tenant_id = $1"

	if f.TransType != "" {
		args = append(args, f.TransType)
		where += fmt.Sprintf(" AND trans_type = $%d", len(args))
	}

	if f.BranchID != nil {
		args = append(args, *f.BranchID)
		where += fmt.Sprintf(" AND branch_id = $%d", len(args))
	}

	if f.WarehouseID != nil {
		args = append(args, *f.WarehouseID)
		where += fmt.Sprintf(" AND warehouse_id = $%d", len(args))
	}

	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		where += fmt.Sprintf(" AND customer_id = $%d", len(args))
	}

	if f.SupplierID != nil {
		args = append(args, *f.SupplierID)
		where += fmt.Sprintf(" AND supplier_id = $%d", len(args))
	}

	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (trxno ILIKE $%d OR note ILIKE $%d)", n, n)
	}

	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		where += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}

	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		where += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}

	args = append(args, q.Limit, q.Offset())
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(`
		SELECT id, tenant_id, trxno, branch_id, warehouse_id, customer_id, supplier_id,
		       user_id, trans_type, total_amount, tax, discount, reference_transaction_id,
		       note, created_at,
		       COUNT(*) OVER() AS total_count
		FROM transactions
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.TransactionRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Transaction
		total int
	)
	for rows.Next() {
		var trx model.Transaction
		if err := rows.Scan(
			&trx.ID, &trx.TenantID, &trx.TrxNo, &trx.BranchID, &trx.WarehouseID,
			&trx.CustomerID, &trx.SupplierID, &trx.UserID, &trx.TransType,
			&trx.TotalAmount, &trx.Tax, &trx.Discount, &trx.ReferenceTransactionID,
			&trx.Note, &trx.CreatedAt, &total,
		); err != nil {
			return nil, 0, fmt.Errorf("singledb.TransactionRepo.List scan: %w", err)
		}
		list = append(list, trx)
	}

	return list, total, rows.Err()
}

// GetByID fetches a single transaction with its details joined.
func (r *TransactionRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Transaction, error) {
	var trx model.Transaction
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, trxno, branch_id, warehouse_id, customer_id, supplier_id,
		        user_id, trans_type, total_amount, tax, discount, reference_transaction_id,
		        note, created_at
		 FROM transactions
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(
		&trx.ID, &trx.TenantID, &trx.TrxNo, &trx.BranchID, &trx.WarehouseID,
		&trx.CustomerID, &trx.SupplierID, &trx.UserID, &trx.TransType,
		&trx.TotalAmount, &trx.Tax, &trx.Discount, &trx.ReferenceTransactionID,
		&trx.Note, &trx.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("transaction id %d not found for this tenant", id))
		}
		return nil, fmt.Errorf("singledb.TransactionRepo.GetByID (header): %w", err)
	}

	detailRows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, branch_item_id, warehouse_item_id,
		        quantity, cogs, price, subtotal
		 FROM transaction_detail
		 WHERE transaction_id = $1
		 ORDER BY id ASC`,
		trx.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("singledb.TransactionRepo.GetByID (details): %w", err)
	}
	defer detailRows.Close()

	for detailRows.Next() {
		var d model.TransactionDetail
		if err := detailRows.Scan(
			&d.ID, &d.TransactionID, &d.BranchItemID, &d.WarehouseItemID,
			&d.Quantity, &d.COGS, &d.Price, &d.Subtotal,
		); err != nil {
			return nil, fmt.Errorf("singledb.TransactionRepo.GetByID detail scan: %w", err)
		}
		trx.Details = append(trx.Details, d)
	}

	return &trx, detailRows.Err()
}

// ExecuteVoidTx handles the DB transaction for voiding a transaction.
func (r *TransactionRepo) ExecuteVoidTx(
	ctx context.Context,
	tenantID int,
	userID int,
	originalTrxID int,
	reason string,
	processFn func(data model.ProcessVoidData) error,
) (*model.Transaction, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Identify target IDs (Single or Pair if Transfer)
	var targets []int
	targets = append(targets, originalTrxID)

	// Preliminary check to see if we need a partner
	var transType model.TransactionType
	var trxNo string
	err = tx.QueryRow(ctx, `SELECT trans_type, trxno FROM transactions WHERE id = $1 AND tenant_id = $2`, originalTrxID, tenantID).Scan(&transType, &trxNo)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("transaction id %d not found", originalTrxID))
		}
		return nil, err
	}

	if transType == model.TxTransfer {
		suffix := ""
		partnerNo := ""
		if strings.HasPrefix(trxNo, "TRFO-") {
			suffix = strings.TrimPrefix(trxNo, "TRFO-")
			partnerNo = "TRFI-" + suffix
		} else if strings.HasPrefix(trxNo, "TRFI-") {
			suffix = strings.TrimPrefix(trxNo, "TRFI-")
			partnerNo = "TRFO-" + suffix
		}

		if partnerNo != "" {
			var partnerID int
			err = tx.QueryRow(ctx, `SELECT id FROM transactions WHERE trxno = $1 AND tenant_id = $2`, partnerNo, tenantID).Scan(&partnerID)
			if err == nil {
				targets = append(targets, partnerID)
			}
		}
	}

	batch := &pgx.Batch{}
	var requestedResult *model.Transaction

	for _, targetID := range targets {

		// 1. Fetch transaction header & Lock it
		var original model.Transaction
		err = tx.QueryRow(ctx,
			`SELECT id, trxno, branch_id, warehouse_id, customer_id, supplier_id, user_id, trans_type, total_amount, tax, discount, note, created_at
			 FROM transactions
			 WHERE id = $1 AND tenant_id = $2 FOR UPDATE`,
			targetID, tenantID,
		).Scan(
			&original.ID, &original.TrxNo, &original.BranchID, &original.WarehouseID,
			&original.CustomerID, &original.SupplierID, &original.UserID, &original.TransType,
			&original.TotalAmount, &original.Tax, &original.Discount, &original.Note, &original.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch transaction %d: %w", targetID, err)
		}

		// 2. Check if already voided
		var alreadyVoided bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM transactions WHERE reference_transaction_id = $1 AND trans_type = 'VOID')`, targetID).Scan(&alreadyVoided)
		if err != nil {
			return nil, err
		}


		// 3. Fetch details WITH current stock info
		var processDetails []model.ProcessVoidDetail
		rows, err := tx.Query(ctx, `
			SELECT td.branch_item_id, td.warehouse_item_id, td.quantity, td.cogs, td.price, td.subtotal,
				   COALESCE(bi.stock, wi.stock, 0) AS current_stock
			FROM transaction_detail td
			LEFT JOIN branch_items bi ON td.branch_item_id = bi.id
			LEFT JOIN warehouse_items wi ON td.warehouse_item_id = wi.id
			WHERE td.transaction_id = $1`, targetID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch details for %d: %w", targetID, err)
		}
		for rows.Next() {
			var pvd model.ProcessVoidDetail
			var d model.TransactionDetail
			err := rows.Scan(
				&d.BranchItemID, &d.WarehouseItemID,
				&d.Quantity, &d.COGS, &d.Price, &d.Subtotal,
				&pvd.CurrentStock,
			)
			if err != nil {
				rows.Close()
				return nil, err
			}
			pvd.Detail = d
			processDetails = append(processDetails, pvd)
		}
		rows.Close()

		// 4. Validate via Closure
		err = processFn(model.ProcessVoidData{
			OriginalHeader: original,
			AlreadyVoided:  alreadyVoided,
			Details:        processDetails,
		})
		if err != nil {
			return nil, err
		}

		// 5. Create VOID record
		// Use partial seconds to avoid unique constraint if many voids happen same tiny second
		voidTrxNo := fmt.Sprintf("VOID-%s", time.Now().Format("20060102150405.000")) 
		if targetID != originalTrxID {
			voidTrxNo += "-PAIR"
		}
		
		var voidID int
		err = tx.QueryRow(ctx,
			`INSERT INTO transactions (tenant_id, trxno, branch_id, warehouse_id, customer_id, supplier_id, user_id, trans_type, total_amount, tax, discount, reference_transaction_id, note)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			 RETURNING id`,
			tenantID, voidTrxNo, original.BranchID, original.WarehouseID, original.CustomerID, original.SupplierID, userID, model.TxVoid, original.TotalAmount, original.Tax, original.Discount, original.ID, reason,
		).Scan(&voidID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert void for %d: %w", targetID, err)
		}

		// 6. Update original note
		newNote := fmt.Sprintf("%s [VOIDED]", original.Note)
		batch.Queue(`UPDATE transactions SET note = $1 WHERE id = $2`, newNote, original.ID)

		// 7. Queue Stock & Cashflow reversals
		for _, pvd := range processDetails {
			d := pvd.Detail
			batch.Queue(`INSERT INTO transaction_detail (transaction_id, branch_item_id, warehouse_item_id, quantity, cogs, price, subtotal)
						 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				voidID, d.BranchItemID, d.WarehouseItemID, d.Quantity, d.COGS, d.Price, d.Subtotal)

			switch original.TransType {
			case model.TxSale:
				batch.Queue(`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.BranchItemID)
			case model.TxTransfer:
				if strings.Contains(original.TrxNo, "TRFI") {
					if d.BranchItemID != nil {
						batch.Queue(`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.BranchItemID)
					} else {
						batch.Queue(`UPDATE warehouse_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.WarehouseItemID)
					}
				} else if strings.Contains(original.TrxNo, "TRFO") {
					if d.BranchItemID != nil {
						batch.Queue(`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.BranchItemID)
					} else {
						batch.Queue(`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.WarehouseItemID)
					}
				}
			case model.TxReturn:
				batch.Queue(`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`, d.Quantity, *d.BranchItemID)
			}
		}

		if original.BranchID != nil {
			switch original.TransType {
			case model.TxSale:
				batch.Queue(`INSERT INTO branch_cashflow (tenant_id, branch_id, transaction_id, flow_type, direction, amount)
						VALUES ($1, $2, $3, $4, $5, $6)`,
					tenantID, *original.BranchID, voidID, model.CflowVoid, "OUT", original.TotalAmount)
			case model.TxReturn:
				batch.Queue(`INSERT INTO branch_cashflow (tenant_id, branch_id, transaction_id, flow_type, direction, amount)
						VALUES ($1, $2, $3, $4, $5, $6)`,
					tenantID, *original.BranchID, voidID, model.CflowVoid, "IN", original.TotalAmount)
			}
		}

		if targetID == originalTrxID {
			requestedResult = &model.Transaction{
				ID:                     voidID,
				TrxNo:                  voidTrxNo,
				TenantID:               tenantID,
				BranchID:               original.BranchID,
				WarehouseID:            original.WarehouseID,
				CustomerID:             original.CustomerID,
				SupplierID:             original.SupplierID,
				UserID:                 &userID,
				TransType:              model.TxVoid,
				TotalAmount:            original.TotalAmount,
				ReferenceTransactionID: &original.ID,
				Note:                   reason,
				CreatedAt:              time.Now(),
			}
		}
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return nil, fmt.Errorf("void batch failed: %w", err)
		}
	}
	br.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return requestedResult, nil
}
