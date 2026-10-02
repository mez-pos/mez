package order

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// waker counts wakes.
type waker struct{ n atomic.Int64 }

func (w *waker) Wake() { w.n.Add(1) }

// printingService is a service that queues print jobs, at a venue with a
// receipt printer (1, 58mm), a kitchen printer (2) on the kitchen station,
// and a bar station with no printer.
func printingService(t *testing.T, receipt bool) (*Service, *db.DB, *waker) {
	t.Helper()
	_, d, _ := newService(t)
	w := &waker{}
	s, err := New(Options{DB: d, Clock: fixedClock{}, Prints: w})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO printers (venue_id, name, transport, address, paper_mm, is_receipt, created_at, updated_at)
		 VALUES (1, 'Counter', 'tcp', '10.0.0.5', 58, 1, '2026-09-18 04:30:00+00:00', '2026-09-18 04:30:00+00:00')`,
		`INSERT INTO printers (venue_id, name, transport, address, created_at, updated_at)
		 VALUES (1, 'Kitchen', 'tcp', '10.0.0.6', '2026-09-18 04:30:00+00:00', '2026-09-18 04:30:00+00:00')`,
		`INSERT INTO stations (venue_id, code, name, printer_id, created_at, updated_at)
		 VALUES (1, 'kitchen', 'Kitchen', 2, '2026-09-18 04:30:00+00:00', '2026-09-18 04:30:00+00:00')`,
		`INSERT INTO stations (venue_id, code, name, created_at, updated_at)
		 VALUES (1, 'bar', 'Bar', '2026-09-18 04:30:00+00:00', '2026-09-18 04:30:00+00:00')`,
	} {
		if _, err := d.Writer().ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if !receipt {
		if _, err := d.Writer().ExecContext(ctx, `UPDATE printers SET is_receipt = 0 WHERE id = 1`); err != nil {
			t.Fatal(err)
		}
	}
	return s, d, w
}

func jobs(t *testing.T, d *db.DB) []gen.PrintJobsForVenueRow {
	t.Helper()
	all, err := d.Queries().PrintJobsForVenue(context.Background(), gen.PrintJobsForVenueParams{VenueID: 1, Since: now.Add(-24 * time.Hour), MaxJobs: 100})
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func payload(t *testing.T, d *db.DB, id int64) []byte {
	t.Helper()
	j, err := d.Queries().PrintJobByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return j.Payload
}

// TestPrintJobsFromOrders: a kitchen ticket when lines are sent, a receipt
// with the drawer kick when a cash payment settles, a duplicate on reprint,
// each in the command that caused it.
func TestPrintJobsFromOrders(t *testing.T) {
	s, d, w := printingService(t, true)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, id, dosa(1)))
	must(t)(s.AddLine(ctx, m, id, coffee(1)))
	must(t)(s.AddNote(ctx, m, id, AddNote{Scope: order.ScopeLine, LineID: 2, Text: "no onion"}))
	if len(jobs(t, d)) != 0 || w.n.Load() != 0 {
		t.Fatal("jobs before anything was sent or paid")
	}

	must(t)(s.SendTicket(ctx, m, id, SendTicket{Station: "kitchen"}))
	must(t)(s.SendTicket(ctx, m, id, SendTicket{Station: "bar"}))
	got := jobs(t, d)
	if len(got) != 2 || w.n.Load() != 2 {
		t.Fatalf("after two sends: %d jobs, %d wakes", len(got), w.n.Load())
	}
	bar, kitchen := got[0], got[1] // newest first
	if kitchen.Label != "Kitchen ticket, table 4" || kitchen.State != "queued" || kitchen.PrinterID.Int64 != 2 || kitchen.Station != "kitchen" {
		t.Errorf("kitchen job = %+v", kitchen)
	}
	if p := payload(t, d, kitchen.ID); !bytes.Contains(p, []byte("1 x Masala dosa")) || !bytes.Contains(p, []byte("NOTE: no onion")) || bytes.Contains(p, []byte("Filter coffee")) {
		t.Errorf("kitchen ticket:\n%q", p)
	}
	// The bar has no printer: its ticket is kept, dead, and says why.
	if bar.State != "dead" || bar.LastError != printq.NoPrinter || bar.PrinterID.Valid || bar.Station != "bar" {
		t.Errorf("bar job = %+v", bar)
	}

	// A refused payment queues nothing.
	if _, err := s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: inr(99999)}); err == nil {
		t.Fatal("an over-tender was taken")
	}
	if len(jobs(t, d)) != 2 || w.n.Load() != 2 {
		t.Fatal("a refused payment queued a job or woke the queue")
	}

	must(t)(s.TenderPayment(ctx, cashier("pay-1"), id, TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
	got = jobs(t, d)
	if len(got) != 3 {
		t.Fatalf("after settling: %d jobs", len(got))
	}
	rcpt := got[0]
	if rcpt.Label != "Receipt INV/26-27/000001" || rcpt.State != "queued" || rcpt.PrinterID.Int64 != 1 || rcpt.Kind != "receipt" {
		t.Errorf("receipt job = %+v", rcpt)
	}
	p := payload(t, d, rcpt.ID)
	if !bytes.Contains(p, []byte("INV/26-27/000001")) || !bytes.Contains(p, []byte{0x10, 0x14, 1, 0, 2}) || bytes.Contains(p, []byte("DUPLICATE")) {
		t.Errorf("receipt is missing its number or the drawer kick, or says DUPLICATE")
	}
	// 58mm paper: 32 columns, so the rule under the header is 32 wide.
	if !bytes.Contains(p, bytes.Repeat([]byte("-"), 32)) || bytes.Contains(p, bytes.Repeat([]byte("-"), 33)) {
		t.Error("the receipt is not laid out for the printer's 58mm paper")
	}
	// A retry of the same payment is a replay, not a second receipt.
	must(t)(s.TenderPayment(ctx, cashier("pay-1"), id, TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
	if len(jobs(t, d)) != 3 {
		t.Error("a replayed payment queued another receipt")
	}

	must(t)(s.Close(ctx, m, id))
	res := must(t)(s.Reprint(ctx, m, id, ReprintInvoice{InvoiceSeq: 8, Reason: "guest_copy", AuthorisedBy: 7}))
	got = jobs(t, d)
	if len(got) != 4 || got[0].Label != "Duplicate INV/26-27/000001" || got[0].PrinterID.Int64 != 1 {
		t.Fatalf("after the reprint: %+v", got[0])
	}
	dup := payload(t, d, got[0].ID)
	if !bytes.Contains(dup, []byte("DUPLICATE")) || bytes.Contains(dup, []byte{0x10, 0x14}) {
		t.Error("the duplicate does not say so, or kicks the drawer")
	}
	j, _ := d.Queries().PrintJobByID(ctx, got[0].ID)
	if j.EventSeq.Int64 != res.Events[0].Seq {
		t.Errorf("the duplicate names event %d, the reprint was %d", j.EventSeq.Int64, res.Events[0].Seq)
	}
}

// TestReceiptWithNoPrinter: a receipt with no receipt printer is kept,
// dead, and prints once one is set and the job retried.
func TestReceiptWithNoPrinter(t *testing.T) {
	s, d, _ := printingService(t, false)
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCard, Amount: inr(4200)}))
	rcpt := jobs(t, d)[0]
	if rcpt.State != "dead" || rcpt.LastError != printq.NoPrinter {
		t.Fatalf("receipt = %+v", rcpt)
	}
	if p := payload(t, d, rcpt.ID); bytes.Contains(p, []byte{0x10, 0x14}) {
		t.Error("a card payment kicked the drawer")
	}

	q, err := printq.New(printq.Options{DB: d, Clock: fixedClock{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Retry(ctx, 1, rcpt.ID); err == nil {
		t.Error("a retry with still no printer was taken")
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE printers SET is_receipt = 1 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := q.Retry(ctx, 1, rcpt.ID); err != nil {
		t.Fatal(err)
	}
	if again := jobs(t, d)[0]; again.State != "queued" || again.PrinterID.Int64 != 1 || again.LastError != "" {
		t.Errorf("after the retry: %+v", again)
	}
}

// TestCreditNotePrints: a credit note prints on the receipt printer, and
// opens the drawer when the money goes back in cash.
func TestCreditNotePrints(t *testing.T) {
	s, d, _ := printingService(t, true)
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, dosa(2)))
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(25200)}))
	must(t)(s.Credit(ctx, cashier(""), id, IssueCredit{
		Credit: order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}, Reason: "cold_or_late", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: paid.State.Tenders[0].ID, Amount: inr(12600)}},
	}))
	cn := jobs(t, d)[0]
	if cn.Label != "Credit note CN/26-27/000001" || cn.PrinterID.Int64 != 1 || cn.Kind != "receipt" {
		t.Fatalf("credit note job = %+v", cn)
	}
	p := payload(t, d, cn.ID)
	if !bytes.Contains(p, []byte("CREDIT NOTE")) || !bytes.Contains(p, []byte("Against invoice INV/26-27/000001")) ||
		!bytes.Contains(p, []byte("Reason: Cold or late")) || !bytes.Contains(p, []byte("Refunded to Cash")) ||
		!bytes.Contains(p, []byte{0x10, 0x14, 1, 0, 2}) {
		t.Errorf("credit note payload:\n%q", p)
	}
}
