package stock

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/stock"
)

var now = time.Date(2026, 9, 30, 6, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// fixture is venues 1 and 2, and Asha (staff id 1) at venue 1.
type fixture struct {
	s    *Service
	d    *db.DB
	asha int64
}

func newFixture(t *testing.T) fixture {
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
	var asha int64
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for _, v := range []string{"ven_STOCK001", "ven_STOCK002"} {
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: v, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		role, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: "manager", Permissions: `["stock.manage"]`, CreatedAt: now})
		if err != nil {
			return err
		}
		staff, err := q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: "stf_ASHA0001", VenueID: 1, RoleID: role.ID, Name: "Asha", PinHash: "x", Active: true, CreatedAt: now, UpdatedAt: now,
		})
		asha = staff.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(d, clock{})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{s: s, d: d, asha: asha}
}

func (f fixture) item(t *testing.T, venue int64, name string, unit stock.Unit, low stock.Qty) int64 {
	t.Helper()
	id, err := f.s.CreateItem(context.Background(), venue, ItemInput{Name: name, Unit: unit, LowLevel: low})
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	return id
}

func (f fixture) record(t *testing.T, id int64, in MovementInput) Recorded {
	t.Helper()
	r, err := f.s.Record(context.Background(), 1, f.asha, id, in)
	if err != nil {
		t.Fatalf("recording %+v: %v", in, err)
	}
	return r
}

func (f fixture) find(t *testing.T, venue, id int64) Item {
	t.Helper()
	items, err := f.s.Items(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("no item %d", id)
	return Item{}
}

func fieldOf(err error) string {
	var invalid *Invalid
	if errors.As(err, &invalid) {
		return invalid.Field
	}
	return ""
}

const kg = 1000 * stock.PerUnit

// A day of coffee beans, through the ledger: on hand is always its sum.
func TestTheLedger(t *testing.T) {
	f := newFixture(t)
	beans := f.item(t, 1, " Coffee beans ", stock.Gram, 2*kg)

	if it := f.find(t, 1, beans); it.Name != "Coffee beans" || it.OnHand != 0 || !it.Low || !it.Active {
		t.Fatalf("new = %+v; nothing on hand is at or under 2 kg, so low", it)
	}

	steps := []struct {
		name   string
		in     MovementInput
		delta  stock.Qty
		onHand stock.Qty
		low    bool
	}{
		{"a delivery", MovementInput{Kind: stock.Received, Qty: 5 * kg}, 5 * kg, 5 * kg, false},
		{"a spill", MovementInput{Kind: stock.Wasted, Qty: 250 * stock.PerUnit, Reason: stock.Spilled}, -250 * stock.PerUnit, 4750 * stock.PerUnit, false},
		{"a count that finds less", MovementInput{Kind: stock.Counted, Qty: 4700 * stock.PerUnit}, -50 * stock.PerUnit, 4700 * stock.PerUnit, false},
		{"a correction", MovementInput{Kind: stock.Adjusted, Qty: -2800 * stock.PerUnit, Note: "Bag in the wrong store"}, -2800 * stock.PerUnit, 1900 * stock.PerUnit, true},
		{"a count that agrees", MovementInput{Kind: stock.Counted, Qty: 1900 * stock.PerUnit}, 0, 1900 * stock.PerUnit, true},
	}
	for _, st := range steps {
		r := f.record(t, beans, st.in)
		if r.Delta != st.delta || r.OnHand != st.onHand {
			t.Errorf("%s: delta %d on hand %d, want %d and %d", st.name, r.Delta, r.OnHand, st.delta, st.onHand)
		}
		if it := f.find(t, 1, beans); it.OnHand != st.onHand || it.Low != st.low {
			t.Errorf("%s: listed on hand %d low %v, want %d %v", st.name, it.OnHand, it.Low, st.onHand, st.low)
		}
	}

	// The history, newest first, with who recorded it.
	all, next, err := f.s.Movements(context.Background(), 1, beans, 0, 0)
	if err != nil || next != 0 || len(all) != 5 {
		t.Fatalf("movements = %d, next %d, %v", len(all), next, err)
	}
	if all[0].Kind != stock.Counted || all[0].Counted == nil || *all[0].Counted != 1900*stock.PerUnit || all[0].StaffName != "Asha" {
		t.Errorf("newest = %+v", all[0])
	}
	if all[3].Reason != stock.Spilled || all[4].Kind != stock.Received || all[4].Counted != nil {
		t.Errorf("oldest two = %+v, %+v", all[3], all[4])
	}
}

func TestMovementsArePaged(t *testing.T) {
	f := newFixture(t)
	milk := f.item(t, 1, "Milk", stock.Millilitre, 0)
	for i := 0; i < 5; i++ {
		f.record(t, milk, MovementInput{Kind: stock.Received, Qty: stock.Qty(i+1) * stock.PerUnit})
	}
	var seen []stock.Qty
	before := int64(0)
	for pages := 0; ; pages++ {
		page, next, err := f.s.Movements(context.Background(), 1, milk, before, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page {
			seen = append(seen, m.Delta/stock.PerUnit)
		}
		if next == 0 {
			if pages != 2 {
				t.Errorf("%d pages, want 3", pages+1)
			}
			break
		}
		before = next
	}
	want := []stock.Qty{5, 4, 3, 2, 1}
	if len(seen) != len(want) {
		t.Fatalf("seen %v", seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("seen %v, want %v", seen, want)
			break
		}
	}
}

func TestAMovementBelongsToTheOpenShift(t *testing.T) {
	f := newFixture(t)
	ice := f.item(t, 1, "Ice", stock.Each, 0)
	f.record(t, ice, MovementInput{Kind: stock.Received, Qty: 10 * stock.PerUnit})
	if _, err := f.d.Writer().ExecContext(context.Background(),
		`INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at)
		 VALUES ('shf_STOCK001', 1, 'open', '2026-09-30', 1, '2026-09-30 06:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	f.record(t, ice, MovementInput{Kind: stock.Wasted, Qty: stock.PerUnit, Reason: stock.Dropped})
	page, _, err := f.s.Movements(context.Background(), 1, ice, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page[0].ShiftID == 0 || page[1].ShiftID != 0 {
		t.Errorf("shifts = %d then %d; want the open shift, then none", page[1].ShiftID, page[0].ShiftID)
	}
}

func TestMovementsAreChecked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	other := f.item(t, 2, "Beans", stock.Gram, 0)

	if _, err := f.s.Record(ctx, 1, f.asha, beans, MovementInput{Kind: stock.Wasted, Qty: 1}); fieldOf(err) != "reason" {
		t.Errorf("waste with no reason: %v", err)
	}
	if _, err := f.s.Record(ctx, 1, f.asha, beans, MovementInput{Kind: stock.Sold, Qty: 1}); fieldOf(err) != "kind" {
		t.Errorf("a sale by hand: %v", err)
	}
	if _, err := f.s.Record(ctx, 1, f.asha, other, MovementInput{Kind: stock.Received, Qty: 1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another venue's item: %v", err)
	}
	if err := f.s.SetItemActive(ctx, 1, beans, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Record(ctx, 1, f.asha, beans, MovementInput{Kind: stock.Received, Qty: 1}); !errors.Is(err, ErrRetired) {
		t.Errorf("a retired item: %v", err)
	}
	if it := f.find(t, 1, beans); it.Active || it.Low {
		t.Errorf("retired = %+v; a retired item is never low", it)
	}
	if page, _, _ := f.s.Movements(ctx, 1, beans, 0, 0); len(page) != 0 {
		t.Error("refused movements must write nothing")
	}
	if _, _, err := f.s.Movements(ctx, 1, other, 0, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("another venue's history: %v", err)
	}
}

func TestItemsAreChecked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	beans := f.item(t, 1, "Coffee beans", stock.Gram, 0)
	milk := f.item(t, 1, "Milk", stock.Millilitre, 0)

	if _, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "coffee BEANS", Unit: stock.Gram}); !errors.Is(err, ErrNameTaken) || fieldOf(err) != "name" {
		t.Errorf("a name taken, whatever its case: %v", err)
	}
	if _, err := f.s.CreateItem(ctx, 2, ItemInput{Name: "Coffee beans", Unit: stock.Gram}); err != nil {
		t.Errorf("another venue may use the name: %v", err)
	}
	if _, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Sugar", Unit: "kg"}); fieldOf(err) != "unit" {
		t.Errorf("a unit we do not have: %v", err)
	}

	// Milk has no history yet: its unit may change. Beans have some: not.
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Each, LowLevel: 4 * stock.PerUnit}); err != nil {
		t.Errorf("changing the unit of an item with no history: %v", err)
	}
	f.record(t, beans, MovementInput{Kind: stock.Received, Qty: kg})
	if err := f.s.UpdateItem(ctx, 1, beans, ItemInput{Name: "Coffee beans", Unit: stock.Millilitre}); fieldOf(err) != "unit" {
		t.Errorf("changing the unit of an item with history: %v", err)
	}
	if err := f.s.UpdateItem(ctx, 1, beans, ItemInput{Name: "Beans, house blend", Unit: stock.Gram, LowLevel: kg}); err != nil {
		t.Errorf("renaming it: %v", err)
	}
	if it := f.find(t, 1, beans); it.Name != "Beans, house blend" || !it.Low {
		t.Errorf("after rename = %+v", it)
	}
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Beans, house blend", Unit: stock.Each}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("renaming onto another's name: %v", err)
	}
	if err := f.s.UpdateItem(ctx, 1, 999, ItemInput{Name: "X", Unit: stock.Each}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing item: %v", err)
	}
	if err := f.s.SetItemActive(ctx, 1, 999, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("retiring a missing item: %v", err)
	}
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Error("New with no database must fail")
	}
}
