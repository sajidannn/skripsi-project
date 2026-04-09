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
