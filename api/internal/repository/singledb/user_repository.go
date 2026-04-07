package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// UserRepo implements repository.UserRepository for single-DB mode.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo backed by the shared pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user with an already-hashed password.
func (r *UserRepo) Create(ctx context.Context, tenantID int, req dto.CreateUserRequest, hashedPassword string) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (tenant_id, name, email, password, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, name, email, role, created_at`,
		tenantID, req.Name, req.Email, hashedPassword, req.Role,
	).Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.UserRepo.Create: %w", err)
	}
	return &u, nil
}

// GetByID fetches a single user scoped to the tenant.
func (r *UserRepo) GetByID(ctx context.Context, tenantID, id int) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, email, role, created_at
		 FROM users
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.UserRepo.GetByID: %w", err)
	}
	return &u, nil
}

// GetByEmail fetches a user by email and returns the stored hashed password.
// Used exclusively during login.
func (r *UserRepo) GetByEmail(ctx context.Context, tenantID int, email string) (*model.User, string, error) {
	var u model.User
	var hashedPassword string
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, email, role, created_at, password
		 FROM users
		 WHERE email = $1 AND tenant_id = $2`,
		email, tenantID,
	).Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &hashedPassword)
	if err != nil {
		return nil, "", fmt.Errorf("singledb.UserRepo.GetByEmail: %w", err)
	}
	return &u, hashedPassword, nil
}

// List returns all users for the tenant, ordered by id.
func (r *UserRepo) List(ctx context.Context, tenantID int) ([]model.User, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, name, email, role, created_at
		 FROM users
		 WHERE tenant_id = $1
		 ORDER BY id`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("singledb.UserRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("singledb.UserRepo.List scan: %w", err)
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// Update modifies name and/or role of an existing user.
func (r *UserRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateUserRequest) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx,
		`UPDATE users
		 SET name = COALESCE(NULLIF($1,''), name),
		     role = COALESCE(NULLIF($2,''), role)
		 WHERE id = $3 AND tenant_id = $4
		 RETURNING id, tenant_id, name, email, role, created_at`,
		req.Name, req.Role, id, tenantID,
	).Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.UserRepo.Update: %w", err)
	}
	return &u, nil
}

// Delete removes a user scoped to the tenant.
func (r *UserRepo) Delete(ctx context.Context, tenantID, id int) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM users WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("singledb.UserRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("singledb.UserRepo.Delete: user %d not found", id)
	}
	return nil
}
