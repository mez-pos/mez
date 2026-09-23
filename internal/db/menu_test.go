package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/menu"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

var menuNow = time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)

// smallMenu is two stations, two courses, a milk choice on coffee, and an
// exempt water: enough to see every join the menu read makes.
func smallMenu() MenuSpec {
	return MenuSpec{
		Stations:   []StationSpec{{Code: "bar", Name: "Bar"}, {Code: "kitchen", Name: "Kitchen"}},
		Courses:    []CourseSpec{{Number: 1, Name: "Mains"}, {Number: 2, Name: "Dessert"}},
		TaxClasses: []TaxClassSpec{{Name: "Food 5%", Rate: tax.RateFromBP(500)}, {Name: "Exempt", Exempt: true}},
		Groups: []ModifierGroupSpec{
			{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []ModifierSpec{
				{Name: "Dairy", IsDefault: true}, {Name: "Oat", Price: 4000},
			}},
			{Name: "Extras", MaxSelect: 2, Modifiers: []ModifierSpec{{Name: "Extra shot", Price: 3000}}},
		},
		Categories: []CategorySpec{
			{Name: "Coffee", Items: []ItemSpec{
				{Name: "Flat white", Price: 18000, HSN: "996331", TaxClass: "Food 5%", Station: "bar", Course: 1, Groups: []string{"Milk", "Extras"}},
				{Name: "Water", Price: 3000, TaxClass: "Exempt", Station: "bar", Course: 1},
			}},
			{Name: "Cake", Items: []ItemSpec{
				{Name: "Brownie", Price: 15000, HSN: "996331", TaxClass: "Food 5%", Station: "kitchen", Course: 2},
			}},
		},
	}
}

func menuDB(t *testing.T) *DB {
	t.Helper()
	d := migratedDB(t)
	insertVenue(t, d, 1)
	return d
}

func TestAVenueWithNoMenuIsAtVersionOne(t *testing.T) {
	d := menuDB(t)
	m, err := d.Menu(context.Background(), 1)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if m.Version != 1 || m.Currency != "INR" {
		t.Errorf("version %d currency %q, want 1 INR", m.Version, m.Currency)
	}
	if m.Items == nil || m.Categories == nil || m.Groups == nil || m.Stations == nil || m.Courses == nil {
		t.Errorf("an empty menu has nil lists, which encode as null: %+v", m)
	}
}

func TestCreateMenuReadsBackResolved(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()

	v, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow)
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if v != 2 {
		t.Errorf("version after the first write = %d, want 2", v)
	}
	m, err := d.Menu(ctx, 1)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if m.Version != v {
		t.Errorf("read version %d, wrote %d", m.Version, v)
	}
	if got := len(m.Categories); got != 2 || m.Categories[0].Name != "Coffee" {
		t.Fatalf("categories = %+v", m.Categories)
	}
	if len(m.Items) != 3 {
		t.Fatalf("items = %+v", m.Items)
	}

	flat := m.Items[0]
	if flat.Name != "Flat white" || flat.Price.Minor != 18000 || flat.Price.Currency != "INR" {
		t.Errorf("flat white = %+v", flat)
	}
	if flat.Station != "bar" || flat.Course != 1 || flat.TaxRate != tax.RateFromBP(500) || flat.TaxExempt || flat.HSN != "996331" {
		t.Errorf("flat white snapshot facts = %+v", flat)
	}
	if len(flat.Groups) != 2 {
		t.Fatalf("flat white groups = %v", flat.Groups)
	}
	milk := groupByID(t, m, flat.Groups[0])
	if milk.Name != "Milk" || milk.MinSelect != 1 || milk.MaxSelect != 1 || len(milk.Modifiers) != 2 {
		t.Errorf("milk = %+v", milk)
	}
	if !milk.Modifiers[0].IsDefault || milk.Modifiers[1].Price.Minor != 4000 {
		t.Errorf("milk modifiers = %+v", milk.Modifiers)
	}

	water := m.Items[1]
	if !water.TaxExempt || len(water.Groups) != 0 || water.Groups == nil {
		t.Errorf("water = %+v", water)
	}
	if brownie := m.Items[2]; brownie.Station != "kitchen" || brownie.Course != 2 {
		t.Errorf("brownie = %+v", brownie)
	}

	v2, err := d.CreateMenu(ctx, 1, MenuSpec{Stations: []StationSpec{{Code: "pass", Name: "Pass"}}}, menuNow)
	if err != nil {
		t.Fatalf("second CreateMenu: %v", err)
	}
	if v2 != 3 {
		t.Errorf("version after the second write = %d, want 3", v2)
	}
}

func TestRetiredRowsLeaveTheMenu(t *testing.T) {
	tests := []struct {
		name    string
		retire  string
		items   int
		coffee  int // groups on the flat white
		station int
	}{
		{"nothing", ``, 3, 2, 2},
		{"an item", `UPDATE items SET active = 0 WHERE name = 'Water'`, 2, 2, 2},
		{"a category takes its items", `UPDATE categories SET active = 0 WHERE name = 'Cake'`, 2, 2, 2},
		{"a station takes its items", `UPDATE stations SET active = 0 WHERE code = 'kitchen'`, 2, 2, 1},
		{"a group leaves the items", `UPDATE modifier_groups SET active = 0 WHERE name = 'Extras'`, 3, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := menuDB(t)
			ctx := context.Background()
			if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
				t.Fatalf("CreateMenu: %v", err)
			}
			if tt.retire != "" {
				if _, err := d.Writer().ExecContext(ctx, tt.retire); err != nil {
					t.Fatalf("retiring: %v", err)
				}
			}
			m, err := d.Menu(ctx, 1)
			if err != nil {
				t.Fatalf("Menu: %v", err)
			}
			if len(m.Items) != tt.items || len(m.Stations) != tt.station {
				t.Errorf("items %d stations %d, want %d %d", len(m.Items), len(m.Stations), tt.items, tt.station)
			}
			if got := len(m.Items[0].Groups); got != tt.coffee {
				t.Errorf("flat white has %d groups, want %d", got, tt.coffee)
			}
		})
	}
}

func TestCreateMenuRefusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*MenuSpec)
		spec   bool // refused as ErrMenuSpec, before the database is asked
	}{
		{"unknown station", func(s *MenuSpec) { s.Categories[0].Items[0].Station = "grill" }, true},
		{"unknown course", func(s *MenuSpec) { s.Categories[0].Items[0].Course = 9 }, true},
		{"unknown tax class", func(s *MenuSpec) { s.Categories[0].Items[0].TaxClass = "Luxury" }, true},
		{"unknown group", func(s *MenuSpec) { s.Categories[0].Items[0].Groups = []string{"Syrups"} }, true},
		{"negative price", func(s *MenuSpec) { s.Categories[0].Items[0].Price = -1 }, false},
		{"hsn with letters", func(s *MenuSpec) { s.Categories[0].Items[0].HSN = "99A331" }, false},
		{"station code in capitals", func(s *MenuSpec) { s.Stations[0].Code = "Bar" }, false},
		{"max below min", func(s *MenuSpec) { s.Groups[0].MaxSelect = 0 }, false},
		{"rate over the whole", func(s *MenuSpec) { s.TaxClasses[0].Rate = tax.RateScale + 1 }, false},
		{"duplicate station", func(s *MenuSpec) { s.Stations[1].Code = "bar" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := menuDB(t)
			ctx := context.Background()
			spec := smallMenu()
			tt.mutate(&spec)
			_, err := d.CreateMenu(ctx, 1, spec, menuNow)
			if err == nil {
				t.Fatal("CreateMenu accepted it")
			}
			if got := errors.Is(err, ErrMenuSpec); got != tt.spec {
				t.Errorf("errors.Is(ErrMenuSpec) = %v, want %v: %v", got, tt.spec, err)
			}
			m, err := d.Menu(ctx, 1)
			if err != nil {
				t.Fatalf("Menu: %v", err)
			}
			if m.Version != 1 || len(m.Stations) != 0 {
				t.Errorf("a refused menu left version %d and %d stations behind", m.Version, len(m.Stations))
			}
		})
	}
}

func TestMenuOfAnotherVenue(t *testing.T) {
	d := menuDB(t)
	insertVenue(t, d, 2)
	ctx := context.Background()
	if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	m, err := d.Menu(ctx, 2)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(m.Items) != 0 || m.Version != 1 {
		t.Errorf("venue 2 sees venue 1's menu: %+v", m)
	}
	if _, err := d.Menu(ctx, 99); !errors.Is(err, ErrVenueNotFound) {
		t.Errorf("Menu of no venue = %v, want ErrVenueNotFound", err)
	}
}

// TestAnItemCannotUseAnotherVenuesRows is the composite foreign keys at
// work: the ids exist, but in the wrong venue.
func TestAnItemCannotUseAnotherVenuesRows(t *testing.T) {
	d := menuDB(t)
	insertVenue(t, d, 2)
	ctx := context.Background()
	if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	_, err := d.Writer().ExecContext(ctx, `
		INSERT INTO items (venue_id, category_id, name, price, tax_class_id, station_id, course_id, created_at, updated_at)
		SELECT 2, category_id, 'Stolen', price, tax_class_id, station_id, course_id, created_at, updated_at
		FROM items WHERE venue_id = 1 LIMIT 1`)
	if err == nil {
		t.Fatal("an item in venue 2 was filed under venue 1's category, station and course")
	}
}

func TestMenuVersion(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()
	if v, err := d.MenuVersion(ctx, 1); err != nil || v != 1 {
		t.Fatalf("MenuVersion = %d, %v; want 1", v, err)
	}
	if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if v, err := d.MenuVersion(ctx, 1); err != nil || v != 2 {
		t.Fatalf("MenuVersion = %d, %v; want 2", v, err)
	}
}

func groupByID(t *testing.T, m menu.Menu, id int64) menu.ModifierGroup {
	t.Helper()
	for _, g := range m.Groups {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no group %d in %+v", id, m.Groups)
	return menu.ModifierGroup{}
}
