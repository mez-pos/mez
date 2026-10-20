package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
)

// splitBill is the phase's done-when bill: three dosas (378.00 with GST)
// paid three ways, 126.00 each by cash, card and UPI, then closed. The
// payments are seqs 3, 4 and 5.
func splitBill(t *testing.T, s *Service) (int64, order.State) {
	t.Helper()
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, id, dosa(3)))
	for _, method := range []order.PaymentMethod{order.PaymentCash, order.PaymentCard, order.PaymentUPI} {
		must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: method, Amount: inr(12600)}))
	}
	return id, must(t)(s.Close(ctx, m, id)).State
}

// TestPartialRefundOnASplitBill: a partial refund against one of three
// payments is a credit note, numbered in its own series, and the invoice
// stands.
func TestPartialRefundOnASplitBill(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	id, closed := splitBill(t, s)
	if len(closed.Invoices) != 1 || closed.Invoices[0].InvoiceNo != "INV/26-27/000001" {
		t.Fatalf("invoices = %+v", closed.Invoices)
	}
	card := closed.Tenders[1].ID

	// A refund over what the card has left is refused, and takes no number.
	big := inr(20000)
	_, err := s.Credit(ctx, cashier(""), id, IssueCredit{
		Credit: order.CreditRequest{Amount: &big}, Reason: "cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: card, Amount: big}},
	})
	if !errors.Is(err, order.ErrOverRefund) {
		t.Fatalf("over the card: %v", err)
	}
	// Refunds that do not pay out the credit are refused before a number.
	one := order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}
	if _, err := s.Credit(ctx, cashier(""), id, IssueCredit{Credit: one, Reason: "cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: card, Amount: inr(100)}}}); !errors.Is(err, ErrRefundsMismatch) {
		t.Errorf("refunds short of the credit: %v", err)
	}

	preview, err := s.PlanCredit(ctx, 1, id, one)
	if err != nil || preview.Total != inr(12600) || preview.Tax != inr(600) {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	res := must(t)(s.Credit(ctx, cashier("refund-1"), id, IssueCredit{
		Credit: one, Reason: "one dosa was cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: card, Amount: preview.Total}},
	}))
	if len(res.Events) != 2 || res.Events[0].Type != order.TypeCreditNoteIssued || res.Events[1].Type != order.TypeRefundIssued {
		t.Fatalf("events = %+v", res.Events)
	}
	cn := res.Events[0].Payload.(order.CreditNoteIssued)
	if cn.CreditNo != "CN/26-27/000001" || cn.AgainstInvoiceNo != "INV/26-27/000001" || cn.Seller == nil || cn.Seller.Registration == "" {
		t.Errorf("credit note = %+v", cn)
	}
	// The refund says who let it through, as its credit note does: the audit
	// trail reads the refund (ADR-117).
	if rf := res.Events[1].Payload.(order.RefundIssued); rf.AuthorisedBy != 7 || cn.AuthorisedBy != 7 {
		t.Errorf("authorised by: the refund says %d, the credit note %d, want 7", rf.AuthorisedBy, cn.AuthorisedBy)
	}
	st := res.State
	if st.Status != order.StatusClosed || len(st.Invoices) != 1 || st.Totals.Credited != inr(12600) ||
		st.Totals.Refunded != inr(12600) || !st.Totals.BalanceDue.IsZero() || st.Tenders[1].Refunded != inr(12600) {
		t.Errorf("after the refund: %s, %+v, tender %+v", st.Status, st.Totals, st.Tenders[1])
	}
	// A retry of the same command is a replay: no second note, no second number.
	again := must(t)(s.Credit(ctx, cashier("refund-1"), id, IssueCredit{
		Credit: one, Reason: "one dosa was cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: card, Amount: preview.Total}},
	}))
	if !again.Replayed || len(again.State.Credits) != 1 {
		t.Errorf("a retry issued again: replayed %v, credits %d", again.Replayed, len(again.State.Credits))
	}

	notes, err := d.Queries().CreditNotesForVenue(ctx, 1)
	if err != nil || len(notes) != 1 || notes[0].CreditNo != "CN/26-27/000001" || notes[0].TotalMinor != 12600 || notes[0].OrderID != id {
		t.Errorf("credit note register = %+v, %v", notes, err)
	}
	invoices, err := d.Queries().InvoicesForVenue(ctx, 1)
	if err != nil || len(invoices) != 1 {
		t.Errorf("invoice register = %+v, %v", invoices, err)
	}

	// The next credit note, split over the other two payments, is 000002.
	rest := order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 2}}}
	res = must(t)(s.Credit(ctx, cashier(""), id, IssueCredit{
		Credit: rest, Reason: "the table left", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: closed.Tenders[0].ID, Amount: inr(12600)}, {PaymentID: closed.Tenders[2].ID, Amount: inr(12600)}},
	}))
	if cn := res.Events[0].Payload.(order.CreditNoteIssued); cn.CreditNo != "CN/26-27/000002" {
		t.Errorf("second credit note = %s", cn.CreditNo)
	}
	if res.State.Totals.Credited != res.State.Totals.Total || !res.State.Totals.BalanceDue.IsZero() {
		t.Errorf("everything credited: %+v", res.State.Totals)
	}
	if _, err := s.PlanCredit(ctx, 1, id, order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}); !errors.Is(err, order.ErrOverCredit) {
		t.Errorf("crediting past the bill: %v", err)
	}
}

// TestPaymentVoidAfterTheInvoice: a payment voided after the invoice puts
// the balance back to due; the next payment settles it without a number.
func TestPaymentVoidAfterTheInvoice(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, id, coffee(1)))
	paid := must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCard, Amount: inr(4200)}))
	voided := must(t)(s.VoidPayment(ctx, m, id, VoidPayment{PaymentID: paid.State.Tenders[0].ID, Reason: "card declined after", AuthorisedBy: 7}))
	if voided.State.Totals.BalanceDue != inr(4200) {
		t.Fatalf("balance after the void = %d", voided.State.Totals.BalanceDue.Minor)
	}
	again := must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentUPI, Amount: inr(4200)}))
	if len(again.Events) != 1 || len(again.State.Invoices) != 1 {
		t.Errorf("the second payment took a number: %d events, invoices %+v", len(again.Events), again.State.Invoices)
	}
	must(t)(s.Close(ctx, m, id))
	if invoices, _ := d.Queries().InvoicesForVenue(ctx, 1); len(invoices) != 1 {
		t.Errorf("invoice register = %+v", invoices)
	}
}

func TestCreditSeriesConfig(t *testing.T) {
	row := gen.Venue{ID: 1, Currency: "INR", Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4, TaxProfile: "none"}
	v, err := venueFrom(row, map[string]string{})
	if err != nil || v.Credits.Code != "CN" {
		t.Fatalf("default credit series: %+v, %v", v.Credits, err)
	}
	if _, err := venueFrom(row, map[string]string{SettingCreditSeries: "INV"}); !errors.Is(err, ErrVenueConfig) {
		t.Errorf("credit notes in the invoice series: %v", err)
	}
	v, err = venueFrom(row, map[string]string{SettingCreditSeries: "RF", SettingCreditPrefix: "R"})
	if err != nil || v.Credits.Code != "RF" {
		t.Errorf("a named credit series: %+v, %v", v.Credits, err)
	}
}

// TestPastBills: the latest closed bills, and a search by invoice number,
// by its digits, and by credit note number.
func TestPastBills(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	first, _ := splitBill(t, s)
	second, closed := splitBill(t, s)
	open(t, s) // an open ticket is not a past bill

	all, err := s.PastBills(ctx, 1, "")
	if err != nil || len(all) != 2 || all[0].OrderID != second || all[1].OrderID != first {
		t.Fatalf("past bills = %d, %v", len(all), err)
	}
	for _, q := range []string{"INV/26-27/000001", "1", " 000001 "} {
		got, err := s.PastBills(ctx, 1, q)
		if err != nil || len(got) != 1 || got[0].OrderID != first {
			t.Errorf("search %q: %d results, %v", q, len(got), err)
		}
	}
	must(t)(s.Credit(ctx, cashier(""), second, IssueCredit{
		Credit: order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}, Reason: "cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: closed.Tenders[0].ID, Amount: inr(12600)}},
	}))
	if got, _ := s.PastBills(ctx, 1, "CN/26-27/000001"); len(got) != 1 || got[0].OrderID != second {
		t.Errorf("search by credit note: %+v", got)
	}
	if got, _ := s.PastBills(ctx, 1, "INV/26-27/000009"); len(got) != 0 {
		t.Errorf("a number nobody has: %+v", got)
	}
	if got, _ := s.PastBills(ctx, 2, ""); len(got) != 0 {
		t.Errorf("another venue's bills: %+v", got)
	}
}

// TestVoidOnAClosedBill: voiding a payment on a closed bill reopens it in
// the same command; the balance is taken again with no new number.
func TestVoidOnAClosedBill(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id, closed := splitBill(t, s)
	res := must(t)(s.VoidPayment(ctx, cashier(""), id, VoidPayment{PaymentID: closed.Tenders[1].ID, Reason: "card reversed", AuthorisedBy: 7}))
	if len(res.Events) != 2 || res.Events[0].Type != order.TypeOrderReopened || res.Events[1].Type != order.TypePaymentVoided {
		t.Fatalf("events = %+v", res.Events)
	}
	if res.State.Status != order.StatusReopened || res.State.Totals.BalanceDue != inr(12600) {
		t.Fatalf("after the void: %s, balance %d", res.State.Status, res.State.Totals.BalanceDue.Minor)
	}
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(12600)}))
	if len(paid.Events) != 1 || len(paid.State.Invoices) != 1 {
		t.Errorf("the new payment took a number: %+v", paid.Events)
	}
	if res := must(t)(s.Close(ctx, cashier(""), id)); res.State.Status != order.StatusClosed {
		t.Errorf("status = %s", res.State.Status)
	}
}
