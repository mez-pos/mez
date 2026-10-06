package audit

import (
	"context"
	"slices"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A tip a manager let through over the venue's ceiling is in the trail, worth
// the tip (ADR-261). A payment whose tip needed nobody is not a correction.
func TestATipApprovedIsInTheTrail(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	m := svcorder.Meta{VenueID: 1, ActorID: f.asha, DeviceID: 1}
	id := must(t)(f.orders.Open(ctx, m, svcorder.OpenOrder{Channel: order.ChannelCounter})).OrderID
	must(t)(f.orders.AddLine(ctx, m, id, line("Thali", 1, 20000)))
	// 10% on a 100.00 card payment: under the ceiling, nobody needed.
	must(t)(f.orders.TenderPayment(ctx, m, id, svcorder.TenderPayment{Method: order.PaymentCard, Amount: inr(10000), Tip: inr(1000)}))
	// 90.00 on the next 100.00: over half, so Priya approved it.
	must(t)(f.orders.TenderPayment(ctx, m, id, svcorder.TenderPayment{
		Method: order.PaymentUPI, Amount: inr(10000), Tip: inr(9000), AuthorisedBy: f.priya,
	}))

	page, err := f.s.Trail(ctx, 1, day("2026-10-01", "2026-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 || page.Counts[TipApproved] != 1 {
		t.Fatalf("entries %+v, counts %v", page.Entries, page.Counts)
	}
	e := page.Entries[0]
	if e.Kind != TipApproved || e.Amount == nil || *e.Amount != inr(9000) || e.By == nil || e.By.Name != "Asha" ||
		e.AuthorisedBy == nil || e.AuthorisedBy.Name != "Priya" || e.Item != "" || e.Reason != "" {
		t.Errorf("entry = %+v", e)
	}

	// The kind filters to it, and a person finds what they approved.
	only, err := f.s.Trail(ctx, 1, Filter{From: "2026-10-01", To: "2026-10-01", Kind: TipApproved, StaffID: "stf_PRIYA001"})
	if err != nil || len(only.Entries) != 1 {
		t.Fatalf("filtered: %+v, %v", only.Entries, err)
	}
	if voids, err := f.s.Trail(ctx, 1, Filter{From: "2026-10-01", To: "2026-10-01", Kind: Void}); err != nil || len(voids.Entries) != 0 {
		t.Fatalf("voids: %+v, %v", voids.Entries, err)
	}

	if !slices.Contains(Kinds(), TipApproved) || kindWords[TipApproved] != "Tip approved" {
		t.Errorf("kinds %v, words %q", Kinds(), kindWords[TipApproved])
	}
	// Every kind has its words, alone and as a column of them.
	for _, k := range Kinds() {
		if kindWords[k] == "" || kindsWords[k] == "" {
			t.Errorf("%s has no words", k)
		}
	}
	table, err := f.s.Table(ctx, 1, day("2026-10-01", "2026-10-01"))
	if err != nil || len(table.Rows) != 1 || table.Rows[0][2] != "Tip approved" || table.Rows[0][6] != "90.00" {
		t.Fatalf("table: %+v, %v", table.Rows, err)
	}
}
