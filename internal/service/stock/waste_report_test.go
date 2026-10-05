package stock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/stock"
)

// clockAt is a clock stopped at an instant.
type clockAt time.Time

func (c clockAt) Now() time.Time { return time.Time(c) }

// wasteAt writes stock off at an instant, like "2026-09-29T06:00:00Z": a
// movement is written once and never changed, so the entry is keyed through
// a service whose clock stands there.
func (f fixture) wasteAt(t *testing.T, id int64, qty stock.Qty, reason stock.WasteReason, at string) {
	t.Helper()
	when, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(f.d, clockAt(when))
	if err != nil {
		t.Fatal(err)
	}
	in := MovementInput{Kind: stock.Wasted, Qty: qty, Reason: reason}
	if reason == stock.Other {
		in.Note = "left out overnight"
	}
	if _, err := s.Record(context.Background(), 1, f.asha, id, in); err != nil {
		t.Fatalf("writing off %+v: %v", in, err)
	}
}

// aWeekOfWaste is worked out by hand. The venue is in Kolkata (UTC+5:30) and
// its trading day starts at 04:00, so 29 Sept runs from 22:30 UTC on the
// 28th to 22:30 UTC on the 29th.
//
//	Milk     60.00 a litre   500 ml spilled on 29 Sept   30.00
//	                         250 ml expired on 30 Sept   15.00
//	Tomato   no cost         2 kg spoiled on 29 Sept     not costed
//	Milk                     1 l spoiled on 28 Sept      outside the range
//	Milk                     1 l dropped at 03:00 on 1 Oct, which is still 30 Sept's trade   60.00
func aWeekOfWaste(t *testing.T) (fixture, int64, int64) {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	milk, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Milk", Unit: stock.Millilitre, Cost: inr(6000)})
	if err != nil {
		t.Fatal(err)
	}
	tomato := f.item(t, 1, "Tomato", stock.Gram, 0)
	f.record(t, milk, MovementInput{Kind: stock.Received, Qty: 10_000 * ml})
	f.record(t, tomato, MovementInput{Kind: stock.Received, Qty: 5 * kg})

	f.wasteAt(t, milk, 1000*ml, stock.Spoiled, "2026-09-28T10:00:00Z")
	f.wasteAt(t, milk, 500*ml, stock.Spilled, "2026-09-29T06:00:00Z")
	f.wasteAt(t, tomato, 2*kg, stock.Spoiled, "2026-09-29T12:00:00Z")
	f.wasteAt(t, milk, 250*ml, stock.Expired, "2026-09-30T08:00:00Z")
	// 03:00 in Kolkata on 1 Oct: before the day start, so 30 Sept's.
	f.wasteAt(t, milk, 1000*ml, stock.Dropped, "2026-09-30T21:30:00Z")
	return f, milk, tomato
}

func TestTheWasteReport(t *testing.T) {
	f, milk, tomato := aWeekOfWaste(t)
	got, err := f.s.WasteReport(context.Background(), 1, "2026-09-29", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "2026-09-29" || got.To != "2026-09-30" || got.Currency != "INR" {
		t.Errorf("range %s to %s in %s", got.From, got.To, got.Currency)
	}
	if got.Value != inr(10500) || got.Entries != 4 || got.Uncosted != 1 {
		t.Errorf("total %s over %d entries, %d not costed; want 105.00 over 4, 1 not costed", got.Value, got.Entries, got.Uncosted)
	}

	// The largest loss first; an item with no cost after those with one.
	wantItems := []WasteItem{
		{StockItemID: milk, Name: "Milk", Unit: stock.Millilitre, Qty: 1750 * ml, Entries: 3, Value: inr(10500)},
		{StockItemID: tomato, Name: "Tomato", Unit: stock.Gram, Qty: 2 * kg, Entries: 1, Value: inr(0), Uncosted: 1},
	}
	if len(got.Items) != len(wantItems) {
		t.Fatalf("items %+v", got.Items)
	}
	for i, want := range wantItems {
		if got.Items[i] != want {
			t.Errorf("item %d = %+v\n     want %+v", i, got.Items[i], want)
		}
	}

	wantReasons := []WasteReasonTotal{
		{Reason: stock.Dropped, Entries: 1, Value: inr(6000)},
		{Reason: stock.Spilled, Entries: 1, Value: inr(3000)},
		{Reason: stock.Expired, Entries: 1, Value: inr(1500)},
		{Reason: stock.Spoiled, Entries: 1, Value: inr(0), Uncosted: 1},
	}
	if len(got.Reasons) != len(wantReasons) {
		t.Fatalf("reasons %+v", got.Reasons)
	}
	for i, want := range wantReasons {
		if got.Reasons[i] != want {
			t.Errorf("reason %d = %+v\n       want %+v", i, got.Reasons[i], want)
		}
	}
}

func TestAQuietRangeWastesNothing(t *testing.T) {
	f, _, _ := aWeekOfWaste(t)
	got, err := f.s.WasteReport(context.Background(), 1, "2026-10-02", "2026-10-03")
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries != 0 || got.Value != inr(0) || got.Items == nil || got.Reasons == nil || len(got.Items) != 0 {
		t.Errorf("a quiet range = %+v; want nothing, as empty lists", got)
	}
}

func TestWasteIsTheVenuesOwn(t *testing.T) {
	f, _, _ := aWeekOfWaste(t)
	got, err := f.s.WasteReport(context.Background(), 2, "2026-09-29", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries != 0 {
		t.Errorf("venue 2 sees %d of venue 1's entries", got.Entries)
	}
}

func TestTheWasteReportRefusesABadRange(t *testing.T) {
	f := newFixture(t)
	for _, tt := range [][2]string{{"2026-09-30", "2026-09-29"}, {"", "2026-09-29"}, {"2026-09-29", "soon"}, {"2025-01-01", "2026-09-29"}} {
		if _, err := f.s.WasteReport(context.Background(), 1, tt[0], tt[1]); !errors.Is(err, ErrBadRange) {
			t.Errorf("%s to %s: %v, want ErrBadRange", tt[0], tt[1], err)
		}
	}
}

// The key is spelt out here because the services that own it import this
// one. If the day start setting is ever renamed, this is what notices.
func TestTheDayStartSettingMovesTheEdgeOfADay(t *testing.T) {
	f, _, _ := aWeekOfWaste(t)
	ctx := context.Background()
	if _, err := f.d.Writer().ExecContext(ctx,
		`INSERT INTO settings (venue_id, key, value, updated_at) VALUES (1, 'shift.day_start', '02:00', '2026-09-01 00:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	// With the day starting at 02:00, the litre dropped at 03:00 on 1 Oct is 1 Oct's.
	got, err := f.s.WasteReport(ctx, 1, "2026-09-29", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries != 3 || got.Value != inr(4500) {
		t.Errorf("%d entries worth %s; want 3 worth 45.00 once 03:00 belongs to the next day", got.Entries, got.Value)
	}
}
