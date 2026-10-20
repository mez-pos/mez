package shift

import (
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// TestMine: in the tipped day Ravi (3) rang up three orders and kept a
// 450.00 payment with a 50.00 tip (a second, of 50.00 with 10.00, was
// voided), and Priya (2) one order of 300.00 with 20.00.
func TestMine(t *testing.T) {
	f, open := tippedDay(t)

	ravi := must(f.svc.Mine(ctx, 1, 3))
	if ravi.ShiftID != open.ID || ravi.Orders != 2 || ravi.Tables != 0 || ravi.Sales != inr(45000) || ravi.Tips != inr(5000) {
		t.Errorf("Ravi: %+v", ravi)
	}
	if len(ravi.ByTable) != 1 || ravi.ByTable[0].TableID != 0 || ravi.ByTable[0].Tips != inr(5000) || ravi.First == nil {
		t.Errorf("Ravi's tips by table: %+v, first %v", ravi.ByTable, ravi.First)
	}

	priya := must(f.svc.Mine(ctx, 1, 2))
	if priya.Orders != 1 || priya.Sales != inr(30000) || priya.Tips != inr(2000) {
		t.Errorf("Priya: %+v", priya)
	}

	asha := must(f.svc.Mine(ctx, 1, 1))
	if asha.Orders != 0 || asha.Sales != inr(0) || asha.Tips != inr(0) || asha.ByTable != nil || asha.First != nil {
		t.Errorf("Asha did nothing: %+v", asha)
	}
}

func TestMineAtATable(t *testing.T) {
	f := newFixture(t)
	w := newTillFor(t, f)
	must(f.svc.Open(ctx, cashier, float(t, 0)))
	var table int64
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: 1, Name: "Floor", CreatedAt: start, UpdatedAt: start})
		if err != nil {
			return err
		}
		table, err = q.CreateTable(ctx, gen.CreateTableParams{VenueID: 1, AreaID: area, Name: "T4", Seats: 4, Shape: "square", CreatedAt: start, UpdatedAt: start})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	res := must(w.orders.Open(ctx, seller, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: table, Covers: 2}))
	must(w.orders.AddLine(ctx, seller, res.OrderID, svcorder.AddLine{ItemID: 1, Name: "Thali", Qty: 1, UnitPrice: inr(20000), Station: "kitchen"}))
	must(w.orders.TenderPayment(ctx, seller, res.OrderID, svcorder.TenderPayment{Method: order.PaymentUPI, Amount: inr(20000), Tip: inr(1500)}))

	got := must(f.svc.Mine(ctx, 1, 3))
	if got.Orders != 1 || got.Tables != 1 || got.Sales != inr(20000) || len(got.ByTable) != 1 || got.ByTable[0].TableID != table || got.ByTable[0].Tips != inr(1500) {
		t.Errorf("at the table: %+v", got)
	}
}

func TestMineNeedsAnOpenShift(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Mine(ctx, 1, 3); !errors.Is(err, ErrNoOpenShift) {
		t.Fatalf("with no shift open: %v", err)
	}
}
