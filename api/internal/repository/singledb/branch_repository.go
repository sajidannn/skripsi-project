package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
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
func (r *BranchRepo) Create(ctx context.Context, tenantID int, req dto.CreateBranchRequest) (*model.Branch, error) {
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

func (r *BranchRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateBranchRequest) (*model.Branch, error) {
	var b model.Branch
	err := r.db.QueryRow(ctx,
		`UPDATE branches
		 SET name = COALESCE($1, name),
			phone = COALESCE($2, phone),
			address = COALESCE($3, address),
			opening_balance = COALESCE($4, opening_balance)
		 WHERE id = $5 AND tenant_id = $6
		 RETURNING id, tenant_id, name, phone, address, opening_balance, created_at`,
		req.Name, req.Phone, req.Address, req.OpeningBalance, id, tenantID,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.BranchRepo.Update: %w", err)
	}
	return &b, nil
}

// List returns a paginated, filtered list of branches for a tenant.
func (r *BranchRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.BranchFilter) ([]model.Branch, int, error) {
	args := []any{tenantID}
	where := "WHERE tenant_id = $1"

	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (name ILIKE $%d OR phone ILIKE $%d OR address ILIKE $%d)", n, n, n)
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
		SELECT id, tenant_id, name, phone, address, opening_balance, created_at,
		       COUNT(*) OVER() AS total_count
		FROM branches
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.BranchRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Branch
		total int
	)
	for rows.Next() {
		var b model.Branch
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("singledb.BranchRepo.List scan: %w", err)
		}
		list = append(list, b)
	}
	return list, total, rows.Err()
}
