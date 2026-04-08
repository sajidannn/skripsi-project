package dto

import (
	"math"
	"strings"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	defaultPage  = 1
	defaultLimit = 10
	maxLimit     = 100
)

// ── PageQuery ─────────────────────────────────────────────────────────────────

// PageQuery holds the validated pagination + sorting parameters parsed from
// the request query string. Handlers build this from raw query params and pass
// it down to the service / repository.
//
// Supported query params:
//
//	?page=2&limit=20&sort=name&order=desc
type PageQuery struct {
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

// Validate sanitises the query against a whitelist of allowed sort columns.
// defaultSort is used when Sort is empty or not in the whitelist.
// Returns a copy, never mutates the receiver.
func (q PageQuery) Validate(allowedSorts []string, defaultSort string) PageQuery {
	// Page
	if q.Page < 1 {
		q.Page = defaultPage
	}

	// Limit
	if q.Limit < 1 || q.Limit > maxLimit {
		q.Limit = defaultLimit
	}

	// Sort – whitelist check (case-insensitive)
	allowed := make(map[string]struct{}, len(allowedSorts))
	for _, s := range allowedSorts {
		allowed[strings.ToLower(s)] = struct{}{}
	}
	sortKey := strings.ToLower(q.Sort)
	if _, ok := allowed[sortKey]; !ok {
		q.Sort = defaultSort
	} else {
		q.Sort = sortKey // normalise to lowercase for SQL
	}

	// Order
	if strings.ToLower(q.Order) == "desc" {
		q.Order = "DESC"
	} else {
		q.Order = "ASC"
	}

	return q
}

// Offset returns the SQL OFFSET derived from page and limit.
func (q PageQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

// ── PagedResponse ─────────────────────────────────────────────────────────────

// PageMeta holds the pagination metadata included in every list response.
type PageMeta struct {
	Page      int `json:"page"`
	Limit     int `json:"limit"`
	Total     int `json:"total"`
	TotalPage int `json:"total_page"`
}

// NewPageMeta computes PageMeta from the validated query and the total row count
// returned by the repository.
func NewPageMeta(q PageQuery, total int) PageMeta {
	totalPage := int(math.Ceil(float64(total) / float64(q.Limit)))
	if totalPage < 1 {
		totalPage = 1
	}
	return PageMeta{
		Page:      q.Page,
		Limit:     q.Limit,
		Total:     total,
		TotalPage: totalPage,
	}
}

// PagedResponse is the unified envelope for paginated list endpoints.
type PagedResponse[T any] struct {
	Status string   `json:"status"`
	Data   []T      `json:"data"`
	Meta   PageMeta `json:"meta"`
}

// PagedOK wraps a slice and its meta into a success paged response.
// It guarantees data is never JSON-null (empty slice instead).
func PagedOK[T any](data []T, meta PageMeta) PagedResponse[T] {
	if data == nil {
		data = []T{}
	}
	return PagedResponse[T]{Status: "success", Data: data, Meta: meta}
}
