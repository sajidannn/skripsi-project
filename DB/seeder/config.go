package main

// ScaleConfig defines the number of entities to generate per data scale.
type ScaleConfig struct {
	Name             string
	Tenants          int // Total tenants
	WarehousesPerT   int // Warehouses per tenant
	BranchesPerWH    int // Branches per warehouse
	ItemsPerT        int // Items per tenant
	CustomersPerBR   int // Customers per branch
	SuppliersPerT    int // Suppliers per tenant
	StockWH          int // Max initial stock per warehouse_item
	StockBR          int // Max initial stock per branch_item
	OpeningBalanceMax int // Max opening balance per branch (in IDR)
}

// scales maps the scale name to its configuration.
// Based on Proposal Tabel 3.6 — Parameter Skala Data.
var scales = map[string]ScaleConfig{
	"small": {
		Name:             "Small",
		Tenants:          5,
		WarehousesPerT:   5,
		BranchesPerWH:    1,
		ItemsPerT:        1000,
		CustomersPerBR:   100,
		SuppliersPerT:    10,
		StockWH:          500,
		StockBR:          100,
		OpeningBalanceMax: 10_000_000,
	},
	"medium": {
		Name:             "Medium",
		Tenants:          10,
		WarehousesPerT:   7,
		BranchesPerWH:    3,
		ItemsPerT:        3000,
		CustomersPerBR:   200,
		SuppliersPerT:    30,
		StockWH:          500,
		StockBR:          100,
		OpeningBalanceMax: 50_000_000,
	},
	"large": {
		Name:             "Large",
		Tenants:          50,
		WarehousesPerT:   10,
		BranchesPerWH:    5,
		ItemsPerT:        5000,
		CustomersPerBR:   300,
		SuppliersPerT:    50,
		StockWH:          1000,
		StockBR:          200,
		OpeningBalanceMax: 100_000_000,
	},
}

// derivedCounts returns human-readable summary of what will be seeded.
func (s ScaleConfig) derivedCounts() map[string]int {
	totalBranchesPerT := s.WarehousesPerT * s.BranchesPerWH
	totalWHItems := s.WarehousesPerT * s.ItemsPerT
	// Each branch gets 30% of items as branch_items (realistic: not all items at every branch)
	branchItemsPerBR := s.ItemsPerT * 30 / 100
	if branchItemsPerBR < 10 {
		branchItemsPerBR = 10
	}
	usersPerT := 1 + totalBranchesPerT // 1 owner + 1 cashier per branch

	return map[string]int{
		"tenants":          s.Tenants,
		"warehouses":       s.Tenants * s.WarehousesPerT,
		"branches":         s.Tenants * totalBranchesPerT,
		"items":            s.Tenants * s.ItemsPerT,
		"suppliers":        s.Tenants * s.SuppliersPerT,
		"users":            s.Tenants * usersPerT,
		"warehouse_items":  s.Tenants * totalWHItems,
		"branch_items":     s.Tenants * totalBranchesPerT * branchItemsPerBR,
		"customers":        s.Tenants * totalBranchesPerT * s.CustomersPerBR,
	}
}
