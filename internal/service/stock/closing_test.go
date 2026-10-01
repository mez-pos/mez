package stock

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/stock"
)

// shift opens (or, closed, adds a closed) shift at a venue and returns its id.
func (f fixture) shift(t *testing.T, venue int64, publicID, status string) int64 {
	t.Helper()
	closedBy, closedAt := any(0), any(nil)
	if status == "closed" {
		closedBy, closedAt = 1, "2026-09-30 18:00:00+00:00"
	}
	res, err := f.d.Writer().ExecContext(context.Background(),
		`INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at, closed_by, closed_at)
		 VALUES (?, ?, ?, '2026-09-30', 1, '2026-09-30 06:00:00+00:00', ?, ?)`, publicID, venue, status, closedBy, closedAt)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f fixture) flagged(t *testing.T, venue int64, name string, unit stock.Unit) int64 {
	t.Helper()
	id, err := f.s.CreateItem(context.Background(), venue, ItemInput{Name: name, Unit: unit, CountAtClose: true})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTheClosingCount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	milk := f.flagged(t, 1, "Milk", stock.Millilitre)
	beans := f.flagged(t, 1, "Beans", stock.Gram)
	f.item(t, 1, "Sugar", stock.Gram, 0) // not counted at close
	gone := f.flagged(t, 1, "Syrup", stock.Millilitre)
	if err := f.s.SetItemActive(ctx, 1, gone, false); err != nil {
		t.Fatal(err)
	}
	f.record(t, beans, MovementInput{Kind: stock.Received, Qty: 5 * kg})
	shift := f.shift(t, 1, "shf_CLOSE001", "open")

	before, err := f.s.ClosingCount(ctx, 1, shift)
	if err != nil {
		t.Fatal(err)
	}
	if before.Done || len(before.Items) != 2 || before.Items[0].Name != "Beans" || before.Items[1].Name != "Milk" || len(before.Results) != 0 {
		t.Fatalf("before = %+v; the flagged, active items, by name, and nothing about the ledger", before)
	}

	refusals := []struct {
		name   string
		counts []ClosingCountInput
		is     error
		field  string
	}{
		{"one missing", []ClosingCountInput{{beans, 4 * kg}}, ErrStaleCount, ""},
		{"one not on the list", []ClosingCountInput{{beans, 4 * kg}, {milk, 0}, {gone, 0}}, ErrStaleCount, ""},
		{"a count below zero", []ClosingCountInput{{beans, 4 * kg}, {milk, -1}}, nil, "counts.1.qty"},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.s.RecordClosingCount(ctx, 1, f.asha, shift, tt.counts)
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want %v", err, tt.is)
			}
			if tt.is == nil && fieldOf(err) != tt.field {
				t.Errorf("err = %v, want a refusal on %q", err, tt.field)
			}
		})
	}

	after, err := f.s.RecordClosingCount(ctx, 1, f.asha, shift, []ClosingCountInput{{milk, 2 * kg}, {beans, 4800 * stock.PerUnit}})
	if err != nil {
		t.Fatal(err)
	}
	if !after.Done || len(after.Results) != 2 || len(after.Items) != 0 {
		t.Fatalf("after = %+v", after)
	}
	b, m := after.Results[0], after.Results[1]
	if b.Name != "Beans" || b.Counted != 4800*stock.PerUnit || b.Delta != -200*stock.PerUnit {
		t.Errorf("beans = %+v; 5 kg on the ledger, 4.8 kg counted", b)
	}
	if m.Name != "Milk" || m.Delta != 2*kg {
		t.Errorf("milk = %+v; nothing on the ledger, 2 litres counted", m)
	}
	if it := f.find(t, 1, beans); it.OnHand != 4800*stock.PerUnit {
		t.Errorf("beans on hand %d after the count", it.OnHand)
	}
	page, _, err := f.s.Movements(ctx, 1, beans, 0, 1)
	if err != nil || page[0].Kind != stock.Counted || page[0].ShiftID != shift || page[0].StaffName != "Asha" {
		t.Errorf("the count's movement = %+v, %v", page, err)
	}
	if read, err := f.s.ClosingCount(ctx, 1, shift); err != nil || !read.Done || len(read.Results) != 2 {
		t.Errorf("read back = %+v, %v", read, err)
	}

	if _, err := f.s.RecordClosingCount(ctx, 1, f.asha, shift, []ClosingCountInput{{milk, 0}, {beans, 0}}); !errors.Is(err, ErrAlreadyCounted) {
		t.Errorf("counting again: %v", err)
	}
}

func TestTheClosingCountOfAShiftItMayNotCount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	closed := f.shift(t, 1, "shf_CLOSED01", "closed")
	elsewhere := f.shift(t, 2, "shf_ELSEWHR1", "open")

	if _, err := f.s.RecordClosingCount(ctx, 1, f.asha, closed, nil); !errors.Is(err, ErrShiftClosed) {
		t.Errorf("a closed shift: %v", err)
	}
	if _, err := f.s.RecordClosingCount(ctx, 1, f.asha, elsewhere, nil); !errors.Is(err, ErrShiftNotFound) {
		t.Errorf("another venue's shift: %v", err)
	}
	if _, err := f.s.ClosingCount(ctx, 1, 999); !errors.Is(err, ErrShiftNotFound) {
		t.Errorf("a missing shift: %v", err)
	}

	// A venue with nothing flagged counts nothing: an empty count closes it.
	open := f.shift(t, 1, "shf_NOTHING1", "open")
	sheet, err := f.s.RecordClosingCount(ctx, 1, f.asha, open, nil)
	if err != nil || sheet.Done {
		t.Errorf("an empty count = %+v, %v; nothing recorded, so not done", sheet, err)
	}
}
