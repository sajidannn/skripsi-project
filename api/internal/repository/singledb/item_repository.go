package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// ItemRepo implements repository.ItemRepository for single-DB mode.
type ItemRepo struct {
	db *pgxpool.Pool
}

// NewItemRepo creates a new ItemRepo backed by the shared pool.
func NewItemRepo(db *pgxpool.Pool) *ItemRepo {
	return &ItemRepo{db: db}
}

// Create inserts a new item for the given tenant.
func (r *ItemRepo) Create(ctx context.Context, tenantID int, req dto.CreateItemRequest) (*model.Item, error) {
	var it model.Item
	err := r.db.QueryRow(ctx,
		`INSERT INTO items (tenant_id, name, sku, price, description)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, name, sku, price, COALESCE(description,''), created_at, updated_at`,
		tenantID, req.Name, req.SKU, req.Price, req.Description,
	).Scan(&it.ID, &it.TenantID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.ItemRepo.Create: %w", err)
	}
	return &it, nil
}

// GetByID fetches a single item scoped to the tenant.
func (r *ItemRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Item, error) {
	var it model.Item
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, sku, price, COALESCE(description,''), created_at, updated_at
		 FROM items
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&it.ID, &it.TenantID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.ItemRepo.GetByID: %w", err)
	}
	return &it, nil
}

// List returns a paginated, optionally-filtered list of items for the tenant.
// It uses COUNT(*) OVER() to fetch total in a single query trip.
// All filter conditions are built dynamically and bound as parameters (no string interpolation)
// so the query is safe from SQL injection.
func (r *ItemRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.ItemFilter) ([]model.Item, int, error) {
	// --- build WHERE clause dynamically ---
	// Start with the mandatory tenant predicate.
	args := []any{tenantID}
	where := "WHERE tenant_id = $1"

	// search: partial match across name, sku, and description via ILIKE.
	// ILIKE is safer and simpler than full-text search when no GIN/tsvector index exists.
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (name ILIKE $%d OR sku ILIKE $%d OR description ILIKE $%d)", n, n, n)
	}

	// exact SKU match
	if f.SKU != "" {
		args = append(args, f.SKU)
		where += fmt.Sprintf(" AND sku = $%d", len(args))
	}

	// price range
	if f.MinPrice > 0 {
		args = append(args, f.MinPrice)
		where += fmt.Sprintf(" AND price >= $%d", len(args))
	}
	if f.MaxPrice > 0 {
		args = append(args, f.MaxPrice)
		where += fmt.Sprintf(" AND price <= $%d", len(args))
	}

	// created_at range
	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		where += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		where += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}

	// --- pagination args (always last) ---
	args = append(args, q.Limit, q.Offset())
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, sku, price, COALESCE(description,''), created_at, updated_at,
		       COUNT(*) OVER() AS total_count
		FROM items
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.ItemRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Item
		total int
	)
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.TenantID, &it.Name, &it.SKU, &it.Price,
			&it.Description, &it.CreatedAt, &it.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("singledb.ItemRepo.List scan: %w", err)
		}
		list = append(list, it)
	}
	return list, total, rows.Err()
}

// Update modifies an existing item scoped to the tenant.
// SKU is only updated when the request provides a non-empty value.
func (r *ItemRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateItemRequest) (*model.Item, error) {
	var it model.Item
	err := r.db.QueryRow(ctx,
		`UPDATE items
		 SET name        = COALESCE(NULLIF($1,''), name),
		     sku         = COALESCE(NULLIF($2,''), sku),
		     price       = CASE WHEN $3 > 0 THEN $3 ELSE price END,
		     description = COALESCE(NULLIF($4,''), description),
		     updated_at  = NOW()
		 WHERE id = $5 AND tenant_id = $6
		 RETURNING id, tenant_id, name, sku, price, COALESCE(description,''), created_at, updated_at`,
		req.Name, req.SKU, req.Price, req.Description, id, tenantID,
	).Scan(&it.ID, &it.TenantID, &it.Name, &it.SKU, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.ItemRepo.Update: %w", err)
	}
	return &it, nil
}

// Delete removes an item scoped to the tenant.
func (r *ItemRepo) Delete(ctx context.Context, tenantID, id int) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM items WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("singledb.ItemRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("singledb.ItemRepo.Delete: item %d not found", id)
	}
	return nil
}
