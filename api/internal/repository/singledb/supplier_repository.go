package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

type SupplierRepo struct {
	db *pgxpool.Pool
}

func NewSupplierRepo(db *pgxpool.Pool) *SupplierRepo {
	return &SupplierRepo{db: db}
}

func (r *SupplierRepo) Create(ctx context.Context, tenantID int, req dto.CreateSupplierRequest) (*model.Supplier, error) {
	var phone, address *string
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.Address != "" {
		address = &req.Address
	}

	var s model.Supplier
	err := r.db.QueryRow(ctx,
		`INSERT INTO suppliers (tenant_id, name, phone, address)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, tenant_id, name, phone, address, created_at`,
		tenantID, req.Name, phone, address,
	).Scan(&s.ID, &s.TenantID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.SupplierRepo.Create: %w", err)
	}
	return &s, nil
}

func (r *SupplierRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Supplier, error) {
	var s model.Supplier
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, phone, address, created_at
		 FROM suppliers
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&s.ID, &s.TenantID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.SupplierRepo.GetByID: %w", err)
	}
	return &s, nil
}

func (r *SupplierRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.SupplierFilter) ([]model.Supplier, int, error) {
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

	// Query 1: Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM suppliers
		%s`, where)

	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.SupplierRepo.List count: %w", err)
	}

	if total == 0 {
		return []model.Supplier{}, 0, nil
	}

	// Query 2: Get data
	dataArgs := append(args, q.Limit, q.Offset())
	limitIdx := len(dataArgs) - 1
	offsetIdx := len(dataArgs)

	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, phone, address, created_at
		FROM suppliers
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.SupplierRepo.List data: %w", err)
	}
	defer rows.Close()

	var list []model.Supplier
	for rows.Next() {
		var s model.Supplier
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("singledb.SupplierRepo.List scan: %w", err)
		}
		list = append(list, s)
	}
	return list, total, rows.Err()
}

func (r *SupplierRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateSupplierRequest) (*model.Supplier, error) {
	var s model.Supplier
	err := r.db.QueryRow(ctx,
		`UPDATE suppliers
		 SET name = COALESCE(NULLIF($1,''), name),
		     phone = COALESCE(NULLIF($2,''), phone),
		     address = COALESCE(NULLIF($3,''), address)
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, tenant_id, name, phone, address, created_at`,
		req.Name, req.Phone, req.Address, id, tenantID,
	).Scan(&s.ID, &s.TenantID, &s.Name, &s.Phone, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.SupplierRepo.Update: %w", err)
	}
	return &s, nil
}

func (r *SupplierRepo) Delete(ctx context.Context, tenantID, id int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM suppliers WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("singledb.SupplierRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("singledb.SupplierRepo.Delete: supplier %d not found", id)
	}
	return nil
}
