package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// BranchRepo implements repository.BranchRepository for multi-DB mode.
type BranchRepo struct {
	mgr *multidb.Manager
}

// NewBranchRepo creates a new BranchRepo backed by the tenant Manager.
func NewBranchRepo(mgr *multidb.Manager) *BranchRepo {
	return &BranchRepo{mgr: mgr}
}

// Create inserts a branch row into the tenant's own database.
func (r *BranchRepo) Create(ctx context.Context, tenantID int, req dto.CreateBranchRequest) (*model.Branch, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var b model.Branch
	err = pool.QueryRow(ctx,
		`INSERT INTO branches (name, phone, address, opening_balance)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, phone, address, opening_balance, created_at`,
		req.Name, req.Phone, req.Address, req.OpeningBalance,
	).Scan(&b.ID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.BranchRepo.Create: %w", err)
	}
	b.TenantID = tenantID
	return &b, nil
}

// GetByID fetches a branch by ID from the tenant's database.
func (r *BranchRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Branch, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var b model.Branch
	err = pool.QueryRow(ctx,
		`SELECT id, name, phone, address, opening_balance, created_at
		 FROM branches WHERE id = $1`,
		id,
	).Scan(&b.ID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.BranchRepo.GetByID: %w", err)
	}
	b.TenantID = tenantID
	return &b, nil
}

// List returns all branches from the tenant's database.
func (r *BranchRepo) List(ctx context.Context, tenantID int) ([]model.Branch, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx,
		`SELECT id, name, phone, address, opening_balance, created_at
		 FROM branches ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("multidb.BranchRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.Branch
	for rows.Next() {
		var b model.Branch
		if err := rows.Scan(&b.ID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("multidb.BranchRepo.List scan: %w", err)
		}
		b.TenantID = tenantID
		list = append(list, b)
	}
	return list, rows.Err()
}
