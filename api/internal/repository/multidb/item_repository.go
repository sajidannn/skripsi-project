package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// ItemRepo implements repository.ItemRepository for multi-DB mode.
type ItemRepo struct {
	mgr *multidb.Manager
}

// NewItemRepo creates a new ItemRepo backed by the tenant Manager.
func NewItemRepo(mgr *multidb.Manager) *ItemRepo {
	return &ItemRepo{mgr: mgr}
}

// Create inserts a new item into the tenant's own database.
func (r *ItemRepo) Create(ctx context.Context, tenantID int, req dto.CreateItemRequest) (*model.Item, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var it model.Item
	err = pool.QueryRow(ctx,
		`INSERT INTO items (name, sku, price, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, sku, price, COALESCE(description,''), created_at, updated_at`,
		req.Name, req.SKU, req.Price, req.Description,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.ItemRepo.Create: %w", err)
	}
	it.TenantID = tenantID
	return &it, nil
}

// GetByID fetches a single item from the tenant's database.
func (r *ItemRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Item, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var it model.Item
	err = pool.QueryRow(ctx,
		`SELECT id, name, sku, price, COALESCE(description,''), created_at, updated_at
		 FROM items WHERE id = $1`,
		id,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.ItemRepo.GetByID: %w", err)
	}
	it.TenantID = tenantID
	return &it, nil
}

// List returns all items from the tenant's database.
func (r *ItemRepo) List(ctx context.Context, tenantID int) ([]model.Item, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx,
		`SELECT id, name, sku, price, COALESCE(description,''), created_at, updated_at
		 FROM items ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("multidb.ItemRepo.List: %w", err)
	}
	defer rows.Close()

	var list []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, fmt.Errorf("multidb.ItemRepo.List scan: %w", err)
		}
		it.TenantID = tenantID
		list = append(list, it)
	}
	return list, rows.Err()
}

// Update modifies an existing item in the tenant's database.
// SKU is only updated when the request provides a non-empty value.
func (r *ItemRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateItemRequest) (*model.Item, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var it model.Item
	err = pool.QueryRow(ctx,
		`UPDATE items
		 SET name        = COALESCE(NULLIF($1,''), name),
		     sku         = COALESCE(NULLIF($2,''), sku),
		     price       = CASE WHEN $3 > 0 THEN $3 ELSE price END,
		     description = COALESCE(NULLIF($4,''), description),
		     updated_at  = NOW()
		 WHERE id = $5
		 RETURNING id, name, sku, price, COALESCE(description,''), created_at, updated_at`,
		req.Name, req.SKU, req.Price, req.Description, id,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.ItemRepo.Update: %w", err)
	}
	it.TenantID = tenantID
	return &it, nil
}

// Delete removes an item from the tenant's database.
func (r *ItemRepo) Delete(ctx context.Context, tenantID, id int) error {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return err
	}

	tag, err := pool.Exec(ctx, `DELETE FROM items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("multidb.ItemRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("multidb.ItemRepo.Delete: item %d not found", id)
	}
	return nil
}
