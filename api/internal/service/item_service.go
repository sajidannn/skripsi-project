package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// ItemService handles business logic for catalogue items.
type ItemService struct {
	repo repository.ItemRepository
}

// NewItemService constructs an ItemService.
func NewItemService(repo repository.ItemRepository) *ItemService {
	return &ItemService{repo: repo}
}

// Create validates the request, auto-generates SKU when omitted, and delegates to the repository.
func (s *ItemService) Create(ctx context.Context, req dto.CreateItemRequest) (*model.Item, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ItemService.Create: %w", err)
	}
	if req.SKU == "" {
		req.SKU = generateSKU()
	}
	return s.repo.Create(ctx, tenantID, req)
}

// GetByID retrieves a single item, scoped to the tenant in ctx.
func (s *ItemService) GetByID(ctx context.Context, id int) (*model.Item, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ItemService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List returns a paginated, filtered list of items for the tenant in ctx.
func (s *ItemService) List(ctx context.Context, q dto.PageQuery, f dto.ItemFilter) ([]model.Item, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("ItemService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID, q, f)
}

// Update modifies an existing item, scoped to the tenant in ctx.
func (s *ItemService) Update(ctx context.Context, id int, req dto.UpdateItemRequest) (*model.Item, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ItemService.Update: %w", err)
	}
	return s.repo.Update(ctx, tenantID, id, req)
}

// Delete removes an item, scoped to the tenant in ctx.
func (s *ItemService) Delete(ctx context.Context, id int) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("ItemService.Delete: %w", err)
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// generateSKU produces a unique SKU in the format ITEM-{YYYYMMDD}-{6 random uppercase alphanumeric chars}.
// Example: ITEM-20260407-K9XM3P
func generateSKU() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	date := time.Now().Format("20060102")
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("ITEM-%s-%s", date, string(b))
}
