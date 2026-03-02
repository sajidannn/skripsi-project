package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/model"
)

// BranchRepo implements repository.BranchRepository for single-DB mode.
type BranchRepo struct {
	db *pgxpool.Pool
}

// NewBranchRepo creates a new BranchRepo backed by the shared pool.
func NewBranchRepo(db *pgxpool.Pool) *BranchRepo {
	return &BranchRepo{db: db}
}

// Create inserts a branch row for the given tenant.
func (r *BranchRepo) Create(ctx context.Context, tenantID int, req model.CreateBranchRequest) (*model.Branch, error) {
	var b model.Branch
	err := r.db.QueryRow(ctx,
		`INSERT INTO branches (tenant_id, name, phone, address, opening_balance)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, name, phone, address, opening_balance, created_at`,
		tenantID, req.Name, req.Phone, req.Address, req.OpeningBalance,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.BranchRepo.Create: %w", err)
	}
	return &b, nil
}

// GetByID fetches a branch by ID scoped to tenantID.
func (r *BranchRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Branch, error) {
	var b model.Branch
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, phone, address, opening_balance, created_at
		 FROM branches
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.BranchRepo.GetByID: %w", err)
	}
	return &b, nil
}

// List returns all branches for a tenant.
func (r *BranchRepo) List(ctx context.Context, tenantID int) ([]model.Branch, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, name, phone, address, opening_balance, created_at
		 FROM branches
		 WHERE tenant_id = $1
		 ORDER BY id`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("singledb.BranchRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.Branch
	for rows.Next() {
		var b model.Branch
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("singledb.BranchRepo.List scan: %w", err)
		}
		list = append(list, b)
	}
	return list, rows.Err()
}
