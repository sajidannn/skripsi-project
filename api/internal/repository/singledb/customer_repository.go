package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

type CustomerRepo struct {
	db *pgxpool.Pool
}

func NewCustomerRepo(db *pgxpool.Pool) *CustomerRepo {
	return &CustomerRepo{db: db}
}

func (r *CustomerRepo) Create(ctx context.Context, tenantID int, req dto.CreateCustomerRequest) (*model.Customer, error) {
	// Convert empty strings to nil for DB insertion
	var phone, email *string
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.Email != "" {
		email = &req.Email
	}

	var c model.Customer
	err := r.db.QueryRow(ctx,
		`INSERT INTO customers (tenant_id, branch_id, name, phone, email)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, branch_id, name, phone, email, created_at`,
		tenantID, req.BranchID, req.Name, phone, email,
	).Scan(&c.ID, &c.TenantID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.CustomerRepo.Create: %w", err)
	}
	return &c, nil
}

func (r *CustomerRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Customer, error) {
	var c model.Customer
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, branch_id, name, phone, email, created_at
		 FROM customers
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&c.ID, &c.TenantID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.CustomerRepo.GetByID: %w", err)
	}
	return &c, nil
}

func (r *CustomerRepo) List(ctx context.Context, tenantID, branchID int) ([]model.Customer, error) {
	query := `SELECT id, tenant_id, branch_id, name, phone, email, created_at FROM customers WHERE tenant_id = $1`
	args := []any{tenantID}

	if branchID > 0 {
		query += ` AND branch_id = $2`
		args = append(args, branchID)
	}
	query += ` ORDER BY name ASC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("singledb.CustomerRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.Customer
	for rows.Next() {
		var c model.Customer
		if err := rows.Scan(&c.ID, &c.TenantID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("singledb.CustomerRepo.List scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *CustomerRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateCustomerRequest) (*model.Customer, error) {
	var c model.Customer
	// Uses NULLIF so that an empty string ignores the COALESCE update, keeping the old value.
	// If the user wants to truly remove the phone, a separate endpoint or nullable input struct is needed.
	// For now this follows the previous update pattern.
	err := r.db.QueryRow(ctx,
		`UPDATE customers
		 SET name = COALESCE(NULLIF($1,''), name),
		     phone = COALESCE(NULLIF($2,''), phone),
		     email = COALESCE(NULLIF($3,''), email)
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, tenant_id, branch_id, name, phone, email, created_at`,
		req.Name, req.Phone, req.Email, id, tenantID,
	).Scan(&c.ID, &c.TenantID, &c.BranchID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.CustomerRepo.Update: %w", err)
	}
	return &c, nil
}

func (r *CustomerRepo) Delete(ctx context.Context, tenantID, id int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM customers WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("singledb.CustomerRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("singledb.CustomerRepo.Delete: customer %d not found", id)
	}
	return nil
}
