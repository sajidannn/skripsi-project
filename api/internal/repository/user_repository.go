package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// UserRepository is the data-access contract for users.
type UserRepository interface {
	// Create inserts a new user with the given (already-hashed) password.
	Create(ctx context.Context, tenantID int, req dto.CreateUserRequest, hashedPassword string) (*model.User, error)

	// GetByID fetches a single user scoped to the tenant.
	GetByID(ctx context.Context, tenantID, id int) (*model.User, error)

	// GetByEmail fetches a user by email and returns the stored hashed password
	// alongside the model. Used exclusively by the login flow.
	GetByEmail(ctx context.Context, tenantID int, email string) (*model.User, string, error)

	// List returns all users that belong to the tenant.
	List(ctx context.Context, tenantID int) ([]model.User, error)

	// Update modifies name and/or role of an existing user.
	Update(ctx context.Context, tenantID, id int, req dto.UpdateUserRequest) (*model.User, error)

	// Delete removes a user scoped to the tenant.
	Delete(ctx context.Context, tenantID, id int) error
}
