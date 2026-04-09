package multidb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
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

	// 1. PHASE ONE: BULK READ
	branchItemIDs := make([]int, 0, len(req.Items))
	for _, item := range req.Items {
		branchItemIDs = append(branchItemIDs, item.BranchItemID)
	}

	loadedDbItems := make(map[int]model.ProcessSaleItem)

	query := `
		SELECT b.id, b.stock, i.cost, i.price 
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
			cost  float64
			price float64
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
