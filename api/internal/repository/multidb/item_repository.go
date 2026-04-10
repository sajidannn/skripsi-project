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
		`INSERT INTO items (name, sku, cost, price, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, sku, cost, price, COALESCE(description,''), created_at, updated_at`,
		req.Name, req.SKU, req.Cost, req.Price, req.Description,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Cost, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
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
		`SELECT id, name, sku, cost, price, COALESCE(description,''), created_at, updated_at
		 FROM items WHERE id = $1`,
		id,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Cost, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.ItemRepo.GetByID: %w", err)
	}
	it.TenantID = tenantID
	return &it, nil
}

// List returns a paginated, optionally-filtered list of items from the tenant's database.
// Tenant isolation is handled at the connection level (multi-DB), so there is no
// tenant_id column in this schema. All filters are bound as parameters (SQLI-safe).
func (r *ItemRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.ItemFilter) ([]model.Item, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	// --- build WHERE clause dynamically ---
	// Multi-DB schema has no tenant_id column, so we start with a tautology.
	var args []any
	where := "WHERE TRUE"

	// search: ILIKE across name, sku, description
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

	// cost range
	if !f.MinCost.IsZero() {
		args = append(args, f.MinCost)
		where += fmt.Sprintf(" AND cost >= $%d", len(args))
	}
	if !f.MaxCost.IsZero() {
		args = append(args, f.MaxCost)
		where += fmt.Sprintf(" AND cost <= $%d", len(args))
	}

	// price range
	if !f.MinPrice.IsZero() {
		args = append(args, f.MinPrice)
		where += fmt.Sprintf(" AND price >= $%d", len(args))
	}
	if !f.MaxPrice.IsZero() {
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
		SELECT id, name, sku, cost, price, COALESCE(description,''), created_at, updated_at,
		       COUNT(*) OVER() AS total_count
		FROM items
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.ItemRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Item
		total int
	)
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.Name, &it.SKU, &it.Cost, &it.Price,
			&it.Description, &it.CreatedAt, &it.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.ItemRepo.List scan: %w", err)
		}
		it.TenantID = tenantID
		list = append(list, it)
	}
	return list, total, rows.Err()
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
			 cost		 = CASE WHEN NOT $3::numeric = 0 THEN $3 ELSE cost END,
		     price       = CASE WHEN NOT $4::numeric = 0 THEN $4 ELSE price END,
		     description = COALESCE(NULLIF($5,''), description),
		     updated_at  = NOW()
		 WHERE id = $6
		 RETURNING id, name, sku, cost, price, COALESCE(description,''), created_at, updated_at`,
		req.Name, req.SKU, req.Cost, req.Price, req.Description, id,
	).Scan(&it.ID, &it.Name, &it.SKU, &it.Cost, &it.Price, &it.Description, &it.CreatedAt, &it.UpdatedAt)
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
