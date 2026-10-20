package floor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var now = time.Date(2026, 9, 27, 4, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// openDB is a fresh database: venues 1 and 2, untaxed, and an area of
// venue 2's (id 1) that venue 1 must not reach.
func openDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for _, v := range []string{"ven_FLOOR001", "ven_FLOOR002"} {
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: v, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		if _, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: 2, Name: "Elsewhere", CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		// An open shift at venue 1: an order is refused without one (ADR-094).
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_FLOOR001", VenueID: 1, TradingDay: "2026-09-27", OpenedBy: 1, OpenedAt: now})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// newService is the floor plan over openDB, with no orders.
func newService(t *testing.T) *Service {
	t.Helper()
	s, err := New(Options{DB: openDB(t), Clock: clock{}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// must returns the value, or panics on an error, which fails the test: a
// step the test stands on, not the thing it checks.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("New with no database worked")
	}
}

func TestTableSettingsAreChecked(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	inside := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	good := TableSettings{AreaID: inside, Name: " T1 ", Seats: 4, Shape: Round, X: 0, Y: 0}
	tests := []struct {
		name  string
		edit  func(*TableSettings)
		field string
	}{
		{"no name", func(t *TableSettings) { t.Name = "  " }, "name"},
		{"a long name", func(t *TableSettings) { t.Name = "a table with a very long name" }, "name"},
		{"no seats", func(t *TableSettings) { t.Seats = 0 }, "seats"},
		{"too many seats", func(t *TableSettings) { t.Seats = maxSeats + 1 }, "seats"},
		{"no shape", func(t *TableSettings) { t.Shape = "" }, "shape"},
		{"a shape we do not draw", func(t *TableSettings) { t.Shape = "hexagon" }, "shape"},
		{"left of the grid", func(t *TableSettings) { t.X = -1 }, "x"},
		{"right of the grid", func(t *TableSettings) { t.X = Columns }, "x"},
		{"above the grid", func(t *TableSettings) { t.Y = -1 }, "y"},
		{"below the grid", func(t *TableSettings) { t.Y = Rows }, "y"},
		{"no area", func(t *TableSettings) { t.AreaID = 0 }, "area_id"},
		{"another venue's area", func(t *TableSettings) { t.AreaID = 1 }, "area_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := good
			tt.edit(&in)
			_, err := s.CreateTable(ctx, 1, in)
			var invalid *Invalid
			if !errors.As(err, &invalid) || invalid.Field != tt.field {
				t.Errorf("err = %v, want a refusal on %s", err, tt.field)
			}
		})
	}

	id := must(s.CreateTable(ctx, 1, good))
	plan := must(s.Plan(ctx, 1))
	if len(plan.Tables) != 1 || plan.Tables[0].ID != id || plan.Tables[0].Name != "T1" || plan.Tables[0].Shape != "round" {
		t.Errorf("plan = %+v", plan.Tables)
	}
	corner := good
	corner.Name, corner.X, corner.Y = "T2", Columns-1, Rows-1
	must(s.CreateTable(ctx, 1, corner))
}

func TestAreaSettingsAreChecked(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	for _, name := range []string{"", "   ", "an area whose name runs on and on"} {
		var invalid *Invalid
		if _, err := s.CreateArea(ctx, 1, AreaSettings{Name: name}); !errors.As(err, &invalid) || invalid.Field != "name" {
			t.Errorf("%q: err = %v", name, err)
		}
	}
	id := must(s.CreateArea(ctx, 1, AreaSettings{Name: " Inside "}))
	if _, err := s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("a second Inside: %v", err)
	}
	// Another venue may use the same name.
	must(s.CreateArea(ctx, 2, AreaSettings{Name: "Inside"}))

	terrace := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Terrace"}))
	if err := s.RenameArea(ctx, 1, terrace, AreaSettings{Name: "Inside"}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("renamed onto Inside: %v", err)
	}
	if err := s.RenameArea(ctx, 1, terrace, AreaSettings{Name: "Garden"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameArea(ctx, 1, 1, AreaSettings{Name: "Mine"}); !errors.Is(err, ErrAreaNotFound) {
		t.Errorf("renamed another venue's area: %v", err)
	}
	if err := s.RenameArea(ctx, 1, id, AreaSettings{}); err == nil {
		t.Error("renamed to nothing")
	}
	plan := must(s.Plan(ctx, 1))
	if len(plan.Areas) != 2 || plan.Areas[0].Name != "Inside" || plan.Areas[1].Name != "Garden" {
		t.Errorf("areas = %+v", plan.Areas)
	}
}

// TestOneTablePerCell: a cell holds one table in service; a table does not
// collide with itself; a retired table gives its cell up and needs it back.
func TestOneTablePerCell(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	inside := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	terrace := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Terrace"}))
	t1 := must(s.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T1", Seats: 2, Shape: Square, X: 3, Y: 2}))

	clash := TableSettings{AreaID: inside, Name: "T2", Seats: 2, Shape: Square, X: 3, Y: 2}
	_, err := s.CreateTable(ctx, 1, clash)
	var invalid *Invalid
	if !errors.Is(err, ErrCellTaken) || !errors.As(err, &invalid) || invalid.Message != "Table T1 is already at column 4, row 3." {
		t.Fatalf("placed on T1: %v", err)
	}
	// The same cell in another area is another cell.
	clash.AreaID = terrace
	t2 := must(s.CreateTable(ctx, 1, clash))

	// T1 edited in place does not collide with itself; moved onto T2 it does.
	if err := s.UpdateTable(ctx, 1, t1, TableSettings{AreaID: inside, Name: "T1", Seats: 6, Shape: Long, X: 3, Y: 2}); err != nil {
		t.Fatalf("edited in place: %v", err)
	}
	if err := s.UpdateTable(ctx, 1, t1, TableSettings{AreaID: terrace, Name: "T1", Seats: 6, Shape: Long, X: 3, Y: 2}); !errors.Is(err, ErrCellTaken) {
		t.Errorf("moved onto T2: %v", err)
	}

	// Retired, T2 gives up its cell: T3 takes it, and T2 cannot come back
	// until it moves.
	if err := s.SetTableActive(ctx, 1, t2, false); err != nil {
		t.Fatal(err)
	}
	t3 := must(s.CreateTable(ctx, 1, TableSettings{AreaID: terrace, Name: "T3", Seats: 2, Shape: Round, X: 3, Y: 2}))
	if err := s.SetTableActive(ctx, 1, t2, true); !errors.Is(err, ErrCellTaken) {
		t.Errorf("brought back onto T3: %v", err)
	}
	// A retired table moves anywhere, even onto T3: it holds no cell.
	if err := s.UpdateTable(ctx, 1, t2, TableSettings{AreaID: terrace, Name: "T2", Seats: 2, Shape: Square, X: 3, Y: 2}); err != nil {
		t.Errorf("moved while retired: %v", err)
	}
	if err := s.UpdateTable(ctx, 1, t2, TableSettings{AreaID: terrace, Name: "T2", Seats: 2, Shape: Square, X: 0, Y: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTableActive(ctx, 1, t2, true); err != nil {
		t.Errorf("brought back to a free cell: %v", err)
	}
	// Retiring twice, or bringing back one in service, changes nothing.
	for _, active := range []bool{true, false, false} {
		if err := s.SetTableActive(ctx, 1, t3, active); err != nil {
			t.Errorf("SetTableActive(%v): %v", active, err)
		}
	}
}

func TestTableNamesAreUniqueAcrossTheVenue(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	inside := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	terrace := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Terrace"}))
	must(s.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "4", Seats: 2, Shape: Square}))
	_, err := s.CreateTable(ctx, 1, TableSettings{AreaID: terrace, Name: "4", Seats: 2, Shape: Square})
	var invalid *Invalid
	if !errors.Is(err, ErrDuplicate) || !errors.As(err, &invalid) || invalid.Field != "name" {
		t.Errorf("a second table 4: %v", err)
	}
	five := must(s.CreateTable(ctx, 1, TableSettings{AreaID: terrace, Name: "5", Seats: 2, Shape: Square}))
	if err := s.UpdateTable(ctx, 1, five, TableSettings{AreaID: terrace, Name: "4", Seats: 2, Shape: Square}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("renamed onto 4: %v", err)
	}
}

// TestRetiredAreas: an area retires only once its tables have, takes no new
// tables while retired, and holds its retired tables back.
func TestRetiredAreas(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	inside := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	t1 := must(s.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T1", Seats: 2, Shape: Square}))

	if err := s.SetAreaActive(ctx, 1, inside, false); !errors.Is(err, ErrAreaInUse) {
		t.Fatalf("retired with T1 in it: %v", err)
	}
	if err := s.SetTableActive(ctx, 1, t1, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAreaActive(ctx, 1, inside, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T2", Seats: 2, Shape: Square, X: 1}); !errors.Is(err, ErrAreaRetired) {
		t.Errorf("placed in a retired area: %v", err)
	}
	if err := s.SetTableActive(ctx, 1, t1, true); !errors.Is(err, ErrAreaRetired) {
		t.Errorf("brought back into a retired area: %v", err)
	}
	if err := s.SetAreaActive(ctx, 1, inside, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTableActive(ctx, 1, t1, true); err != nil {
		t.Errorf("brought back once the area was: %v", err)
	}
	if err := s.SetAreaActive(ctx, 1, 1, false); !errors.Is(err, ErrAreaNotFound) {
		t.Errorf("retired another venue's area: %v", err)
	}
}

func TestAnotherVenuesTablesAreOutOfReach(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	theirs := must(s.CreateTable(ctx, 2, TableSettings{AreaID: 1, Name: "T1", Seats: 2, Shape: Square}))
	mine := must(s.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	if err := s.UpdateTable(ctx, 1, theirs, TableSettings{AreaID: mine, Name: "T1", Seats: 2, Shape: Square}); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("updated: %v", err)
	}
	if err := s.SetTableActive(ctx, 1, theirs, false); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("retired: %v", err)
	}
	// A retired table of mine cannot be moved into their area either.
	t1 := must(s.CreateTable(ctx, 1, TableSettings{AreaID: mine, Name: "T1", Seats: 2, Shape: Square}))
	if err := s.SetTableActive(ctx, 1, t1, false); err != nil {
		t.Fatal(err)
	}
	var invalid *Invalid
	if err := s.UpdateTable(ctx, 1, t1, TableSettings{AreaID: 1, Name: "T1", Seats: 2, Shape: Square}); !errors.Is(err, ErrAreaNotFound) || !errors.As(err, &invalid) {
		t.Errorf("moved into their area: %v", err)
	}
	if plan := must(s.Plan(ctx, 1)); len(plan.Tables) != 1 {
		t.Errorf("venue 1 sees %d tables", len(plan.Tables))
	}
}
