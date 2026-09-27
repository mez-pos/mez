package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// now is the fixed clock every test runs at: 10:00 IST on 18 September 2026,
// in financial year 2026-27.
var now = time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

// recorder is a Publisher that keeps what it was given.
type recorder struct {
	mu      sync.Mutex
	changes []Change
}

func (r *recorder) Publish(_ context.Context, c Change) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changes = append(r.changes, c)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.changes)
}

// cashier is staff member 7 on terminal 3 at venue 1.
func cashier(key string) Meta { return Meta{VenueID: 1, ActorID: 7, DeviceID: 3, IdempotencyKey: key} }

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// dosa and coffee are GST lines at 5% under the restaurant SAC.
func dosa(qty int64) AddLine {
	return AddLine{ItemID: 1, Name: "Masala dosa", Qty: qty, UnitPrice: inr(12000), Station: "kitchen", TaxRate: 5000, HSN: "996331"}
}

func coffee(qty int64) AddLine {
	return AddLine{ItemID: 2, Name: "Filter coffee", Qty: qty, UnitPrice: inr(4000), Station: "bar", TaxRate: 5000, HSN: "996331"}
}

// newService is a service over a fresh database with one venue: a Bengaluru
// cafe on GST, pricing exclusive.
func newService(t *testing.T) (*Service, *db.DB, *recorder) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	addVenue(t, d, "ven_CAFE0001", map[string]string{SettingTaxStateCode: "29", SettingTaxPricing: "exclusive"})

	rec := &recorder{}
	s, err := New(Options{DB: d, Clock: fixedClock{}, Publisher: rec})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, d, rec
}

func addVenue(t *testing.T, d *db.DB, publicID string, settings map[string]string) {
	t.Helper()
	ctx := context.Background()
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: publicID, Name: "Filter Coffee House", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		for k, val := range settings {
			if err := q.SetSetting(ctx, gen.SetSettingParams{VenueID: v.ID, Key: k, Value: val, UpdatedAt: now}); err != nil {
				return err
			}
		}
		return seedTables(ctx, q, v.ID)
	})
	if err != nil {
		t.Fatalf("adding venue %s: %v", publicID, err)
	}
}

// seedTables gives a venue an area with tables 1 to 10. In the first venue
// of a fresh database their ids are 1 to 10, so table 4 is table id 4.
func seedTables(ctx context.Context, q *gen.Queries, venueID int64) error {
	area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: venueID, Name: "Floor", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return err
	}
	for i := int64(1); i <= 10; i++ {
		if _, err := q.CreateTable(ctx, gen.CreateTableParams{
			VenueID: venueID, AreaID: area, Name: fmt.Sprint(i), Seats: 4, Shape: "square", X: i - 1, Y: 0,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// must fails the test on a command error: must(t)(s.AddLine(...)).
func must(t *testing.T) func(Result, error) Result {
	return func(res Result, err error) Result {
		t.Helper()
		if err != nil {
			t.Fatalf("command: %v", err)
		}
		return res
	}
}

func open(t *testing.T, s *Service) Result {
	t.Helper()
	return must(t)(s.Open(context.Background(), cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))
}

// TestAWholeSale is the phase's done-when run through the service: open,
// add lines, send the kitchen its ticket, split the bill across two tenders,
// close, and find the invoice issued, registered and carrying the tax.
func TestAWholeSale(t *testing.T) {
	s, d, rec := newService(t)
	ctx := context.Background()
	m := cashier("")

	opened := open(t, s)
	if opened.State.Currency != "INR" || opened.State.OpenedBy != 7 || opened.PublicID == "" {
		t.Fatalf("opened = %+v", opened.State)
	}
	id := opened.OrderID
	must(t)(s.AddLine(ctx, m, id, dosa(2)))
	must(t)(s.AddLine(ctx, m, id, coffee(3)))
	sent := must(t)(s.SendTicket(ctx, m, id, SendTicket{Station: "kitchen"}))
	if tk := sent.State.Tickets; len(tk) != 1 || !reflect.DeepEqual(tk[0].LineIDs, []int64{2}) {
		t.Fatalf("tickets = %+v", tk)
	}

	due := sent.State.Totals.BalanceDue
	if due != inr(37800) {
		t.Fatalf("balance due %+v, want 378.00: 360.00 plus 5%% GST", due)
	}
	must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: inr(20000)}))
	paid := must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCard, Amount: inr(17800), Tip: inr(2000), Reference: "AUTH 1"}))
	if !paid.State.Totals.BalanceDue.IsZero() || paid.State.Totals.Tips != inr(2000) {
		t.Fatalf("after paying: %+v", paid.State.Totals)
	}

	closed := must(t)(s.Close(ctx, m, id))
	if closed.State.Status != order.StatusClosed || closed.State.ClosedBy != 7 {
		t.Fatalf("closed = %s by %d", closed.State.Status, closed.State.ClosedBy)
	}
	if len(paid.Events) != 2 || paid.Events[1].Type != order.TypeInvoiceIssued {
		t.Fatalf("the settling tender appended %+v, want payment.tendered then invoice.issued", paid.Events)
	}
	if len(closed.Events) != 1 || closed.Events[0].Type != order.TypeOrderClosed {
		t.Fatalf("close appended %+v, want only order.closed: the invoice came with the payment", closed.Events)
	}
	issued := paid.Events[1].Payload.(order.InvoiceIssued)
	if p := issued.Prices; p == nil || p.Total != inr(37800) || len(p.Lines) != 2 || p.Lines[0].Name != "Masala dosa" ||
		p.Seller.Registration != "29ABCDE1234F1Z5" || p.Seller.StateCode != "29" || p.Seller.Name != "Filter Coffee House" {
		t.Errorf("price snapshot = %+v", issued.Prices)
	}
	if issued.InvoiceNo != "INV/26-27/000001" || issued.Series != "INV" {
		t.Errorf("invoice = %+v", issued)
	}
	var snapshot tax.TaxResult
	if err := json.Unmarshal(issued.TaxSnapshot, &snapshot); err != nil {
		t.Fatalf("tax snapshot: %v", err)
	}
	if !reflect.DeepEqual(snapshot, closed.State.TaxDetail) || snapshot.Tax != inr(1800) {
		t.Errorf("tax snapshot %+v, want the bill's tax detail with 18.00 of GST", snapshot)
	}

	for _, e := range append(paid.Events, closed.Events...) {
		if !e.At.Equal(now) || e.ActorID != 7 || e.DeviceID != 3 {
			t.Errorf("event %s stamped %v by %d on %d", e.Type, e.At, e.ActorID, e.DeviceID)
		}
	}
	register, err := d.Queries().InvoicesForVenue(ctx, 1)
	if err != nil || len(register) != 1 || register[0].OrderID != id || register[0].InvoiceNo != issued.InvoiceNo {
		t.Errorf("register = %+v, %v", register, err)
	}
	if got := rec.count(); got != 7 {
		t.Errorf("%d changes published, want one per command: 7", got)
	}
	read, err := s.Order(ctx, 1, id)
	if err != nil || !reflect.DeepEqual(read.State, closed.State) {
		t.Errorf("Order read back differs from the last result: %v", err)
	}
}

func TestIdempotencyKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("a repeated command is applied once", func(t *testing.T) {
		s, _, rec := newService(t)
		id := open(t, s).OrderID
		must(t)(s.AddLine(ctx, cashier(""), id, dosa(1)))
		must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentUPI, Amount: inr(5000)}))
		pay := TenderPayment{Method: order.PaymentCard, Amount: inr(4000)}
		a := must(t)(s.TenderPayment(ctx, cashier("pay-42"), id, pay))
		published := rec.count()

		// A later command moves the order on before the retry arrives.
		must(t)(s.AddNote(ctx, cashier(""), id, AddNote{Scope: order.ScopeOrder, Text: "after"}))
		b := must(t)(s.TenderPayment(ctx, cashier("pay-42"), id, pay))

		if a.Replayed || !b.Replayed {
			t.Fatalf("replayed flags: first %v, retry %v", a.Replayed, b.Replayed)
		}
		if !reflect.DeepEqual(a.Events, b.Events) || !reflect.DeepEqual(a.State, b.State) || a.PublicID != b.PublicID {
			t.Errorf("the retry's result is not the first result")
		}
		if rec.count() != published+1 {
			t.Errorf("the retry was published")
		}
		current, err := s.Order(ctx, 1, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(current.State.Tenders) != 2 {
			t.Errorf("%d tenders, want 2: the card was charged twice", len(current.State.Tenders))
		}
	})

	t.Run("a key reused for something else is refused", func(t *testing.T) {
		s, _, _ := newService(t)
		id := open(t, s).OrderID
		other := open(t, s).OrderID
		must(t)(s.AddLine(ctx, cashier("k1"), id, coffee(1)))
		if _, err := s.AddLine(ctx, cashier("k1"), id, coffee(2)); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("different body: err = %v", err)
		}
		if _, err := s.AddLine(ctx, cashier("k1"), other, coffee(1)); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("different order: err = %v", err)
		}
		if _, err := s.AddNote(ctx, cashier("k1"), id, AddNote{Scope: order.ScopeOrder, Text: "x"}); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("different command: err = %v", err)
		}
	})

	t.Run("a failed command does not use up its key", func(t *testing.T) {
		s, _, _ := newService(t)
		id := open(t, s).OrderID
		must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
		if _, err := s.Close(ctx, cashier("close-1"), id); !errors.Is(err, order.ErrBalanceDue) {
			t.Fatalf("close with a balance: err = %v, want ErrBalanceDue", err)
		}
		must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
		res := must(t)(s.Close(ctx, cashier("close-1"), id))
		if res.Replayed || res.State.Status != order.StatusClosed {
			t.Errorf("close after paying: replayed %v, status %s", res.Replayed, res.State.Status)
		}
	})

	t.Run("an open is repeated once", func(t *testing.T) {
		s, _, _ := newService(t)
		cmd := OpenOrder{Channel: order.ChannelQRTable, TableID: 9, Covers: 4}
		a := must(t)(s.Open(ctx, cashier("open-9"), cmd))
		b := must(t)(s.Open(ctx, cashier("open-9"), cmd))
		if !b.Replayed || a.OrderID != b.OrderID || a.PublicID != b.PublicID || !reflect.DeepEqual(a.State, b.State) {
			t.Errorf("retried open: %+v vs %+v", a, b)
		}
		if _, err := s.Open(ctx, cashier("open-9"), OpenOrder{Channel: order.ChannelQRTable, TableID: 10}); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("open with the same key for another table: err = %v", err)
		}
	})

	t.Run("a key must be printable", func(t *testing.T) {
		s, _, _ := newService(t)
		for _, key := range []string{"tab\there", "é", string(make([]byte, 256))} {
			if _, err := s.Open(ctx, cashier(key), OpenOrder{Channel: order.ChannelCounter}); !errors.Is(err, ErrInvalidCommand) {
				t.Errorf("key %q: err = %v", key, err)
			}
		}
	})
}

// TestConcurrentRetriesApplyOnce is a terminal's retry arriving while the
// first attempt is still in flight. The single writer serialises them; the
// second finds the first's key.
func TestConcurrentRetriesApplyOnce(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, dosa(3)))

	var wg sync.WaitGroup
	results := make(chan Result, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.TenderPayment(ctx, cashier("retry-storm"), id, TenderPayment{Method: order.PaymentCard, Amount: inr(10000)})
			if err != nil {
				t.Errorf("TenderPayment: %v", err)
				return
			}
			results <- res
		}()
	}
	wg.Wait()
	close(results)
	fresh := 0
	for r := range results {
		if !r.Replayed {
			fresh++
		}
	}
	state, err := s.Order(ctx, 1, id)
	if err != nil {
		t.Fatal(err)
	}
	if fresh != 1 || len(state.State.Tenders) != 1 {
		t.Errorf("%d applied, %d tenders: want exactly one", fresh, len(state.State.Tenders))
	}
}

func TestAnotherVenuesOrderIsNotFound(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	addVenue(t, d, "ven_CAFE0002", map[string]string{SettingTaxStateCode: "29"})
	id := open(t, s).OrderID
	elsewhere := Meta{VenueID: 2, ActorID: 1, DeviceID: 1}

	if _, err := s.AddLine(ctx, elsewhere, id, coffee(1)); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("AddLine from another venue: err = %v", err)
	}
	if _, err := s.Order(ctx, 2, id); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("Order from another venue: err = %v", err)
	}
	if _, err := s.AddLine(ctx, cashier(""), 404, coffee(1)); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("AddLine to no order: err = %v", err)
	}
	if _, err := s.Order(ctx, 1, 404); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("Order of no order: err = %v", err)
	}
}

// TestInvoicesStayGapless issues, refuses and closes across two orders and
// checks the numbers that result.
func TestInvoicesStayGapless(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	m := cashier("")

	a := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, a, coffee(1)))
	if _, err := s.IssueInvoice(ctx, m, a); !errors.Is(err, order.ErrBalanceDue) {
		t.Fatalf("invoice with a balance: err = %v", err)
	}
	// A tender the order refuses allocates nothing, though it would have
	// settled the bill had it been in the right currency.
	if _, err := s.TenderPayment(ctx, m, a, TenderPayment{Method: order.PaymentCash, Amount: money.Amount{Minor: 4200, Currency: "USD"}}); err == nil {
		t.Fatal("a dollar tender on a rupee bill was taken")
	}
	part := must(t)(s.TenderPayment(ctx, m, a, TenderPayment{Method: order.PaymentCash, Amount: inr(2000)}))
	if len(part.State.Invoices) != 0 {
		t.Fatalf("a part payment issued %+v", part.State.Invoices)
	}
	// The tender that settles the bill issues its invoice in the same command.
	inv := must(t)(s.TenderPayment(ctx, m, a, TenderPayment{Method: order.PaymentCard, Amount: inr(2200)}))
	if len(inv.Events) != 2 || inv.Events[1].Type != order.TypeInvoiceIssued {
		t.Fatalf("settling tender appended %+v, want payment.tendered then invoice.issued", inv.Events)
	}
	if no := inv.State.Invoices[0].InvoiceNo; no != "INV/26-27/000001" {
		t.Fatalf("first invoice %s: the refused ones left a gap", no)
	}
	if _, err := s.IssueInvoice(ctx, m, a); !errors.Is(err, order.ErrAlreadyInvoiced) {
		t.Errorf("second invoice: err = %v", err)
	}
	closed := must(t)(s.Close(ctx, m, a))
	if len(closed.Events) != 1 || len(closed.State.Invoices) != 1 {
		t.Errorf("close of an invoiced order appended %d events, %d invoices", len(closed.Events), len(closed.State.Invoices))
	}

	b := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, b, dosa(1)))
	must(t)(s.TenderPayment(ctx, m, b, TenderPayment{Method: order.PaymentUPI, Amount: inr(12600)}))
	if res := must(t)(s.Close(ctx, m, b)); res.State.Invoices[0].InvoiceNo != "INV/26-27/000002" {
		t.Errorf("second order's invoice = %s", res.State.Invoices[0].InvoiceNo)
	}

	// A bill that comes to nothing closes without a number.
	c := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, c, coffee(1)))
	must(t)(s.CompLine(ctx, m, c, CompLine{LineID: 2, ReasonCode: "birthday", AuthorisedBy: 7}))
	if res := must(t)(s.Close(ctx, m, c)); len(res.State.Invoices) != 0 {
		t.Errorf("a zero bill was invoiced: %+v", res.State.Invoices)
	}

	register, err := d.Queries().InvoicesForVenue(ctx, 1)
	if err != nil || len(register) != 2 {
		t.Errorf("register has %d invoices (%v), want 2", len(register), err)
	}
}

func TestSendTicketHoldsUnfiredCourses(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID

	starter := dosa(1)
	main := dosa(1)
	main.Name, main.Course = "Thali", 2
	must(t)(s.AddLine(ctx, m, id, starter)) // 2
	must(t)(s.AddLine(ctx, m, id, main))    // 3
	must(t)(s.AddLine(ctx, m, id, coffee(1)))

	first := must(t)(s.SendTicket(ctx, m, id, SendTicket{Station: "kitchen"}))
	if got := first.Events[0].Payload.(order.TicketSent).LineIDs; !reflect.DeepEqual(got, []int64{2}) {
		t.Errorf("first ticket sent %v, want only the starter", got)
	}
	if _, err := s.SendTicket(ctx, m, id, SendTicket{Station: "kitchen"}); !errors.Is(err, ErrNothingToSend) {
		t.Errorf("nothing waiting: err = %v", err)
	}
	// Firing course 2 sends the main with it, to the kitchen alone (ADR-089).
	fired := must(t)(s.FireCourse(ctx, m, id, FireCourse{Course: 2}))
	if len(fired.Events) != 2 || fired.Events[1].Payload.(order.TicketSent).Station != "kitchen" ||
		!reflect.DeepEqual(fired.Events[1].Payload.(order.TicketSent).LineIDs, []int64{3}) {
		t.Errorf("firing course 2 appended %+v, want the fire and the main's ticket", fired.Events)
	}
	if _, err := s.SendTicket(ctx, m, id, SendTicket{Station: "kitchen"}); !errors.Is(err, ErrNothingToSend) {
		t.Errorf("after the fire, the kitchen still had %v", err)
	}
	explicit := must(t)(s.SendTicket(ctx, m, id, SendTicket{Station: "bar", LineIDs: []int64{4}}))
	if !explicit.State.Lines[2].Sent {
		t.Error("the coffee was not marked sent")
	}
}

// TestEveryCommandReachesItsEvent runs the pass-through commands once each,
// so a command wired to the wrong event fails here and not at a till.
func TestEveryCommandReachesItsEvent(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	ten := inr(1000)

	steps := []struct {
		want order.Type
		run  func() (Result, error)
	}{
		{order.TypeLineAdded, func() (Result, error) { return s.AddLine(ctx, m, id, dosa(1)) }},   // 2
		{order.TypeLineAdded, func() (Result, error) { return s.AddLine(ctx, m, id, coffee(1)) }}, // 3
		{order.TypeLineAdded, func() (Result, error) { return s.AddLine(ctx, m, id, coffee(1)) }}, // 4
		{order.TypeLineQtyChanged, func() (Result, error) { return s.ChangeQty(ctx, m, id, ChangeQty{LineID: 2, From: 1, To: 2}) }},
		{order.TypeLineVoided, func() (Result, error) {
			return s.VoidLine(ctx, m, id, VoidLine{LineID: 3, ReasonCode: "wrong_item", AuthorisedBy: 7})
		}},
		{order.TypeLineComped, func() (Result, error) {
			return s.CompLine(ctx, m, id, CompLine{LineID: 4, ReasonCode: "regular", AuthorisedBy: 7})
		}},
		{order.TypeDiscountApplied, func() (Result, error) {
			return s.ApplyDiscount(ctx, m, id, ApplyDiscount{Scope: order.ScopeOrder, Kind: order.DiscountAmount, Amount: &ten, Reason: "loyalty"})
		}},
		{order.TypeNoteAdded, func() (Result, error) {
			return s.AddNote(ctx, m, id, AddNote{Scope: order.ScopeLine, LineID: 2, Text: "crisp"})
		}},
		{order.TypeCourseFired, func() (Result, error) { return s.FireCourse(ctx, m, id, FireCourse{Course: 1}) }},
		{order.TypePaymentTendered, func() (Result, error) {
			return s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCard, Amount: inr(5000)})
		}}, // 11
		{order.TypePaymentVoided, func() (Result, error) {
			return s.VoidPayment(ctx, m, id, VoidPayment{PaymentID: 11, Reason: "wrong card", AuthorisedBy: 7})
		}},
		{order.TypePaymentTendered, func() (Result, error) {
			return s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: inr(30000)})
		}}, // 13: more than the bill, which is refused
	}
	for i, st := range steps[:len(steps)-1] {
		res, err := st.run()
		if err != nil {
			t.Fatalf("step %d (%s): %v", i, st.want, err)
		}
		if res.Events[0].Type != st.want {
			t.Fatalf("step %d appended %s, want %s", i, res.Events[0].Type, st.want)
		}
	}
	if _, err := steps[len(steps)-1].run(); !errors.Is(err, order.ErrOverTender) {
		t.Fatalf("over-tender: err = %v", err)
	}

	state := must(t)(s.Order(ctx, 1, id)).State
	due := state.Totals.BalanceDue
	must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: due}))
	closed := must(t)(s.Close(ctx, m, id))
	pay := closed.State.Tenders[len(closed.State.Tenders)-1].ID
	// A credit note and its refund on the closed bill (ADR-083), then a
	// reopen, which corrects payments and never the sale.
	req := order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}
	preview, err := s.PlanCredit(ctx, 1, id, req)
	if err != nil {
		t.Fatal(err)
	}
	credited := must(t)(s.Credit(ctx, m, id, IssueCredit{
		Credit: req, Reason: "pastry was stale", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: pay, Amount: preview.Total}},
	}))
	if len(credited.State.Credits) != 1 || len(credited.State.Refunds) != 1 || credited.State.Status != order.StatusClosed {
		t.Errorf("credit: %+v, refunds %+v, %s", credited.State.Credits, credited.State.Refunds, credited.State.Status)
	}
	reopened := must(t)(s.Reopen(ctx, m, id, Reopen{Reason: "card reversed", AuthorisedBy: 7}))
	if reopened.State.Status != order.StatusReopened || !reopened.State.Invoiced {
		t.Errorf("reopen %s, invoiced %v", reopened.State.Status, reopened.State.Invoiced)
	}
}

func TestVenueConfig(t *testing.T) {
	base := gen.Venue{ID: 1, Currency: "INR", Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4}
	with := func(profile, reg string) gen.Venue {
		v := base
		v.TaxProfile, v.TaxRegistrationID = profile, reg
		return v
	}
	us := with("sales_tax_us", "")
	us.Currency = "USD"

	tests := []struct {
		name     string
		row      gen.Venue
		settings map[string]string
		wantErr  bool
		check    func(t *testing.T, v venue)
	}{
		{"untaxed", with("none", ""), nil, false, func(t *testing.T, v venue) {
			if v.Taxes.Profile != nil || v.Series.Code != "INV" || v.Series.MaxLength != 0 {
				t.Errorf("taxes %+v, series %+v", v.Taxes, v.Series)
			}
		}},
		{"gst, inclusive by default", with("gst_in", "29ABCDE1234F1Z5"), map[string]string{SettingTaxStateCode: "29"}, false, func(t *testing.T, v venue) {
			if _, ok := v.Taxes.Profile.(tax.GST); !ok || v.Taxes.Venue.Pricing != tax.Inclusive || v.Taxes.Venue.GSTIN == "" || v.Series.MaxLength != 16 {
				t.Errorf("taxes %+v, series max %d", v.Taxes, v.Series.MaxLength)
			}
		}},
		{"gst with no state", with("gst_in", "29ABCDE1234F1Z5"), nil, true, nil},
		{"gst with no GSTIN", with("gst_in", ""), map[string]string{SettingTaxStateCode: "29"}, true, nil},
		{"vat", with("vat_uk", "GB123456789"), nil, false, func(t *testing.T, v venue) {
			if v.Taxes.Venue.VATNumber != "GB123456789" {
				t.Errorf("VAT number %q", v.Taxes.Venue.VATNumber)
			}
		}},
		{"vat with no number", with("vat_uk", ""), nil, true, nil},
		{"us, exclusive by default", us, map[string]string{SettingTaxJurisdiction: `[{"name":"CA","rate":7250}]`}, false, func(t *testing.T, v venue) {
			if v.Taxes.Venue.Pricing != tax.Exclusive || len(v.Taxes.Venue.Jurisdiction) != 1 || v.Taxes.Venue.Currency != "USD" {
				t.Errorf("taxes %+v", v.Taxes.Venue)
			}
		}},
		{"us with no rates", us, nil, true, nil},
		{"a pricing that is neither", with("none", ""), map[string]string{SettingTaxPricing: "sometimes"}, true, nil},
		{"an unknown profile", with("gst_uae", ""), nil, true, nil},
		{"a timezone that does not exist", func() gen.Venue { v := with("none", ""); v.Timezone = "Mars/Olympus"; return v }(), nil, true, nil},
		{"a template that cannot number", with("none", ""), map[string]string{SettingInvoiceTemplate: "{nonsense}"}, true, nil},
		{"a series of the venue's own", with("none", ""), map[string]string{SettingInvoiceSeries: "DIN", SettingInvoicePrefix: ""}, false, func(t *testing.T, v venue) {
			fy, err := v.Series.FY.YearOf(now, v.Location)
			if err != nil {
				t.Fatal(err)
			}
			no, err := v.Series.Number(fy, 7)
			if err != nil || v.Series.Code != "DIN" || no != "/26-27/000007" {
				t.Errorf("series %+v numbers %q (%v)", v.Series, no, err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := venueFrom(tt.row, tt.settings)
			if tt.wantErr {
				if !errors.Is(err, ErrVenueConfig) {
					t.Errorf("err = %v, want ErrVenueConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("venueFrom: %v", err)
			}
			tt.check(t, v)
		})
	}
}

func TestNewAndMeta(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("New with no database succeeded")
	}
	s, _, _ := newService(t)
	ctx := context.Background()
	for _, m := range []Meta{{}, {VenueID: 1, ActorID: -1}, {VenueID: 1, DeviceID: -1}} {
		if _, err := s.AddLine(ctx, m, 1, coffee(1)); !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("meta %+v: err = %v", m, err)
		}
	}
	if _, err := s.Open(ctx, Meta{VenueID: 9}, OpenOrder{Channel: order.ChannelCounter}); !errors.Is(err, ErrVenueConfig) {
		t.Errorf("open at a venue that does not exist: err = %v", err)
	}
}

// TestSendRoutesEachStationItsOwn: Send fires the first course and gives the
// bar and the kitchen one ticket each with only their lines; a later course
// waits for its fire; a second Send has nothing (ADR-089).
func TestSendRoutesEachStationItsOwn(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	starter, main, drink := dosa(1), dosa(1), coffee(1)
	starter.Course, main.Name, main.Course, drink.Course = 1, "Thali", 2, 1
	must(t)(s.AddLine(ctx, m, id, starter)) // 2, kitchen, course 1
	must(t)(s.AddLine(ctx, m, id, main))    // 3, kitchen, course 2
	must(t)(s.AddLine(ctx, m, id, drink))   // 4, bar, course 1

	sent := must(t)(s.Send(ctx, m, id))
	var got []string
	for _, e := range sent.Events {
		switch pl := e.Payload.(type) {
		case order.CourseFired:
			got = append(got, fmt.Sprintf("fire %d", pl.Course))
		case order.TicketSent:
			got = append(got, fmt.Sprintf("%s %v", pl.Station, pl.LineIDs))
		}
	}
	if want := []string{"fire 1", "kitchen [2]", "bar [4]"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Send appended %v, want %v", got, want)
	}
	if sent.State.Lines[1].Sent {
		t.Error("the main went before its course was fired")
	}
	if _, err := s.Send(ctx, m, id); !errors.Is(err, ErrNothingToSend) {
		t.Errorf("a second Send: %v", err)
	}
}
