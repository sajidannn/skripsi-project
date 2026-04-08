package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

type CustomerRepo struct {
	mgr *multidb.Manager
}

func NewCustomerRepo(mgr *multidb.Manager) *CustomerRepo {
	return &CustomerRepo{mgr: mgr}
}

func (r *CustomerRepo) Create(ctx context.Context, tenantID int, req dto.CreateCustomerRequest) (*model.Customer, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var phone, email *string
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.Email != "" {
		email = &req.Email
	}

	var c model.Customer
	err = pool.QueryRow(ctx,
		`INSERT INTO customers (branch_id, name, phone, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, branch_id, name, phone, email, created_at`,
		req.BranchID, req.Name, phone, email,
	).Scan(&c.ID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.CustomerRepo.Create: %w", err)
	}
	c.TenantID = tenantID
	return &c, nil
}

func (r *CustomerRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Customer, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var c model.Customer
	err = pool.QueryRow(ctx,
		`SELECT id, branch_id, name, phone, email, created_at
		 FROM customers
		 WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.CustomerRepo.GetByID: %w", err)
	}
	c.TenantID = tenantID
	return &c, nil
}

func (r *CustomerRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.CustomerFilter) ([]model.Customer, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	var args []any
	where := "WHERE TRUE"

	if f.BranchID > 0 {
		args = append(args, f.BranchID)
		where += fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (name ILIKE $%d OR phone ILIKE $%d OR email ILIKE $%d)", n, n, n)
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
		SELECT id, branch_id, name, phone, email, created_at,
		       COUNT(*) OVER() AS total_count
		FROM customers
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.CustomerRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Customer
		total int
	)
	for rows.Next() {
		var c model.Customer
		if err := rows.Scan(&c.ID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.CustomerRepo.List scan: %w", err)
		}
		c.TenantID = tenantID
		list = append(list, c)
	}
	return list, total, rows.Err()
}

func (r *CustomerRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateCustomerRequest) (*model.Customer, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var c model.Customer
	err = pool.QueryRow(ctx,
		`UPDATE customers
		 SET name = COALESCE(NULLIF($1,''), name),
		     phone = COALESCE(NULLIF($2,''), phone),
		     email = COALESCE(NULLIF($3,''), email)
		 WHERE id = $4
		 RETURNING id, branch_id, name, phone, email, created_at`,
		req.Name, req.Phone, req.Email, id,
	).Scan(&c.ID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.CustomerRepo.Update: %w", err)
	}
	c.TenantID = tenantID
	return &c, nil
}

func (r *CustomerRepo) Delete(ctx context.Context, tenantID, id int) error {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return err
	}

	tag, err := pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("multidb.CustomerRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("multidb.CustomerRepo.Delete: customer %d not found", id)
	}
	return nil
}
