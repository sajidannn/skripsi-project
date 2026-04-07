package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// UserRepo implements repository.UserRepository for multi-DB mode.
type UserRepo struct {
	mgr *multidb.Manager
}

// NewUserRepo creates a new UserRepo backed by the tenant Manager.
func NewUserRepo(mgr *multidb.Manager) *UserRepo {
	return &UserRepo{mgr: mgr}
}

// Create inserts a new user with an already-hashed password into the tenant's database.
func (r *UserRepo) Create(ctx context.Context, tenantID int, req dto.CreateUserRequest, hashedPassword string) (*model.User, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var u model.User
	err = pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, email, role, created_at`,
		req.Name, req.Email, hashedPassword, req.Role,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.UserRepo.Create: %w", err)
	}
	u.TenantID = tenantID
	return &u, nil
}

// GetByID fetches a single user from the tenant's database.
func (r *UserRepo) GetByID(ctx context.Context, tenantID, id int) (*model.User, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var u model.User
	err = pool.QueryRow(ctx,
		`SELECT id, name, email, role, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.UserRepo.GetByID: %w", err)
	}
	u.TenantID = tenantID
	return &u, nil
}

// GetByEmail fetches a user by email and returns the stored hashed password.
// Used exclusively during login.
func (r *UserRepo) GetByEmail(ctx context.Context, tenantID int, email string) (*model.User, string, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}

	var u model.User
	var hashedPassword string
	err = pool.QueryRow(ctx,
		`SELECT id, name, email, role, created_at, password
		 FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &hashedPassword)
	if err != nil {
		return nil, "", fmt.Errorf("multidb.UserRepo.GetByEmail: %w", err)
	}
	u.TenantID = tenantID
	return &u, hashedPassword, nil
}

// List returns all users from the tenant's database.
func (r *UserRepo) List(ctx context.Context, tenantID int) ([]model.User, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx,
		`SELECT id, name, email, role, created_at
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("multidb.UserRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("multidb.UserRepo.List scan: %w", err)
		}
		u.TenantID = tenantID
		list = append(list, u)
	}
	return list, rows.Err()
}

// Update modifies name and/or role of an existing user.
func (r *UserRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateUserRequest) (*model.User, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var u model.User
	err = pool.QueryRow(ctx,
		`UPDATE users
		 SET name = COALESCE(NULLIF($1,''), name),
		     role = COALESCE(NULLIF($2,''), role)
		 WHERE id = $3
		 RETURNING id, name, email, role, created_at`,
		req.Name, req.Role, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.UserRepo.Update: %w", err)
	}
	u.TenantID = tenantID
	return &u, nil
}

// Delete removes a user from the tenant's database.
func (r *UserRepo) Delete(ctx context.Context, tenantID, id int) error {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return err
	}

	tag, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("multidb.UserRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("multidb.UserRepo.Delete: user %d not found", id)
	}
	return nil
}
