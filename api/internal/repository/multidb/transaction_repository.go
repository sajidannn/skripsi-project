package multidb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/shopspring/decimal"
)

// TransactionRepo implements repository.TransactionRepository for multi-DB mode.
type TransactionRepo struct {
	mgr *multidb.Manager
}

// NewTransactionRepo creates a new TransactionRepo backed by the tenant Manager.
func NewTransactionRepo(mgr *multidb.Manager) *TransactionRepo {
	return &TransactionRepo{mgr: mgr}
}

// ExecuteSaleTx executes the entire SALE flow inside a postgres transaction for the tenant's DB.
// It applies Bulk Queries and pgx.Batch pipelines to completely bypass the N+1 problem.
func (r *TransactionRepo) ExecuteSaleTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateSaleRequest,
	processFn func(loadedItems map[int]model.ProcessSaleItem) (model.FinalSaleAggregate, error),
) (*model.Transaction, error) {

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, req.BranchID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found", req.BranchID))
		}
		return nil, fmt.Errorf("failed to validate branch identity: %w", err)
	}

	if req.CustomerID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM customers WHERE id = $1`, *req.CustomerID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("customer id %d not found", *req.CustomerID))
			}
			return nil, fmt.Errorf("failed to validate customer identity: %w", err)
		}
	}

	// 1. PHASE ONE: BULK READ
	branchItemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		branchItemIDs = append(branchItemIDs, item.BranchItemID)
	}

	loadedDbItems := make(map[int]model.ProcessSaleItem)

	// 2-Level Price Resolution algorithm implemented natively in SQL via COALESCE.
	query := `
		SELECT b.id, b.stock, i.cost, COALESCE(b.price, i.price) 
		FROM branch_items b
		JOIN items i ON b.item_id = i.id
		WHERE b.id = ANY($1) AND b.branch_id = $2 
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, branchItemIDs, req.BranchID)
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
			rows.Close()
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

	// 3. PHASE THREE: BULK WRITE
	trxNo := fmt.Sprintf("SALE-%s", time.Now().Format("20060102150405"))

	var trxID int
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (trxno, branch_id, customer_id, user_id, trans_type, total_amount, tax, discount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		trxNo, req.BranchID, req.CustomerID, userID, model.TxSale, finalAggregate.TotalAmount, req.Tax, req.Discount, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert transaction header: %w", err)
	}

	batch := &pgx.Batch{}

	for _, detail := range finalAggregate.Details {
		batch.Queue(
			`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			trxID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
		)
		batch.Queue(
			`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`,
			detail.Quantity, *detail.BranchItemID,
		)
	}

	batch.Queue(
		`INSERT INTO branch_cashflow (branch_id, transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4, $5)`,
		req.BranchID, trxID, model.CflowSale, "IN", finalAggregate.TotalAmount,
	)

	// Execute pipelined queue!
	batchResults := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		_, err := batchResults.Exec()
		if err != nil {
			batchResults.Close()
			return nil, fmt.Errorf("bulk execution failed at query %d: %w", i, err)
		}
	}
	batchResults.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	branchIDProxy := req.BranchID
	return &model.Transaction{
		ID:          trxID,
		TenantID:    tenantID, // Inherited contextually by the response despite not existing natively in multi-DB tables
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
		Details:     finalAggregate.Details, // Generated IDs omitted securely via Batching!
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

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.BranchID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, *req.BranchID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found", *req.BranchID))
			}
			return nil, fmt.Errorf("failed to validate branch identity: %w", err)
		}
	} else if req.WarehouseID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1`, *req.WarehouseID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("warehouse id %d not found", *req.WarehouseID))
			}
			return nil, fmt.Errorf("failed to validate warehouse identity: %w", err)
		}
	}

	err = tx.QueryRow(ctx, `SELECT 1 FROM suppliers WHERE id = $1`, req.SupplierID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("supplier id %d not found", req.SupplierID))
		}
		return nil, fmt.Errorf("failed to validate supplier identity: %w", err)
	}

	// 1. PHASE ONE: BULK READ & GLOBAL STOCK CALCULATION
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	loadedDbItems := make(map[int]model.ProcessPurchaseItem)

	// A. Lock and Load Master Items
	rows, err := tx.Query(ctx, `SELECT id, cost FROM items WHERE id = ANY($1) FOR UPDATE`, itemIDs)
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

	// B. Calculate Global Stock
	branchRows, err := tx.Query(ctx, `SELECT item_id, SUM(stock) FROM branch_items WHERE item_id = ANY($1) GROUP BY item_id`, itemIDs)
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

	whRows, err := tx.Query(ctx, `SELECT item_id, SUM(stock) FROM warehouse_items WHERE item_id = ANY($1) GROUP BY item_id`, itemIDs)
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

	// C. Resolve Target Location IDs
	locationItemIDs := make(map[int]int)
	for _, id := range itemIDs {
		var locID int
		if req.BranchID != nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO branch_items (branch_id, item_id, stock)
				VALUES ($1, $2, 0)
				ON CONFLICT (branch_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id`,
				*req.BranchID, id,
			).Scan(&locID)
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO warehouse_items (warehouse_id, item_id, stock)
				VALUES ($1, $2, 0)
				ON CONFLICT (warehouse_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id`,
				*req.WarehouseID, id,
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
		`INSERT INTO transactions (trxno, branch_id, warehouse_id, supplier_id, user_id, trans_type, total_amount, tax, discount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		trxNo, finalBranchID, finalWarehouseID, req.SupplierID, userID, model.TxPurchase, finalAggregate.TotalAmount, req.Tax, req.Discount, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert purchase header: %w", err)
	}

	batch := &pgx.Batch{}

	for itemID, newCost := range finalAggregate.NewCosts {
		batch.Queue(`UPDATE items SET cost = $1, updated_at = NOW() WHERE id = $2`, newCost, itemID)
	}

	for _, detail := range finalAggregate.Details {
		var locID int
		if req.BranchID != nil {
			locID = locationItemIDs[*detail.BranchItemID]
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, locID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
				detail.Quantity, locID,
			)
		} else {
			locID = locationItemIDs[*detail.WarehouseItemID]
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, warehouse_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, locID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
				detail.Quantity, locID,
			)
		}
	}

	batch.Queue(
		`INSERT INTO tenant_cashflow (transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4)`,
		trxID, model.CflowPurch, "OUT", finalAggregate.TotalAmount,
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

// ExecuteTransferTx handles the DB transaction for OMNI-DIRECTIONAL transfer in Multi-DB mode.
func (r *TransactionRepo) ExecuteTransferTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateTransferRequest,
	processFn func(loadedItems map[int]model.ProcessTransferItem) (model.FinalTransferAggregate, error),
) (*model.Transaction, error) {

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.SourceType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, req.SourceID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1`, req.SourceID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("source %s id %d not found", req.SourceType, req.SourceID))
		}
		return nil, fmt.Errorf("failed to validate source identity: %w", err)
	}

	if req.DestType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, req.DestID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1`, req.DestID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("destination %s id %d not found", req.DestType, req.DestID))
		}
		return nil, fmt.Errorf("failed to validate destination identity: %w", err)
	}

	// 1. PHASE ONE: BULK READ & LOCK (SOURCE & DEST)
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	loadedDbItems := make(map[int]model.ProcessTransferItem)

	// A. Load Master Item Info
	rows, err := tx.Query(ctx, `SELECT id, cost FROM items WHERE id = ANY($1)`, itemIDs)
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
		sourceQuery = `SELECT id, item_id, stock FROM branch_items WHERE item_id = ANY($1) AND branch_id = $2 FOR UPDATE`
	} else {
		sourceQuery = `SELECT id, item_id, stock FROM warehouse_items WHERE item_id = ANY($1) AND warehouse_id = $2 FOR UPDATE`
	}

	rows, err = tx.Query(ctx, sourceQuery, itemIDs, req.SourceID)
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
				INSERT INTO branch_items (branch_id, item_id, stock)
				VALUES ($1, $2, 0)
				ON CONFLICT (branch_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id, stock`,
				req.DestID, id,
			).Scan(&destLocID, &destStock)
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO warehouse_items (warehouse_id, item_id, stock)
				VALUES ($1, $2, 0)
				ON CONFLICT (warehouse_id, item_id) DO UPDATE SET updated_at = NOW()
				RETURNING id, stock`,
				req.DestID, id,
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
	trfoNo := fmt.Sprintf("TRFO-%s", timestamp)
	trfiNo := fmt.Sprintf("TRFI-%s", timestamp)

	// A. Insert Header TRFO
	var trfoID int
	var sourceBranchID, sourceWarehouseID *int
	if req.SourceType == "branch" {
		sourceBranchID = &req.SourceID
	} else {
		sourceWarehouseID = &req.SourceID
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (trxno, branch_id, warehouse_id, user_id, trans_type, total_amount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		trfoNo, sourceBranchID, sourceWarehouseID, userID, model.TxTransfer, 0, req.Note,
	).Scan(&trfoID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert TRFO header: %w", err)
	}

	// B. Insert Header TRFI
	var trfiID int
	var destBranchID, destWarehouseID *int
	if req.DestType == "branch" {
		destBranchID = &req.DestID
	} else {
		destWarehouseID = &req.DestID
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (trxno, branch_id, warehouse_id, user_id, trans_type, total_amount, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		trfiNo, destBranchID, destWarehouseID, userID, model.TxTransfer, 0, req.Note,
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

	return &model.Transaction{
		ID:          trfiID,
		TrxNo:       trfiNo,
		BranchID:    destBranchID,
		WarehouseID: destWarehouseID,
		UserID:      &userID,
		TransType:   model.TxTransfer,
		Note:        req.Note,
		CreatedAt:   time.Now(),
		Details:     finalAggregate.DestDetails,
	}, nil
}

// ExecuteReturnTx handles the DB transaction and coordination for RETURNS (Customer -> Branch) in Multi-DB mode.
func (r *TransactionRepo) ExecuteReturnTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreateReturnRequest,
	processFn func(loadedItems map[int]model.ProcessReturnItem) (model.FinalReturnAggregate, error),
) (*model.Transaction, error) {

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, req.BranchID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found", req.BranchID))
		}
		return nil, fmt.Errorf("failed to validate branch identity: %w", err)
	}

	if req.CustomerID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM customers WHERE id = $1`, *req.CustomerID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("customer id %d not found", *req.CustomerID))
			}
			return nil, fmt.Errorf("failed to validate customer identity: %w", err)
		}
	}

	// 0.1 RESOLVE ORIGINAL TRANSACTION (must be a SALE)
	var originalTrxID int
	var originalTransType string
	err = tx.QueryRow(ctx, `SELECT id, trans_type FROM transactions WHERE trxno = $1`, req.OriginalTrxNo).Scan(&originalTrxID, &originalTransType)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("original transaction %s not found", req.OriginalTrxNo))
		}
		return nil, fmt.Errorf("failed to lookup original transaction: %w", err)
	}
	if originalTransType != string(model.TxSale) {
		return nil, apierr.BadRequest(fmt.Sprintf("transaction %s is not a SALE (got %s); only SALE transactions can be returned", req.OriginalTrxNo, originalTransType))
	}

	// 1. PHASE ONE: BULK READ
	// A. Load original transaction details (price and qty) keyed by branch_item_id
	type origDetail struct {
		Price decimal.Decimal
		Qty   int
	}
	originalDetails := make(map[int]origDetail) // branch_item_id -> {price, qty}
	detailRows, err := tx.Query(ctx, `SELECT branch_item_id, price, quantity FROM transaction_detail WHERE transaction_id = $1 AND branch_item_id IS NOT NULL`, originalTrxID)
	if err != nil {
		return nil, fmt.Errorf("failed to load original transaction details: %w", err)
	}
	for detailRows.Next() {
		var branchItemID, qty int
		var price decimal.Decimal
		if err := detailRows.Scan(&branchItemID, &price, &qty); err != nil {
			detailRows.Close()
			return nil, err
		}
		originalDetails[branchItemID] = origDetail{Price: price, Qty: qty}
	}
	detailRows.Close()

	// B. Lock & load current branch stock
	branchItemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		branchItemIDs = append(branchItemIDs, item.BranchItemID)
	}

	loadedDbItems := make(map[int]model.ProcessReturnItem)
	query := `SELECT id, stock FROM branch_items WHERE id = ANY($1) AND branch_id = $2 FOR UPDATE`
	rows, err := tx.Query(ctx, query, branchItemIDs, req.BranchID)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk lookup branch items for return: %w", err)
	}
	for rows.Next() {
		var id, stock int
		if err := rows.Scan(&id, &stock); err != nil {
			rows.Close()
			return nil, err
		}
		orig := originalDetails[id]
		loadedDbItems[id] = model.ProcessReturnItem{
			BranchItemID:  id,
			CurrentStock:  stock,
			OriginalPrice: orig.Price,
			OriginalQty:   orig.Qty,
		}
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
		`INSERT INTO transactions (trxno, branch_id, customer_id, user_id, trans_type, total_amount, reference_transaction_id, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		trxNo, req.BranchID, req.CustomerID, userID, model.TxReturn, finalAggregate.TotalAmount, finalAggregate.ReferenceTransactionID, req.Note,
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
			`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
			detail.Quantity, *detail.BranchItemID,
		)
	}

	// Record Cashflow (OUT)
	batch.Queue(
		`INSERT INTO branch_cashflow (branch_id, transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4, $5)`,
		req.BranchID, trxID, model.CflowReturn, "OUT", finalAggregate.TotalAmount,
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

// ExecutePurchaseReturnTx handles the DB transaction for returning goods to a supplier in Multi-DB mode.
// Stock is deducted at current WAC; refund amount is based on user-specified ReturnPrice.
func (r *TransactionRepo) ExecutePurchaseReturnTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.CreatePurchaseReturnRequest,
	processFn func(loadedItems map[int]model.ProcessPurchaseReturnItem) (model.FinalPurchaseReturnAggregate, error),
) (*model.Transaction, error) {

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.BranchID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, *req.BranchID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("branch id %d not found", *req.BranchID))
			}
			return nil, fmt.Errorf("failed to validate branch identity: %w", err)
		}
	} else if req.WarehouseID != nil {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1`, *req.WarehouseID).Scan(&exists)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("warehouse id %d not found", *req.WarehouseID))
			}
			return nil, fmt.Errorf("failed to validate warehouse identity: %w", err)
		}
	}

	err = tx.QueryRow(ctx, `SELECT 1 FROM suppliers WHERE id = $1`, req.SupplierID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("supplier id %d not found", req.SupplierID))
		}
		return nil, fmt.Errorf("failed to validate supplier identity: %w", err)
	}

	// 0.1 RESOLVE ORIGINAL PURCHASE TRANSACTION
	var originalTrxID int
	err = tx.QueryRow(ctx, `SELECT id FROM transactions WHERE trxno = $1 AND trans_type = 'PURC'`, req.OriginalTrxNo).Scan(&originalTrxID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("original purchase transaction %s not found", req.OriginalTrxNo))
		}
		return nil, fmt.Errorf("failed to lookup original purchase transaction: %w", err)
	}

	// 1. PHASE ONE: BULK READ & LOCK
	itemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		itemIDs = append(itemIDs, item.ItemID)
	}

	loadedDbItems := make(map[int]model.ProcessPurchaseReturnItem)

	// A. Load current WAC (master cost) per item
	rows, err := tx.Query(ctx, `SELECT id, cost FROM items WHERE id = ANY($1)`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load item costs: %w", err)
	}
	for rows.Next() {
		var id int
		var cost decimal.Decimal
		if err := rows.Scan(&id, &cost); err != nil {
			rows.Close()
			return nil, err
		}
		loadedDbItems[id] = model.ProcessPurchaseReturnItem{ItemID: id, CurrentCost: cost}
	}
	rows.Close()

	// B. Load OriginalQty from original purchase transaction details (via branch_items or warehouse_items → items)
	origQtyRows, err := tx.Query(ctx, `
		SELECT bi.item_id, td.quantity
		FROM transaction_detail td
		JOIN branch_items bi ON td.branch_item_id = bi.id
		WHERE td.transaction_id = $1 AND bi.item_id = ANY($2)
		UNION ALL
		SELECT wi.item_id, td.quantity
		FROM transaction_detail td
		JOIN warehouse_items wi ON td.warehouse_item_id = wi.id
		WHERE td.transaction_id = $1 AND wi.item_id = ANY($2)
	`, originalTrxID, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load original purchase quantities: %w", err)
	}
	for origQtyRows.Next() {
		var itemID, qty int
		if err := origQtyRows.Scan(&itemID, &qty); err != nil {
			origQtyRows.Close()
			return nil, err
		}
		ptr := loadedDbItems[itemID]
		ptr.OriginalQty = qty
		loadedDbItems[itemID] = ptr
	}
	origQtyRows.Close()

	// C. Load & Lock location stock
	if req.BranchID != nil {
		locRows, err := tx.Query(ctx, `SELECT id, item_id, stock FROM branch_items WHERE item_id = ANY($1) AND branch_id = $2 FOR UPDATE`, itemIDs, *req.BranchID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock branch items: %w", err)
		}
		for locRows.Next() {
			var locID, itemID, stock int
			if err := locRows.Scan(&locID, &itemID, &stock); err != nil {
				locRows.Close()
				return nil, err
			}
			ptr := loadedDbItems[itemID]
			ptr.LocItemID = locID
			ptr.CurrentStock = stock
			loadedDbItems[itemID] = ptr
		}
		locRows.Close()
	} else {
		locRows, err := tx.Query(ctx, `SELECT id, item_id, stock FROM warehouse_items WHERE item_id = ANY($1) AND warehouse_id = $2 FOR UPDATE`, itemIDs, *req.WarehouseID)
		if err != nil {
			return nil, fmt.Errorf("failed to lock warehouse items: %w", err)
		}
		for locRows.Next() {
			var locID, itemID, stock int
			if err := locRows.Scan(&locID, &itemID, &stock); err != nil {
				locRows.Close()
				return nil, err
			}
			ptr := loadedDbItems[itemID]
			ptr.LocItemID = locID
			ptr.CurrentStock = stock
			loadedDbItems[itemID] = ptr
		}
		locRows.Close()
	}

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC (Closure)
	finalAggregate, err := processFn(loadedDbItems)
	if err != nil {
		return nil, err
	}
	finalAggregate.ReferenceTransactionID = originalTrxID

	// 3. PHASE THREE: BULK WRITE
	trxNo := fmt.Sprintf("RETPURC-%s", time.Now().Format("20060102150405"))

	var finalBranchID, finalWarehouseID *int
	if req.BranchID != nil {
		finalBranchID = req.BranchID
	} else {
		finalWarehouseID = req.WarehouseID
	}

	var trxID int
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (trxno, branch_id, warehouse_id, supplier_id, user_id, trans_type, total_amount, reference_transaction_id, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		trxNo, finalBranchID, finalWarehouseID, req.SupplierID, userID, model.TxReturnPurc, finalAggregate.TotalRefund, finalAggregate.ReferenceTransactionID, req.Note,
	).Scan(&trxID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert purchase return header: %w", err)
	}

	batch := &pgx.Batch{}

	for _, detail := range finalAggregate.Details {
		if req.BranchID != nil {
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, branch_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, detail.BranchItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE branch_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`,
				detail.Quantity, detail.BranchItemID,
			)
		} else {
			batch.Queue(
				`INSERT INTO transaction_detail (transaction_id, warehouse_item_id, quantity, cogs, price, subtotal)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				trxID, detail.WarehouseItemID, detail.Quantity, detail.COGS, detail.Price, detail.Subtotal,
			)
			batch.Queue(
				`UPDATE warehouse_items SET stock = stock - $1, updated_at = NOW() WHERE id = $2`,
				detail.Quantity, detail.WarehouseItemID,
			)
		}
	}

	// Tenant Cashflow IN (uang kembali dari supplier)
	batch.Queue(
		`INSERT INTO tenant_cashflow (transaction_id, flow_type, direction, amount)
		 VALUES ($1, $2, $3, $4)`,
		trxID, model.CflowReturnPurc, "IN", finalAggregate.TotalRefund,
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
		return nil, fmt.Errorf("failed to commit purchase return transaction: %w", err)
	}

	return &model.Transaction{
		ID:                     trxID,
		TrxNo:                  trxNo,
		TenantID:               tenantID,
		BranchID:               finalBranchID,
		WarehouseID:            finalWarehouseID,
		SupplierID:             &req.SupplierID,
		UserID:                 &userID,
		TransType:              model.TxReturnPurc,
		TotalAmount:            finalAggregate.TotalRefund,
		ReferenceTransactionID: &finalAggregate.ReferenceTransactionID,
		Note:                   req.Note,
		CreatedAt:              time.Now(),
		Details:                finalAggregate.Details,
	}, nil
}

// ExecuteAdjustmentTx handles bulk stock adjustments for Multi-DB mode.
func (r *TransactionRepo) ExecuteAdjustmentTx(
	ctx context.Context,
	tenantID int,
	userID int,
	req dto.AdjustStockRequest,
	processFn func(currentStocks map[int]int) (map[int]int, error),
) error {

	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant pool: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 0. PRE-PHASE: IDENTITY VALIDATION
	var exists int
	if req.LocationType == "branch" {
		err = tx.QueryRow(ctx, `SELECT 1 FROM branches WHERE id = $1`, req.LocationID).Scan(&exists)
	} else {
		err = tx.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1`, req.LocationID).Scan(&exists)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return apierr.NotFound(fmt.Sprintf("%s id %d not found", req.LocationType, req.LocationID))
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
		query = `SELECT item_id, stock FROM branch_items WHERE item_id = ANY($1) AND branch_id = $2 FOR UPDATE`
	} else {
		query = `SELECT item_id, stock FROM warehouse_items WHERE item_id = ANY($1) AND warehouse_id = $2 FOR UPDATE`
	}

	rows, err := tx.Query(ctx, query, itemIDs, req.LocationID)
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

	// 2. PHASE TWO: EXECUTE BUSINESS LOGIC
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
			batch.Queue(`UPDATE branch_items SET stock = stock + $1, updated_at = NOW() WHERE item_id = $2 AND branch_id = $3`,
				changeUnit, itemID, req.LocationID)
			batch.Queue(`INSERT INTO audit_stock (branch_item_id, change_unit, reason, user_id) 
				SELECT id, $1, $2, $3 FROM branch_items WHERE item_id = $4 AND branch_id = $5`,
				changeUnit, req.Reason, userID, itemID, req.LocationID)
		} else {
			batch.Queue(`UPDATE warehouse_items SET stock = stock + $1, updated_at = NOW() WHERE item_id = $2 AND warehouse_id = $3`,
				changeUnit, itemID, req.LocationID)
			batch.Queue(`INSERT INTO audit_stock (warehouse_item_id, change_unit, reason, user_id) 
				SELECT id, $1, $2, $3 FROM warehouse_items WHERE item_id = $4 AND warehouse_id = $5`,
				changeUnit, req.Reason, userID, itemID, req.LocationID)
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

// List returns a paginated, filtered list of transactions from the tenant's database.
func (r *TransactionRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.TransactionFilter) ([]model.Transaction, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	var args []any
	where := "WHERE TRUE"

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
		SELECT id, trxno, branch_id, warehouse_id, customer_id, supplier_id,
		       user_id, trans_type, total_amount, tax, discount, reference_transaction_id,
		       note, created_at,
		       COUNT(*) OVER() AS total_count
		FROM transactions
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.TransactionRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Transaction
		total int
	)
	for rows.Next() {
		var trx model.Transaction
		if err := rows.Scan(
			&trx.ID, &trx.TrxNo, &trx.BranchID, &trx.WarehouseID,
			&trx.CustomerID, &trx.SupplierID, &trx.UserID, &trx.TransType,
			&trx.TotalAmount, &trx.Tax, &trx.Discount, &trx.ReferenceTransactionID,
			&trx.Note, &trx.CreatedAt, &total,
		); err != nil {
			return nil, 0, fmt.Errorf("multidb.TransactionRepo.List scan: %w", err)
		}
		trx.TenantID = tenantID
		list = append(list, trx)
	}

	return list, total, rows.Err()
}

// GetByID fetches a single transaction with its details from the tenant's database.
func (r *TransactionRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Transaction, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var trx model.Transaction
	err = pool.QueryRow(ctx,
		`SELECT id, trxno, branch_id, warehouse_id, customer_id, supplier_id,
		        user_id, trans_type, total_amount, tax, discount, reference_transaction_id,
		        note, created_at
		 FROM transactions
		 WHERE id = $1`,
		id,
	).Scan(
		&trx.ID, &trx.TrxNo, &trx.BranchID, &trx.WarehouseID,
		&trx.CustomerID, &trx.SupplierID, &trx.UserID, &trx.TransType,
		&trx.TotalAmount, &trx.Tax, &trx.Discount, &trx.ReferenceTransactionID,
		&trx.Note, &trx.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("transaction id %d not found", id))
		}
		return nil, fmt.Errorf("multidb.TransactionRepo.GetByID (header): %w", err)
	}
	trx.TenantID = tenantID

	detailRows, err := pool.Query(ctx,
		`SELECT id, transaction_id, branch_item_id, warehouse_item_id,
		        quantity, cogs, price, subtotal
		 FROM transaction_detail
		 WHERE transaction_id = $1
		 ORDER BY id ASC`,
		trx.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("multidb.TransactionRepo.GetByID (details): %w", err)
	}
	defer detailRows.Close()

	for detailRows.Next() {
		var d model.TransactionDetail
		if err := detailRows.Scan(
			&d.ID, &d.TransactionID, &d.BranchItemID, &d.WarehouseItemID,
			&d.Quantity, &d.COGS, &d.Price, &d.Subtotal,
		); err != nil {
			return nil, fmt.Errorf("multidb.TransactionRepo.GetByID detail scan: %w", err)
		}
		trx.Details = append(trx.Details, d)
	}

	return &trx, detailRows.Err()
}

// ExecuteVoidTx handles the DB transaction for voiding a transaction in multi-DB mode.
func (r *TransactionRepo) ExecuteVoidTx(
	ctx context.Context, 
	tenantID int, 
	userID int, 
	originalTrxID int, 
	reason string,
	processFn func(data model.ProcessVoidData) error,
) (*model.Transaction, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Identify target IDs (Single or Pair if Transfer)
	var targets []int
	targets = append(targets, originalTrxID)

	// Preliminary check
	var transType model.TransactionType
	var trxNo string
	err = tx.QueryRow(ctx, `SELECT trans_type, trxno FROM transactions WHERE id = $1`, originalTrxID).Scan(&transType, &trxNo)
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
			err = tx.QueryRow(ctx, `SELECT id FROM transactions WHERE trxno = $1`, partnerNo).Scan(&partnerID)
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
			 WHERE id = $1 FOR UPDATE`,
			targetID,
		).Scan(
			&original.ID, &original.TrxNo, &original.BranchID, &original.WarehouseID,
			&original.CustomerID, &original.SupplierID, &original.UserID, &original.TransType,
			&original.TotalAmount, &original.Tax, &original.Discount, &original.Note, &original.CreatedAt,
		)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, apierr.NotFound(fmt.Sprintf("transaction %d not found", targetID))
			}
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
		voidTrxNo := fmt.Sprintf("VOID-%s", time.Now().Format("20060102150405.000")) 
		if targetID != originalTrxID {
			voidTrxNo += "-PAIR"
		}
		
		var voidID int
		err = tx.QueryRow(ctx,
			`INSERT INTO transactions (trxno, branch_id, warehouse_id, customer_id, supplier_id, user_id, trans_type, total_amount, tax, discount, reference_transaction_id, note)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 RETURNING id`,
			voidTrxNo, original.BranchID, original.WarehouseID, original.CustomerID, original.SupplierID, userID, model.TxVoid, original.TotalAmount, original.Tax, original.Discount, original.ID, reason,
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
				batch.Queue(`INSERT INTO branch_cashflow (branch_id, transaction_id, flow_type, direction, amount)
						 VALUES ($1, $2, $3, $4, $5)`,
					*original.BranchID, voidID, model.CflowVoid, "OUT", original.TotalAmount)
			case model.TxReturn:
				batch.Queue(`INSERT INTO branch_cashflow (branch_id, transaction_id, flow_type, direction, amount)
						 VALUES ($1, $2, $3, $4, $5)`,
					*original.BranchID, voidID, model.CflowVoid, "IN", original.TotalAmount)
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
