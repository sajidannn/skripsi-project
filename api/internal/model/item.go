package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Item is the tenant's master product catalogue entry.
type Item struct {
	ID          int             `json:"id"`
	TenantID    int             `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	Name        string          `json:"name"`
	SKU         string          `json:"sku"`
	Cost        decimal.Decimal `json:"cost"`
	Price       decimal.Decimal `json:"price"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
