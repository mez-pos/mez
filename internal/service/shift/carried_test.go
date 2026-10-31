package shift

import (
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/zreport"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// withTable gives venue 1 a table, and returns its id.
func withTable(t *testing.T, f *fixture) int64 {
	t.Helper()
	var id int64
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: 1, Name: "Floor", CreatedAt: start, UpdatedAt: start})
		if err != nil {
			return err
		}
		id, err = q.CreateTable(ctx, gen.CreateTableParams{
			VenueID: 1, AreaID: area, Name: "T5", Seats: 4, Shape: "square", CreatedAt: start, UpdatedAt: start,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// openAt opens an order for meta with one line of price on it, and takes
// tenders against it.
func (w *till) openAt(meta svcorder.Meta, open svcorder.OpenOrder, price int64, tenders ...svcorder.TenderPayment) int64 {
	w.t.Helper()
	res := must(w.orders.Open(ctx, meta, open))
	if price > 0 {
		must(w.orders.AddLine(ctx, meta, res.OrderID, svcorder.AddLine{ItemID: 1, Name: "Thali", Qty: 1, UnitPrice: inr(price), Station: "kitchen"}))
	}
	for _, tp := range tenders {
		must(w.orders.TenderPayment(ctx, meta, res.OrderID, tp))
	}
	return res.OrderID
}

func TestTheCloseCountsTheOrdersItCarries(t *testing.T) {
	f := newFixture(t)
	withStaff(t, f)
	table := withTable(t, f)
	w := newTillFor(t, f)
	first := must(f.svc.Open(ctx, cashier, float(t, 0)))

	counter := svcorder.OpenOrder{Channel: order.ChannelCounter, Covers: 1}
	unpaid := w.openAt(seller, counter, 45000)
	w.openAt(seller, counter, 30000, card(10000)) // part paid: 200.00 to go
	w.openAt(seller, counter, 0)                  // opened, nothing rung up yet
	// Not carried: a counter sale its payment closed (ADR-383), a table's
	// ticket paid in full and kept for the next round, another venue's.
	w.openAt(seller, counter, 12000, cash(12000, 0))
	w.openAt(seller, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: table, Covers: 2}, 50000, card(50000))
	must(f.svc.Open(ctx, Meta{VenueID: 2, ActorID: 3, DeviceID: 7}, float(t, 0)))
	w.openAt(svcorder.Meta{VenueID: 2, ActorID: 3, DeviceID: 7}, counter, 9900)

	manager := Meta{VenueID: 1, ActorID: 2, DeviceID: 7}
	closeExact(t, f, manager, first.ID)
	want := zreport.Tally{Count: 3, Amount: inr(65000)}
	doc := must(f.svc.ZReport(ctx, 1, 1)).Document
	if doc.Carried == nil || *doc.Carried != want {
		t.Fatalf("carried = %+v, want %+v", doc.Carried, want)
	}

	// The next shift settles one; the first shift's correction still says
	// what was open when it closed, which cannot be read again.
	must(f.svc.Open(ctx, cashier, float(t, 0)))
	must(w.orders.TenderPayment(ctx, seller, unpaid, cash(45000, 0)))
	must(f.svc.CorrectZReport(ctx, manager, first.ID, "Recounted the coffee"))
	fixed := must(f.svc.ZReport(ctx, 1, 2)).Document
	if fixed.Corrects != 1 || fixed.Carried == nil || *fixed.Carried != want {
		t.Fatalf("the correction: corrects %d, carried %+v, want %+v", fixed.Corrects, fixed.Carried, want)
	}
}

func TestACloseWithNothingOpenCarriesNone(t *testing.T) {
	f := newFixture(t)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	w.sale(12000, cash(12000, 0))
	closeExact(t, f, cashier, open.ID)
	doc := must(f.svc.ZReport(ctx, 1, 1)).Document
	if want := (zreport.Tally{Amount: inr(0)}); doc.Carried == nil || *doc.Carried != want {
		t.Fatalf("carried = %+v, want none, said", doc.Carried)
	}
}
