package order

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
)

// stream numbers payloads from seq 1, a minute apart, as staff member 11 on
// device 2. Tests that care about the actor build events by hand.
func stream(payloads ...Payload) []Event {
	events := make([]Event, len(payloads))
	for i, p := range payloads {
		events[i] = NewEvent(int64(i+1), testAt.Add(time.Duration(i)*time.Minute), 11, 2, p)
	}
	return events
}

// untaxed is a venue with no tax profile: Totals.Tax stays zero and Total is
// the Subtotal. Most tests use it so their figures stay about the bill.
var untaxed = Taxes{}

func mustProject(t *testing.T, payloads ...Payload) State {
	t.Helper()
	s, err := Project(stream(payloads...), untaxed)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	return s
}

func opened() OrderOpened {
	return OrderOpened{Channel: ChannelCounter, Currency: "INR", Covers: 1, OpenedBy: 11}
}

// item is a line on the bar with no modifiers, in course 1.
func item(name string, qty, unitMinor int64) LineAdded {
	return LineAdded{ItemID: 1, Name: name, Qty: qty, UnitPrice: inr(unitMinor), Course: 1, Station: "bar"}
}

func coffee(qty int64) LineAdded { return item("Flat white", qty, 22000) }

func pay(method PaymentMethod, minor int64) PaymentTendered {
	return PaymentTendered{Method: method, Amount: inr(minor), Tip: inr(0), TenderedBy: 11}
}

func pct(scope Scope, lineID int64, bp int32) DiscountApplied {
	return DiscountApplied{Scope: scope, LineID: lineID, Kind: DiscountPercent, BasisPoints: bp, Reason: "staff"}
}

func off(scope Scope, lineID, minor int64) DiscountApplied {
	a := inr(minor)
	return DiscountApplied{Scope: scope, LineID: lineID, Kind: DiscountAmount, Amount: &a, Reason: "goodwill"}
}

func void(lineID int64) LineVoided {
	return LineVoided{LineID: lineID, ReasonCode: "wrong_item", AuthorisedBy: 3}
}

func invoiceNo(n string) InvoiceIssued {
	return InvoiceIssued{InvoiceNo: n, Series: "NF", TaxSnapshot: []byte(`{}`)}
}

var closed = OrderClosed{ClosedBy: 11}

func TestProjectRejectsIllegalSequences(t *testing.T) {
	usd := money.Amount{Minor: 100, Currency: "USD"}
	oneCoffee := []Payload{opened(), coffee(1)}                                   // line 2, 22000
	paidClosed := []Payload{opened(), coffee(1), pay(PaymentCash, 22000), closed} // tender 3

	with := func(prefix []Payload, more ...Payload) []Payload {
		return append(append([]Payload{}, prefix...), more...)
	}

	tests := []struct {
		name     string
		payloads []Payload
		wantErr  error
	}{
		{"first event is not an open", []Payload{coffee(1)}, ErrNotOpened},
		{"opened twice", with(oneCoffee, opened()), ErrAlreadyOpened},
		{"unknown channel", []Payload{OrderOpened{Channel: "drive_thru", Currency: "INR"}}, ErrInvalidPayload},
		{"bad currency", []Payload{OrderOpened{Channel: ChannelCounter, Currency: "inr"}}, ErrInvalidPayload},
		{"negative covers", []Payload{OrderOpened{Channel: ChannelCounter, Currency: "INR", Covers: -1}}, ErrInvalidPayload},

		{"zero quantity", with(oneCoffee, coffee(0)), ErrInvalidPayload},
		{"no name snapshot", with(oneCoffee, item("  ", 1, 100)), ErrInvalidPayload},
		{"negative course", with(oneCoffee, LineAdded{Name: "x", Qty: 1, UnitPrice: inr(1), Course: -1}), ErrInvalidPayload},
		{"price in another currency", with(oneCoffee, LineAdded{Name: "x", Qty: 1, UnitPrice: usd}), ErrInvalidPayload},
		{"negative price", with(oneCoffee, item("x", 1, -1)), ErrInvalidPayload},
		{"negative modifier", with(oneCoffee, LineAdded{Name: "x", Qty: 1, UnitPrice: inr(1),
			Modifiers: []Modifier{{Name: "no cheese", Price: inr(-500)}}}), ErrInvalidPayload},

		{"qty change on a missing line", with(oneCoffee, LineQtyChanged{LineID: 9, From: 1, To: 2}), ErrLineNotFound},
		{"qty change from a stale value", with(oneCoffee, LineQtyChanged{LineID: 2, From: 3, To: 4}), ErrQtyMismatch},
		{"qty change to zero", with(oneCoffee, LineQtyChanged{LineID: 2, From: 1, To: 0}), ErrInvalidPayload},
		{"qty change to the same", with(oneCoffee, LineQtyChanged{LineID: 2, From: 1, To: 1}), ErrInvalidPayload},
		{"qty change on a voided line", with(oneCoffee, void(2), LineQtyChanged{LineID: 2, From: 1, To: 2}), ErrLineNotActive},
		{"modify a missing line", with(oneCoffee, oat(9)), ErrLineNotFound},
		{"modify from stale modifiers", with(oneCoffee, LineModified{LineID: 2, From: []Modifier{oatMilk}, To: nil}), ErrModifiersMismatch},
		{"modify to the same", with(oneCoffee, LineModified{LineID: 2, From: nil, To: []Modifier{}}), ErrInvalidPayload},
		{"modify to a negative price", with(oneCoffee, LineModified{LineID: 2, To: []Modifier{{ModifierID: 1, Name: "x", Price: inr(-1)}}}), ErrInvalidPayload},
		{"modify to another currency", with(oneCoffee, LineModified{LineID: 2, To: []Modifier{{ModifierID: 1, Name: "x", Price: usd}}}), ErrInvalidPayload},
		{"modify to a nameless modifier", with(oneCoffee, LineModified{LineID: 2, To: []Modifier{{ModifierID: 1, Price: inr(0)}}}), ErrInvalidPayload},
		{"modify a voided line", with(oneCoffee, void(2), oat(2)), ErrLineNotActive},
		{"qty change on a sent line", with(oneCoffee, TicketSent{Station: "bar", LineIDs: []int64{2}}, LineQtyChanged{LineID: 2, From: 1, To: 2}), ErrLineInKitchen},
		{"qty change on a fired course", with(oneCoffee, CourseFired{Course: 1}, LineQtyChanged{LineID: 2, From: 1, To: 2}), ErrLineInKitchen},
		{"modify a sent line", with(oneCoffee, TicketSent{Station: "bar", LineIDs: []int64{2}}, oat(2)), ErrLineInKitchen},
		{"modify a line whose course is fired", with(oneCoffee, CourseFired{Course: 1}, oat(2)), ErrLineInKitchen},
		{"modify after close", with(paidClosed, oat(2)), ErrOrderClosed},

		{"void with no authoriser", with(oneCoffee, LineVoided{LineID: 2, ReasonCode: "x"}), ErrInvalidPayload},
		{"void with no reason", with(oneCoffee, LineVoided{LineID: 2, AuthorisedBy: 3}), ErrInvalidPayload},
		{"void twice", with(oneCoffee, void(2), void(2)), ErrLineNotActive},
		{"comp a voided line", with(oneCoffee, void(2), LineComped{LineID: 2, ReasonCode: "x", AuthorisedBy: 3}), ErrLineNotActive},
		{"comp a missing line", with(oneCoffee, LineComped{LineID: 7, ReasonCode: "x", AuthorisedBy: 3}), ErrLineNotFound},

		{"discount of zero percent", with(oneCoffee, pct(ScopeLine, 2, 0)), ErrInvalidPayload},
		{"discount over 100 percent", with(oneCoffee, pct(ScopeLine, 2, 10001)), ErrInvalidPayload},
		{"percent discount with an amount", with(oneCoffee, DiscountApplied{Scope: ScopeOrder, Kind: DiscountPercent,
			BasisPoints: 100, Amount: &usd, Reason: "x"}), ErrInvalidPayload},
		{"amount discount with no amount", with(oneCoffee, DiscountApplied{Scope: ScopeOrder, Kind: DiscountAmount, Reason: "x"}), ErrInvalidPayload},
		{"amount discount of zero", with(oneCoffee, off(ScopeOrder, 0, 0)), ErrInvalidPayload},
		{"amount discount in another currency", with(oneCoffee, DiscountApplied{Scope: ScopeOrder, Kind: DiscountAmount,
			Amount: &usd, Reason: "x"}), ErrInvalidPayload},
		{"unknown discount kind", with(oneCoffee, DiscountApplied{Scope: ScopeOrder, Kind: "bogo", Reason: "x"}), ErrInvalidPayload},
		{"unknown scope", with(oneCoffee, DiscountApplied{Scope: "table", Kind: DiscountPercent, BasisPoints: 1, Reason: "x"}), ErrInvalidPayload},
		{"order discount naming a line", with(oneCoffee, pct(ScopeOrder, 2, 1000)), ErrInvalidPayload},
		{"discount with no reason", with(oneCoffee, DiscountApplied{Scope: ScopeOrder, Kind: DiscountPercent, BasisPoints: 1}), ErrInvalidPayload},
		{"discount on a voided line", with(oneCoffee, void(2), pct(ScopeLine, 2, 1000)), ErrLineNotActive},
		{"line discount bigger than the line", with(oneCoffee, off(ScopeLine, 2, 22001)), ErrDiscountExceedsValue},
		{"line discounts adding past the line", with(oneCoffee, pct(ScopeLine, 2, 6000), pct(ScopeLine, 2, 5000)), ErrDiscountExceedsValue},
		{"order discount bigger than the order", with(oneCoffee, off(ScopeLine, 2, 2000), off(ScopeOrder, 0, 20001)), ErrDiscountExceedsValue},
		{"qty cut below an amount discount", with([]Payload{opened(), coffee(2)}, off(ScopeLine, 2, 30000),
			LineQtyChanged{LineID: 2, From: 2, To: 1}), ErrDiscountExceedsValue},

		{"empty note", with(oneCoffee, NoteAdded{Scope: ScopeOrder, Text: " "}), ErrInvalidPayload},
		{"note on a missing line", with(oneCoffee, NoteAdded{Scope: ScopeLine, LineID: 5, Text: "x"}), ErrLineNotFound},

		{"course zero fired", with(oneCoffee, CourseFired{Course: 0}), ErrInvalidPayload},
		{"course fired twice", with(oneCoffee, CourseFired{Course: 1}, CourseFired{Course: 1}), ErrCourseAlreadyFired},

		{"ticket with no station", with(oneCoffee, TicketSent{LineIDs: []int64{2}}), ErrInvalidPayload},
		{"ticket with no lines", with(oneCoffee, TicketSent{Station: "bar"}), ErrInvalidPayload},
		{"ticket to the wrong station", with(oneCoffee, TicketSent{Station: "kitchen", LineIDs: []int64{2}}), ErrInvalidPayload},
		{"ticket for a voided line", with(oneCoffee, void(2), TicketSent{Station: "bar", LineIDs: []int64{2}}), ErrLineNotActive},
		{"ticket for a missing line", with(oneCoffee, TicketSent{Station: "bar", LineIDs: []int64{2, 8}}), ErrLineNotFound},

		{"unknown payment method", with(oneCoffee, pay("cheque", 100)), ErrInvalidPayload},
		{"zero tender", with(oneCoffee, pay(PaymentCash, 0)), ErrInvalidPayload},
		{"tender in another currency", with(oneCoffee, PaymentTendered{Method: PaymentCash, Amount: usd, Tip: inr(0)}), ErrInvalidPayload},
		{"negative tip", with(oneCoffee, PaymentTendered{Method: PaymentCard, Amount: inr(100), Tip: inr(-1)}), ErrInvalidPayload},
		{"tip with no currency", with(oneCoffee, PaymentTendered{Method: PaymentCard, Amount: inr(100)}), ErrInvalidPayload},
		{"tender over the balance", with(oneCoffee, pay(PaymentCash, 22001)), ErrOverTender},
		{"tender on an empty order", []Payload{opened(), pay(PaymentCash, 1)}, ErrOverTender},
		{"tender after settling in full", with(oneCoffee, pay(PaymentCash, 22000), pay(PaymentCash, 1)), ErrOverTender},

		{"void a missing payment", with(oneCoffee, PaymentVoided{PaymentID: 2, Reason: "x", AuthorisedBy: 3}), ErrPaymentNotFound},
		{"void a payment twice", with(oneCoffee, pay(PaymentCard, 22000), PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3},
			PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3}), ErrPaymentNotCaptured},
		{"void a payment with no authoriser", with(oneCoffee, pay(PaymentCard, 22000), PaymentVoided{PaymentID: 3, Reason: "x"}), ErrInvalidPayload},
		{"void a partly refunded payment", with(oneCoffee, pay(PaymentCard, 22000), void(2),
			RefundIssued{Amount: inr(100), AgainstPaymentID: 3, Reason: "x"},
			PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3}), ErrPaymentNotCaptured},

		{"invoice with no number", with(oneCoffee, pay(PaymentCash, 22000), InvoiceIssued{Series: "NF"}), ErrInvalidPayload},
		{"invoice with a balance due", with(oneCoffee, invoiceNo("NF/1")), ErrBalanceDue},
		{"invoice twice", with(oneCoffee, pay(PaymentCash, 22000), invoiceNo("NF/1"), invoiceNo("NF/2")), ErrAlreadyInvoiced},

		{"refund a missing payment", with(oneCoffee, RefundIssued{Amount: inr(1), AgainstPaymentID: 2, Reason: "x"}), ErrPaymentNotFound},
		{"refund a voided payment", with(oneCoffee, pay(PaymentCard, 22000), PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3},
			void(2), RefundIssued{Amount: inr(1), AgainstPaymentID: 3, Reason: "x"}), ErrPaymentNotCaptured},
		{"refund with nothing owed", with(oneCoffee, pay(PaymentCard, 22000), RefundIssued{Amount: inr(1), AgainstPaymentID: 3, Reason: "x"}), ErrOverRefund},
		{"refund more than owed", with(oneCoffee, coffee(1), pay(PaymentCard, 44000), void(3),
			RefundIssued{Amount: inr(22001), AgainstPaymentID: 4, Reason: "x"}), ErrOverRefund},
		{"refund more than the payment", with([]Payload{opened(), coffee(2)}, pay(PaymentCash, 22000), pay(PaymentCard, 22000), void(2),
			RefundIssued{Amount: inr(44000), AgainstPaymentID: 3, Reason: "x"}), ErrOverRefund},
		{"refund of zero", with(oneCoffee, pay(PaymentCard, 22000), void(2), RefundIssued{Amount: inr(0), AgainstPaymentID: 3, Reason: "x"}), ErrInvalidPayload},
		{"refund with no reason", with(oneCoffee, pay(PaymentCard, 22000), void(2), RefundIssued{Amount: inr(1), AgainstPaymentID: 3}), ErrInvalidPayload},

		{"close with a balance due", with(oneCoffee, pay(PaymentCash, 21999), closed), ErrBalanceDue},
		{"close owing the guest", with(oneCoffee, pay(PaymentCash, 22000), void(2), closed), ErrBalanceDue},
		{"add a line when closed", with(paidClosed, coffee(1)), ErrOrderClosed},
		{"refund when closed", with(paidClosed, RefundIssued{Amount: inr(1), AgainstPaymentID: 3, Reason: "x"}), ErrOrderClosed},
		{"close twice", with(paidClosed, closed), ErrOrderClosed},
		{"reopen an open order", with(oneCoffee, OrderReopened{Reason: "x", AuthorisedBy: 3}), ErrOrderNotClosed},
		{"reopen with no authoriser", with(paidClosed, OrderReopened{Reason: "x"}), ErrInvalidPayload},
		{"reopen with no reason", with(paidClosed, OrderReopened{AuthorisedBy: 3}), ErrInvalidPayload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := stream(tt.payloads...)
			s, err := Project(events, untaxed)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			// Every case fails at its last event, so every event before it
			// is shown to be legal.
			var pe *ProjectionError
			if !errors.As(err, &pe) {
				t.Fatalf("err is %T, want *ProjectionError", err)
			}
			last := events[len(events)-1]
			if pe.Seq != last.Seq || pe.Type != last.Type {
				t.Errorf("failed at %s seq %d, want the last event, %s seq %d", pe.Type, pe.Seq, last.Type, last.Seq)
			}
			if !reflect.DeepEqual(s, State{}) {
				t.Errorf("a failed projection returned a partial state")
			}
		})
	}
}

func TestProjectRejectsABrokenEnvelope(t *testing.T) {
	good := stream(opened(), coffee(1), coffee(1))

	gap := append([]Event{}, good...)
	gap[2].Seq = 4

	repeat := append([]Event{}, good...)
	repeat[2].Seq = 2

	mismatch := append([]Event{}, good...)
	mismatch[2].Type = TypeLineVoided

	noPayload := append([]Event{}, good...)
	noPayload[2].Payload = nil

	tests := []struct {
		name    string
		events  []Event
		wantErr error
	}{
		{"no events", nil, ErrNotOpened},
		{"a gap in seq", gap, ErrSeqGap},
		{"a repeated seq", repeat, ErrSeqGap},
		{"type disagrees with payload", mismatch, ErrInvalidPayload},
		{"no payload", noPayload, ErrInvalidPayload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(tt.events, untaxed); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestProjectionErrorNamesTheEvent(t *testing.T) {
	_, err := Project(stream(opened(), coffee(1), void(9)), untaxed)

	var pe *ProjectionError
	if !errors.As(err, &pe) {
		t.Fatalf("err is %T, want *ProjectionError", err)
	}
	if pe.Seq != 3 || pe.Type != TypeLineVoided {
		t.Errorf("error names %s seq %d, want line.voided seq 3", pe.Type, pe.Seq)
	}
	for _, want := range []string{"line.voided", "seq 3", "no such line"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err.Error(), want)
		}
	}
}

func TestOpenedRecordsTheEnvelope(t *testing.T) {
	s := mustProject(t, OrderOpened{Channel: ChannelQRTable, Currency: "INR", TableID: 4, Covers: 3, OpenedBy: 11})

	if s.Channel != ChannelQRTable || s.TableID != 4 || s.Covers != 3 || s.OpenedBy != 11 || s.Currency != "INR" {
		t.Errorf("opened state = %+v", s)
	}
	if !s.OpenedAt.Equal(testAt) || s.Status != StatusOpen || s.LastSeq != 1 {
		t.Errorf("OpenedAt %v, Status %q, LastSeq %d", s.OpenedAt, s.Status, s.LastSeq)
	}
	if s.Totals.Total != inr(0) || s.Totals.BalanceDue != inr(0) {
		t.Errorf("an empty order totals %+v, want zero in INR", s.Totals)
	}
}

func TestLinePricesIncludeModifiersPerUnit(t *testing.T) {
	s := mustProject(t, opened(), LineAdded{
		ItemID: 42, Name: "Cold brew", Qty: 3, UnitPrice: inr(20000), Station: "bar",
		Modifiers: []Modifier{{ModifierID: 1, Name: "Oat milk", Price: inr(4000)}, {ModifierID: 2, Name: "No ice", Price: inr(0)}},
	})

	l := s.Lines[0]
	if l.ID != 2 || l.Gross != inr(72000) || l.Net != inr(72000) || l.Status != LineStatusActive {
		t.Errorf("line = %+v, want ID 2 gross and net 72000, active", l)
	}
	if s.Totals.Gross != inr(72000) || s.Totals.BalanceDue != inr(72000) {
		t.Errorf("totals = %+v", s.Totals)
	}
}

var oatMilk = Modifier{ModifierID: 4, Name: "Oat", Price: inr(5000)}

// oat swaps a plain line to oat milk.
func oat(lineID int64) LineModified {
	return LineModified{LineID: lineID, From: nil, To: []Modifier{oatMilk}}
}

// TestModifyingALineRepricesIt: the new modifiers are the line's, per unit,
// and the totals follow; a second change starts from the first.
func TestModifyingALineRepricesIt(t *testing.T) {
	s := mustProject(t, opened(), coffee(2), oat(2))
	l := s.Lines[0]
	if len(l.Modifiers) != 1 || l.Modifiers[0] != oatMilk {
		t.Fatalf("modifiers = %+v", l.Modifiers)
	}
	if l.Gross != inr(54000) || s.Totals.Gross != inr(54000) {
		t.Errorf("gross %v, totals %v; want 2 x (22000 + 5000) = 54000", l.Gross, s.Totals.Gross)
	}

	dairy := Modifier{ModifierID: 3, Name: "Dairy", Price: inr(0)}
	back := mustProject(t, opened(), coffee(2), oat(2), LineModified{LineID: 2, From: []Modifier{oatMilk}, To: []Modifier{dairy}})
	if back.Lines[0].Gross != inr(44000) || back.Lines[0].Modifiers[0] != dairy {
		t.Errorf("after the second change: %+v", back.Lines[0])
	}
}

// TestModifyingKeepsTheLinesDiscounts: a percentage discount follows the new
// price; an amount discount larger than the new value is refused.
func TestModifyingKeepsTheLinesDiscounts(t *testing.T) {
	s := mustProject(t, opened(), coffee(1), pct(ScopeLine, 2, 1000), oat(2))
	if s.Lines[0].Discount != inr(2700) {
		t.Errorf("10%% of 27000 = %v, want 2700", s.Lines[0].Discount)
	}

	withOat := LineAdded{Name: "Flat white", Qty: 1, UnitPrice: inr(22000), Modifiers: []Modifier{oatMilk}, Course: 1, Station: "bar"}
	_, err := Project(stream(opened(), withOat, off(ScopeLine, 2, 25000),
		LineModified{LineID: 2, From: []Modifier{oatMilk}, To: nil}), untaxed)
	if !errors.Is(err, ErrDiscountExceedsValue) {
		t.Errorf("err = %v, want ErrDiscountExceedsValue", err)
	}
}

func TestNotesAreKeptOnAnyLineAndAfterClose(t *testing.T) {
	s := mustProject(t, opened(), coffee(1), void(2),
		NoteAdded{Scope: ScopeLine, LineID: 2, Text: "sent back: too salty"},
		closed,
		NoteAdded{Scope: ScopeOrder, Text: "regular, likes the window table"},
	)

	if len(s.Notes) != 2 || s.Notes[0].LineID != 2 || s.Notes[1].Seq != 6 {
		t.Errorf("notes = %+v", s.Notes)
	}
	if s.Status != StatusClosed {
		t.Errorf("a note reopened the order: status %q", s.Status)
	}
}

func TestStatusFollowsTheOrder(t *testing.T) {
	steps := []struct {
		event Payload
		want  Status
	}{
		{opened(), StatusOpen},
		{coffee(1), StatusOpen},
		{pay(PaymentCard, 22000), StatusSettling},
		{PaymentVoided{PaymentID: 3, Reason: "declined", AuthorisedBy: 3}, StatusOpen},
		{pay(PaymentCash, 22000), StatusSettling},
		{closed, StatusClosed},
		{OrderReopened{Reason: "wrong table", AuthorisedBy: 3}, StatusReopened},
		{NoteAdded{Scope: ScopeOrder, Text: "moved to table 6"}, StatusReopened},
		{closed, StatusClosed},
	}

	var payloads []Payload
	for i, st := range steps {
		payloads = append(payloads, st.event)
		s := mustProject(t, payloads...)
		if s.Status != st.want {
			t.Errorf("after step %d (%s), status = %q, want %q", i+1, st.event.EventType(), s.Status, st.want)
		}
	}

	s := mustProject(t, payloads...)
	if s.Reopens != 1 || s.ClosedAt.IsZero() || s.ClosedBy != 11 {
		t.Errorf("closed again: reopens %d, closed at %v by %d", s.Reopens, s.ClosedAt, s.ClosedBy)
	}
	reopened := mustProject(t, payloads[:7]...)
	if !reopened.ClosedAt.IsZero() || reopened.ClosedBy != 0 {
		t.Errorf("a reopened order still carries its close: at %v by %d", reopened.ClosedAt, reopened.ClosedBy)
	}
}

func TestProjectIsDeterministicAndDoesNotAliasEvents(t *testing.T) {
	events := stream(opened(),
		LineAdded{Name: "Cold brew", Qty: 1, UnitPrice: inr(20000), Station: "bar",
			Modifiers: []Modifier{{Name: "Oat milk", Price: inr(4000)}}},
		off(ScopeOrder, 0, 1000),
		TicketSent{Station: "bar", LineIDs: []int64{2}},
	)

	first, err := Project(events, untaxed)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	second, err := Project(events, untaxed)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("the same events projected to two different states")
	}

	// Scribble on everything the state could share with the events.
	first.Lines[0].Modifiers[0].Price = inr(1)
	first.Tickets[0].LineIDs[0] = 99
	first.Discounts[0].Amount.Minor = 1

	again, err := Project(events, untaxed)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !reflect.DeepEqual(again, second) {
		t.Error("changing a projected state changed the events it came from")
	}
}

func TestDiscountOnALineLaterVoidedFallsAway(t *testing.T) {
	s := mustProject(t, opened(),
		coffee(1),               // 2
		coffee(1),               // 3
		pct(ScopeLine, 3, 5000), // half off line 3
		void(3),
	)

	if d := s.Discounts[0]; !d.Value.IsZero() {
		t.Errorf("discount on a voided line still takes off %d", d.Value.Minor)
	}
	if s.Totals.LineDiscounts != inr(0) || s.Totals.Total != inr(22000) || s.Totals.Voided != inr(22000) {
		t.Errorf("totals = %+v", s.Totals)
	}
}

func TestAFullRefundEndsThePayment(t *testing.T) {
	base := []Payload{opened(),
		coffee(1),               // 2
		pay(PaymentCard, 22000), // 3
		void(2),
		RefundIssued{Amount: inr(22000), AgainstPaymentID: 3, Reason: "walked out before it was made"},
	}

	s := mustProject(t, append(base, closed)...)
	if tn := s.Tenders[0]; tn.Status != PaymentStatusRefunded || tn.Refunded != inr(22000) {
		t.Errorf("tender = %+v, want refunded in full", tn)
	}
	checkFigures(t, s.Totals, figures{paid: 22000, refunded: 22000, voided: 22000})
	if s.Status != StatusClosed {
		t.Errorf("status = %q", s.Status)
	}

	_, err := Project(stream(append(base, PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3})...), untaxed)
	if !errors.Is(err, ErrPaymentNotCaptured) {
		t.Errorf("voiding a refunded payment: err = %v, want ErrPaymentNotCaptured", err)
	}
}

func TestOverflowIsAnErrorNotAWrappedTotal(t *testing.T) {
	huge := int64(math.MaxInt64 / 2)
	tests := []struct {
		name     string
		payloads []Payload
	}{
		{"quantity times price", []Payload{opened(), item("x", 3, huge)}},
		{"price plus modifiers", []Payload{opened(), LineAdded{Name: "x", Qty: 1, UnitPrice: inr(huge),
			Modifiers: []Modifier{{Name: "a", Price: inr(huge)}, {Name: "b", Price: inr(huge)}}}}},
		{"lines summed", []Payload{opened(), item("x", 1, huge), item("y", 1, huge), item("z", 1, huge)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(stream(tt.payloads...), untaxed); !errors.Is(err, money.ErrOverflow) {
				t.Errorf("err = %v, want money.ErrOverflow", err)
			}
		})
	}
}

// foreignPayload is a Payload from outside the known set, which any package
// can write because Payload is an exported interface.
type foreignPayload struct{}

func (foreignPayload) EventType() Type { return "loyalty.points_earned" }

func TestProjectRejectsAPayloadItDoesNotKnow(t *testing.T) {
	events := append(stream(opened()), NewEvent(2, testAt, 11, 2, foreignPayload{}))
	if _, err := Project(events, untaxed); !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("err = %v, want ErrInvalidPayload", err)
	}
}

func TestACompedLineCanStillGoToTheKitchen(t *testing.T) {
	s := mustProject(t, opened(),
		item("Birthday brownie", 1, 16000), // 2
		LineComped{LineID: 2, ReasonCode: "birthday", AuthorisedBy: 3},
		TicketSent{Station: "bar", LineIDs: []int64{2}},
	)
	if l := s.Lines[0]; !l.Sent || l.Status != LineStatusComped {
		t.Errorf("line = %+v, want comped and sent", l)
	}
}
