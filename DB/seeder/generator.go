package main

import (
	"fmt"
	"math/rand"
	"time"
)

// ─── Seed value ───────────────────────────────────────────────────────────────
// Fixed seed ensures fully deterministic, reproducible data across runs and
// environments. Change only when you intentionally want a different dataset.
const randSeed int64 = 42

// ─── Data pools ───────────────────────────────────────────────────────────────

var firstNames = []string{
	"Ahmad", "Budi", "Candra", "Dian", "Eko", "Fajar", "Gilang", "Hendra",
	"Indra", "Joko", "Kurnia", "Lukman", "Muhamad", "Nanda", "Omar",
	"Putra", "Qodir", "Rizal", "Siti", "Taufik", "Ujang", "Vira",
	"Wati", "Xena", "Yusuf", "Zahra", "Andi", "Bagas", "Citra", "Dewi",
	"Erwan", "Fani", "Gita", "Hadi", "Ira", "Juki", "Kartika", "Laras",
	"Mira", "Nina", "Okta", "Putri", "Rini", "Surya", "Tina", "Umar",
	"Vina", "Wawan", "Yasmin", "Zul",
}

var lastNames = []string{
	"Santoso", "Wibowo", "Kusuma", "Pratama", "Rahayu", "Hidayat",
	"Purnomo", "Setiawan", "Utama", "Hartono", "Nugroho", "Saputra",
	"Permadi", "Lestari", "Sanjaya", "Wijaya", "Kurniawan", "Sudrajat",
	"Wahyudi", "Firmansyah", "Gunawan", "Hakim", "Ismail", "Junaidi",
}

var companyTypes = []string{
	"CV", "PT", "UD", "Toko", "Warung", "Distributor",
}

var companyNames = []string{
	"Maju Jaya", "Berkah Abadi", "Sinar Terang", "Karya Mandiri",
	"Usaha Bersama", "Nusantara Raya", "Sejahtera", "Barokah",
	"Makmur Jaya", "Sentosa", "Perkasa", "Mulia Abadi",
	"Bintang Timur", "Harapan Baru", "Surya Abadi",
}

var cityNames = []string{
	"Jakarta", "Surabaya", "Bandung", "Medan", "Semarang",
	"Makassar", "Palembang", "Tangerang", "Depok", "Bekasi",
	"Yogyakarta", "Solo", "Malang", "Bogor", "Denpasar",
}

var streetNames = []string{
	"Jl. Merdeka", "Jl. Sudirman", "Jl. Diponegoro", "Jl. Pahlawan",
	"Jl. Gatot Subroto", "Jl. Ahmad Yani", "Jl. Siliwangi", "Jl. Imam Bonjol",
	"Jl. Hayam Wuruk", "Jl. Mayjend Sutoyo", "Jl. Raya Darmo", "Jl. Bukit Tinggi",
}

var itemCategories = []string{
	"Makanan", "Minuman", "Elektronik", "Pakaian", "Peralatan", "Kosmetik",
	"Obat", "Mainan", "Alat Tulis", "Sembako",
}

// ─── Generator ────────────────────────────────────────────────────────────────

// Generator holds all pre-generated data for a given scale configuration.
// All slices are indexed by a flat sequential ID approach for easy FK lookups.
type Generator struct {
	rng    *rand.Rand
	cfg    ScaleConfig

	// Generated data
	Tenants        []TenantRow
	Warehouses     []WarehouseRow
	Branches       []BranchRow
	Items          []ItemRow
	Suppliers      []SupplierRow
	Users          []UserRow
	Customers      []CustomerRow
	WarehouseItems []WarehouseItemRow
	BranchItems    []BranchItemRow

	// bcrypt hash shared by all users (precomputed once for performance)
	// Password: "password123"
	ownerPassHash string
	// Password: "cashier123"
	cashierPassHash string
}

// ─── Row types ────────────────────────────────────────────────────────────────

type TenantRow struct {
	ID   int
	Name string
}

type WarehouseRow struct {
	ID       int
	TenantID int
	Name     string
}

type BranchRow struct {
	ID             int
	TenantID       int
	Phone          string
	Name           string
	Address        string
	OpeningBalance int64
}

type ItemRow struct {
	ID          int
	TenantID    int
	Name        string
	SKU         string
	Cost        int64 // in IDR, stored as NUMERIC(12,2) → multiply by 100
	Price       int64
	Description string
}

type SupplierRow struct {
	ID       int
	TenantID int
	Name     string
	Phone    string
	Address  string
}

type UserRow struct {
	ID       int
	TenantID int
	Name     string
	Email    string
	Password string // bcrypt hash
	Role     string // 'owner' | 'cashier'
}

type CustomerRow struct {
	ID       int
	TenantID int
	BranchID int
	Name     string
	Phone    string
	Email    string
}

type WarehouseItemRow struct {
	ID          int
	TenantID    int
	WarehouseID int
	ItemID      int
	Stock       int
}

type BranchItemRow struct {
	ID       int
	TenantID int
	BranchID int
	ItemID   int
	Stock    int
}

// ─── Constructor ──────────────────────────────────────────────────────────────

func NewGenerator(cfg ScaleConfig, ownerHash, cashierHash string) *Generator {
	return &Generator{
		rng:             rand.New(rand.NewSource(randSeed)),
		cfg:             cfg,
		ownerPassHash:   ownerHash,
		cashierPassHash: cashierHash,
	}
}

// Generate builds all data in dependency order.
func (g *Generator) Generate() {
	g.genTenants()
	g.genWarehouses()
	g.genBranches()
	g.genItems()
	g.genSuppliers()
	g.genUsers()
	g.genWarehouseItems()
	g.genBranchItems()
	g.genCustomers()
}

// ─── Generators ───────────────────────────────────────────────────────────────

func (g *Generator) genTenants() {
	g.Tenants = make([]TenantRow, g.cfg.Tenants)
	for i := range g.cfg.Tenants {
		g.Tenants[i] = TenantRow{
			ID:   i + 1,
			Name: fmt.Sprintf("Tenant-%03d", i+1),
		}
	}
}

func (g *Generator) genWarehouses() {
	g.Warehouses = make([]WarehouseRow, 0, g.cfg.Tenants*g.cfg.WarehousesPerT)
	id := 1
	for _, t := range g.Tenants {
		for w := range g.cfg.WarehousesPerT {
			g.Warehouses = append(g.Warehouses, WarehouseRow{
				ID:       id,
				TenantID: t.ID,
				Name:     fmt.Sprintf("Gudang-%03d-%02d", t.ID, w+1),
			})
			id++
		}
	}
}

func (g *Generator) genBranches() {
	totalBranches := g.cfg.Tenants * g.cfg.WarehousesPerT * g.cfg.BranchesPerWH
	g.Branches = make([]BranchRow, 0, totalBranches)
	id := 1
	for _, wh := range g.Warehouses {
		for b := range g.cfg.BranchesPerWH {
			city := cityNames[g.rng.Intn(len(cityNames))]
			street := streetNames[g.rng.Intn(len(streetNames))]
			g.Branches = append(g.Branches, BranchRow{
				ID:             id,
				TenantID:       wh.TenantID,
				Phone:          g.randPhone(),
				Name:           fmt.Sprintf("Cabang-%03d-%02d", wh.TenantID, (id-1)%g.cfg.WarehousesPerT*g.cfg.BranchesPerWH+b+1),
				Address:        fmt.Sprintf("%s No.%d, %s", street, g.rng.Intn(99)+1, city),
				OpeningBalance: int64(g.rng.Intn(g.cfg.OpeningBalanceMax/1_000_000)+1) * 1_000_000,
			})
			id++
		}
	}
}

func (g *Generator) genItems() {
	totalItems := g.cfg.Tenants * g.cfg.ItemsPerT
	g.Items = make([]ItemRow, 0, totalItems)
	id := 1
	for _, t := range g.Tenants {
		for i := range g.cfg.ItemsPerT {
			cat := itemCategories[g.rng.Intn(len(itemCategories))]
			cost := int64(g.rng.Intn(95_000)+5_000) // 5.000 – 100.000
			// Price = cost * 1.2 to 1.5 (rounded to nearest 500)
			margin := 120 + g.rng.Intn(31) // 120–150%
			price := cost * int64(margin) / 100
			price = ((price + 499) / 500) * 500 // round up to nearest 500
			g.Items = append(g.Items, ItemRow{
				ID:          id,
				TenantID:    t.ID,
				Name:        fmt.Sprintf("%s Produk %05d", cat, i+1),
				SKU:         fmt.Sprintf("T%03d-SKU-%05d", t.ID, i+1),
				Cost:        cost,
				Price:       price,
				Description: fmt.Sprintf("Deskripsi produk %s nomor %d untuk tenant %d", cat, i+1, t.ID),
			})
			id++
		}
	}
}

func (g *Generator) genSuppliers() {
	totalSuppliers := g.cfg.Tenants * g.cfg.SuppliersPerT
	g.Suppliers = make([]SupplierRow, 0, totalSuppliers)
	id := 1
	for _, t := range g.Tenants {
		for range g.cfg.SuppliersPerT {
			cType := companyTypes[g.rng.Intn(len(companyTypes))]
			cName := companyNames[g.rng.Intn(len(companyNames))]
			city := cityNames[g.rng.Intn(len(cityNames))]
			street := streetNames[g.rng.Intn(len(streetNames))]
			g.Suppliers = append(g.Suppliers, SupplierRow{
				ID:       id,
				TenantID: t.ID,
				Name:     fmt.Sprintf("%s %s %d", cType, cName, id),
				Phone:    g.randPhone(),
				Address:  fmt.Sprintf("%s No.%d, %s", street, g.rng.Intn(99)+1, city),
			})
			id++
		}
	}
}

func (g *Generator) genUsers() {
	// 1 owner per tenant + 1 cashier per branch
	totalBranchesPerT := g.cfg.WarehousesPerT * g.cfg.BranchesPerWH
	g.Users = make([]UserRow, 0, g.cfg.Tenants*(1+totalBranchesPerT))
	id := 1

	// Build branch lookup by tenant (index into g.Branches)
	branchByTenant := make(map[int][]BranchRow)
	for _, br := range g.Branches {
		branchByTenant[br.TenantID] = append(branchByTenant[br.TenantID], br)
	}

	for _, t := range g.Tenants {
		// Owner
		g.Users = append(g.Users, UserRow{
			ID:       id,
			TenantID: t.ID,
			Name:     fmt.Sprintf("Admin Tenant-%03d", t.ID),
			Email:    fmt.Sprintf("admin@tenant-%03d.test", t.ID),
			Password: g.ownerPassHash,
			Role:     "owner",
		})
		id++

		// One cashier per branch
		for brIdx, br := range branchByTenant[t.ID] {
			_ = br // branch referenced for ordering; not included in UserRow
			first := firstNames[g.rng.Intn(len(firstNames))]
			last := lastNames[g.rng.Intn(len(lastNames))]
			g.Users = append(g.Users, UserRow{
				ID:       id,
				TenantID: t.ID,
				Name:     fmt.Sprintf("%s %s", first, last),
				Email:    fmt.Sprintf("kasir.%03d.%03d@tenant-%03d.test", t.ID, brIdx+1, t.ID),
				Password: g.cashierPassHash,
				Role:     "cashier",
			})
			id++
		}
	}
}

func (g *Generator) genWarehouseItems() {
	// Every warehouse gets ALL items from that tenant
	totalWHItems := g.cfg.Tenants * g.cfg.WarehousesPerT * g.cfg.ItemsPerT
	g.WarehouseItems = make([]WarehouseItemRow, 0, totalWHItems)
	id := 1

	// Build item lookup by tenant
	itemByTenant := make(map[int][]ItemRow)
	for _, it := range g.Items {
		itemByTenant[it.TenantID] = append(itemByTenant[it.TenantID], it)
	}

	for _, wh := range g.Warehouses {
		for _, item := range itemByTenant[wh.TenantID] {
			stock := g.rng.Intn(g.cfg.StockWH) + 50 // 50 to StockWH+50
			g.WarehouseItems = append(g.WarehouseItems, WarehouseItemRow{
				ID:          id,
				TenantID:    wh.TenantID,
				WarehouseID: wh.ID,
				ItemID:      item.ID,
				Stock:       stock,
			})
			id++
		}
	}
}

func (g *Generator) genBranchItems() {
	// Each branch gets 30% of items from that tenant (realistic: branch ≠ full catalogue)
	branchItemsPerBR := g.cfg.ItemsPerT * 30 / 100
	if branchItemsPerBR < 10 {
		branchItemsPerBR = 10
	}

	totalBranchItems := len(g.Branches) * branchItemsPerBR
	g.BranchItems = make([]BranchItemRow, 0, totalBranchItems)
	id := 1

	// Build item index by tenant for random sampling
	itemByTenant := make(map[int][]ItemRow)
	for _, it := range g.Items {
		itemByTenant[it.TenantID] = append(itemByTenant[it.TenantID], it)
	}

	for _, br := range g.Branches {
		items := itemByTenant[br.TenantID]
		// Sample branchItemsPerBR items deterministically using Fisher-Yates partial shuffle
		sampled := g.sampleItems(items, branchItemsPerBR)
		for _, item := range sampled {
			stock := g.rng.Intn(g.cfg.StockBR) + 10 // 10 to StockBR+10
			g.BranchItems = append(g.BranchItems, BranchItemRow{
				ID:       id,
				TenantID: br.TenantID,
				BranchID: br.ID,
				ItemID:   item.ID,
				Stock:    stock,
			})
			id++
		}
	}
}

func (g *Generator) genCustomers() {
	totalCustomers := len(g.Branches) * g.cfg.CustomersPerBR
	g.Customers = make([]CustomerRow, 0, totalCustomers)
	id := 1

	for _, br := range g.Branches {
		for c := range g.cfg.CustomersPerBR {
			first := firstNames[g.rng.Intn(len(firstNames))]
			last := lastNames[g.rng.Intn(len(lastNames))]
			g.Customers = append(g.Customers, CustomerRow{
				ID:       id,
				TenantID: br.TenantID,
				BranchID: br.ID,
				Name:     fmt.Sprintf("%s %s", first, last),
				Phone:    g.randPhone(),
				Email:    fmt.Sprintf("cust.%d.%d@mail.test", br.ID, c+1),
			})
			id++
		}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (g *Generator) randPhone() string {
	prefixes := []string{"0811", "0812", "0813", "0821", "0822", "0823", "0851", "0852", "0858"}
	prefix := prefixes[g.rng.Intn(len(prefixes))]
	return fmt.Sprintf("%s%07d", prefix, g.rng.Intn(10_000_000))
}

// sampleItems returns n items sampled from the given slice without replacement.
// Uses a copy to avoid mutating the original.
func (g *Generator) sampleItems(items []ItemRow, n int) []ItemRow {
	if n >= len(items) {
		return items
	}
	cp := make([]ItemRow, len(items))
	copy(cp, items)
	for i := range n {
		j := i + g.rng.Intn(len(cp)-i)
		cp[i], cp[j] = cp[j], cp[i]
	}
	return cp[:n]
}

// Now returns current UTC time (used as created_at for all rows).
func now() time.Time {
	return time.Now().UTC()
}
