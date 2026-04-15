package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

type SupplierRepo struct {
	mgr *multidb.Manager
}

func NewSupplierRepo(mgr *multidb.Manager) *SupplierRepo {
	return &SupplierRepo{mgr: mgr}
}

func (r *SupplierRepo) Create(ctx context.Context, tenantID int, req dto.CreateSupplierRequest) (*model.Supplier, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var phone, address *string
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.Address != "" {
		address = &req.Address
	}

	var s model.Supplier
	err = pool.QueryRow(ctx,
		`INSERT INTO suppliers (name, phone, address)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, phone, address, created_at`,
		req.Name, phone, address,
	).Scan(&s.ID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.SupplierRepo.Create: %w", err)
	}
	s.TenantID = tenantID
	return &s, nil
}

func (r *SupplierRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Supplier, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var s model.Supplier
	err = pool.QueryRow(ctx,
		`SELECT id, name, phone, address, created_at
		 FROM suppliers
		 WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.SupplierRepo.GetByID: %w", err)
	}
	s.TenantID = tenantID
	return &s, nil
}

func (r *SupplierRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.SupplierFilter) ([]model.Supplier, int, error) {
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
		SELECT id, name, phone, address, created_at,
		       COUNT(*) OVER() AS total_count
		FROM suppliers
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.SupplierRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Supplier
		total int
	)
	for rows.Next() {
		var s model.Supplier
		if err := rows.Scan(&s.ID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.SupplierRepo.List scan: %w", err)
		}
		s.TenantID = tenantID
		list = append(list, s)
	}
	return list, total, rows.Err()
}

func (r *SupplierRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateSupplierRequest) (*model.Supplier, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var s model.Supplier
	err = pool.QueryRow(ctx,
		`UPDATE suppliers
		 SET name = COALESCE(NULLIF($1,''), name),
		     phone = COALESCE(NULLIF($2,''), phone),
		     address = COALESCE(NULLIF($3,''), address)
		 WHERE id = $4
		 RETURNING id, name, phone, address, created_at`,
		req.Name, req.Phone, req.Address, id,
	).Scan(&s.ID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.SupplierRepo.Update: %w", err)
	}
	s.TenantID = tenantID
	return &s, nil
}

func (r *SupplierRepo) Delete(ctx context.Context, tenantID, id int) error {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return err
	}

	tag, err := pool.Exec(ctx, `DELETE FROM suppliers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("multidb.SupplierRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("multidb.SupplierRepo.Delete: supplier %d not found", id)
	}
	return nil
}
