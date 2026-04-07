package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
	"golang.org/x/crypto/bcrypt"
)

// UserService handles business logic for users and authentication.
type UserService struct {
	repo      repository.UserRepository
	jwtSecret string
}

// NewUserService constructs a UserService.
func NewUserService(repo repository.UserRepository, jwtSecret string) *UserService {
	return &UserService{repo: repo, jwtSecret: jwtSecret}
}

// Login validates credentials and returns a signed JWT on success.
func (s *UserService) Login(ctx context.Context, req dto.LoginRequest) (string, error) {
	user, hashed, err := s.repo.GetByEmail(ctx, req.TenantID, req.Email)
	if err != nil {
		return "", fmt.Errorf("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)); err != nil {
		return "", fmt.Errorf("invalid email or password")
	}

	claims := jwt.MapClaims{
		"tenant_id": user.TenantID,
		"user_id":   user.ID,
		"role":      user.Role,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("UserService.Login: failed to sign token: %w", err)
	}
	return signed, nil
}

// Create hashes the password then persists the new user.
func (s *UserService) Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserService.Create: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("UserService.Create: failed to hash password: %w", err)
	}

	return s.repo.Create(ctx, tenantID, req, string(hashed))
}

// GetByID retrieves a single user, scoped to the tenant in ctx.
func (s *UserService) GetByID(ctx context.Context, id int) (*model.User, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List returns all users for the tenant in ctx.
func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID)
}

// Update modifies name and/or role of an existing user.
func (s *UserService) Update(ctx context.Context, id int, req dto.UpdateUserRequest) (*model.User, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserService.Update: %w", err)
	}
	return s.repo.Update(ctx, tenantID, id, req)
}

// Delete removes a user, scoped to the tenant in ctx.
func (s *UserService) Delete(ctx context.Context, id int) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("UserService.Delete: %w", err)
	}
	return s.repo.Delete(ctx, tenantID, id)
}
