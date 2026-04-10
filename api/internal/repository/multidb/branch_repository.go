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

// List returns a paginated, filtered list of branches from the tenant's database.
func (r *BranchRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.BranchFilter) ([]model.Branch, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	var args []any
	where := "WHERE TRUE"

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
		SELECT id, name, phone, address, opening_balance, created_at,
		       COUNT(*) OVER() AS total_count
		FROM branches
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.BranchRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Branch
		total int
	)
	for rows.Next() {
		var b model.Branch
		if err := rows.Scan(&b.ID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.BranchRepo.List scan: %w", err)
		}
		b.TenantID = tenantID
		list = append(list, b)
	}
	return list, total, rows.Err()
}
func (r *BranchRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateBranchRequest) (*model.Branch, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var b model.Branch
	err = pool.QueryRow(ctx,
		`UPDATE branches
		 SET name = COALESCE($1, name),
    phone = COALESCE($2, phone),
    address = COALESCE($3, address),
    opening_balance = COALESCE($4, opening_balance)
 WHERE id = $5
 RETURNING id, name, phone, address, opening_balance, created_at`,
req.Name, req.Phone, req.Address, req.OpeningBalance, id,
).Scan(&b.ID, &b.Name, &b.Phone, &b.Address, &b.OpeningBalance, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.BranchRepo.Update: %w", err)
	}
	b.TenantID = tenantID
	return &b, nil
}
