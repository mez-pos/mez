package order

import (
	"errors"
	"fmt"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// invoicedBill is a paid, invoiced, closed GST bill, priced exclusive:
// line 2 is three dosas (360.00 + 18.00 tax), line 3 a filter coffee (40.00
// + 2.00): 420.00 in all, paid 300.00 by card (seq 4) and 120.00 in cash
// (seq 5), invoiced (seq 6) and closed (seq 7).
func invoicedBill() []Payload {
	return []Payload{
		opened(),
		dish("Masala dosa", 3, 12000),
		dish("Filter coffee", 1, 4000),
		pay(PaymentCard, 30000),
		pay(PaymentCash, 12000),
		invoiceNo("NF/26-27/000001"),
		closed,
	}
}

var gst = gstCafe(tax.Exclusive)

// note turns a plan into the event, as the service does.
func note(p CreditPlan, no string) CreditNoteIssued {
	return CreditNoteIssued{
		CreditNo: no, Series: "CN", AgainstInvoiceNo: "NF/26-27/000001", Reason: "cold food", AuthorisedBy: 3,
		Lines: p.Lines, Taxable: p.Taxable, Tax: p.Tax, Total: p.Total, Components: p.Components,
	}
}

func refund(paymentID, minor int64) RefundIssued {
	return RefundIssued{Amount: inr(minor), AgainstPaymentID: paymentID, Reason: "cold food"}
}

func plan(t *testing.T, s State, req CreditRequest) CreditPlan {
	t.Helper()
	p, err := PlanCredit(s, req)
	if err != nil {
		t.Fatalf("PlanCredit: %v", err)
	}
	if p.Taxable.Minor+p.Tax.Minor != p.Total.Minor {
		t.Errorf("taxable %d + tax %d != total %d", p.Taxable.Minor, p.Tax.Minor, p.Total.Minor)
	}
	for _, l := range p.Lines {
		var comps int64
		for _, c := range l.Components {
			comps += c.Amount.Minor
		}
		if comps != l.Tax.Minor {
			t.Errorf("line %d: components %d, tax %d", l.LineID, comps, l.Tax.Minor)
		}
	}
	return p
}

func TestCreditByUnits(t *testing.T) {
	s := projectTaxed(t, gst, invoicedBill()...)
	one := plan(t, s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}}})
	if one.Total != inr(12600) || one.Tax != inr(600) || one.Lines[0].Qty != 1 {
		t.Fatalf("one dosa = %+v", one)
	}
	if len(one.Components) != 2 || one.Components[0].Amount != inr(300) || one.Components[1].Amount != inr(300) || one.Components[0].Taxable != inr(12000) {
		t.Errorf("components = %+v", one.Components)
	}

	// Credited in parts, a line comes to its Due and its tax exactly.
	s2 := projectTaxed(t, gst, append(invoicedBill(), note(one, "CN/26-27/000001"), refund(4, 12600))...)
	rest := plan(t, s2, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 2}}})
	line := s2.Lines[0]
	if one.Total.Minor+rest.Total.Minor != line.Due.Minor || one.Tax.Minor+rest.Tax.Minor != line.Tax.Minor {
		t.Errorf("credited in parts: %d + %d of %d, tax %d + %d of %d",
			one.Total.Minor, rest.Total.Minor, line.Due.Minor, one.Tax.Minor, rest.Tax.Minor, line.Tax.Minor)
	}
	if _, err := PlanCredit(s2, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 3}}}); !errors.Is(err, ErrOverCredit) {
		t.Errorf("three more dosas: %v", err)
	}
}

// TestCreditOddUnits: a line whose Due does not divide by its quantity is
// still credited to the paisa, whatever the order of the parts.
func TestCreditOddUnits(t *testing.T) {
	bill := []Payload{opened(), dish("Idli", 3, 1001), pay(PaymentCash, 3153), invoiceNo("NF/26-27/000001"), closed}
	s := projectTaxed(t, gst, bill...)
	line := s.Lines[0]
	if line.Due.Minor != 3153 {
		t.Fatalf("due %d; the test wants 30.03 + 5%%", line.Due.Minor)
	}
	var credited, taxed int64
	events := bill
	for i := range 3 {
		st := projectTaxed(t, gst, events...)
		p := plan(t, st, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}}})
		credited += p.Total.Minor
		taxed += p.Tax.Minor
		events = append(events, note(p, fmt.Sprintf("CN/26-27/%06d", i+1)), refund(3, p.Total.Minor))
	}
	if credited != line.Due.Minor || taxed != line.Tax.Minor {
		t.Errorf("three idlis one at a time: %d of %d, tax %d of %d", credited, line.Due.Minor, taxed, line.Tax.Minor)
	}
	if final := projectTaxed(t, gst, events...); !final.Totals.BalanceDue.IsZero() || final.Totals.Credited != final.Totals.Total {
		t.Errorf("after crediting it all: %+v", final.Totals)
	}
}

func TestCreditByAmount(t *testing.T) {
	s := projectTaxed(t, gst, invoicedBill()...)
	p := plan(t, s, CreditRequest{Amount: new100()})
	if p.Total != inr(10000) || len(p.Lines) != 2 {
		t.Fatalf("credit of 100.00 = %+v", p)
	}
	// Spread by what each line has left: 378.00 and 42.00.
	if p.Lines[0].Amount != inr(9000) || p.Lines[1].Amount != inr(1000) || p.Lines[0].Qty != 0 {
		t.Errorf("shares = %+v", p.Lines)
	}
	// Tax in proportion: 90.00 of 378.00 carries 4.29 of the dosas' 18.00.
	if p.Lines[0].Tax != inr(429) || p.Lines[1].Tax != inr(48) {
		t.Errorf("tax = %d and %d", p.Lines[0].Tax.Minor, p.Lines[1].Tax.Minor)
	}

	all := inr(42000)
	whole := plan(t, s, CreditRequest{Amount: &all})
	if whole.Total != s.Totals.Total || whole.Tax != s.Totals.Tax {
		t.Errorf("the whole bill as an amount: %+v, bill %+v", whole, s.Totals)
	}
	more := inr(42001)
	if _, err := PlanCredit(s, CreditRequest{Amount: &more}); !errors.Is(err, ErrOverCredit) {
		t.Errorf("more than the bill: %v", err)
	}
}

func new100() *money.Amount { a := inr(10000); return &a }

func TestPlanCreditRefusals(t *testing.T) {
	s := projectTaxed(t, gst, invoicedBill()...)
	unpaid := projectTaxed(t, gst, opened(), dish("Masala dosa", 1, 12000))
	usd := money.Amount{Minor: 100, Currency: "USD"}
	zero := inr(0)
	tests := []struct {
		name string
		s    State
		req  CreditRequest
		want error
	}{
		{"no invoice", unpaid, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}}}, ErrNotInvoiced},
		{"nothing asked", s, CreditRequest{}, ErrInvalidPayload},
		{"both kinds", s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}}, Amount: new100()}, ErrInvalidPayload},
		{"no such line", s, CreditRequest{Lines: []CreditItem{{LineID: 9, Qty: 1}}}, ErrLineNotFound},
		{"zero units", s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 0}}}, ErrInvalidPayload},
		{"a line twice", s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}, {LineID: 2, Qty: 1}}}, ErrInvalidPayload},
		{"four of three", s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 4}}}, ErrOverCredit},
		{"another currency", s, CreditRequest{Amount: &usd}, money.ErrCurrencyMismatch},
		{"zero", s, CreditRequest{Amount: &zero}, ErrInvalidPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PlanCredit(tt.s, tt.req); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestCreditAndRefundAClosedBill: the credit note and its refunds go onto
// the closed bill without a reopen; the invoice stands; the balance nets to
// zero; the refund may be split over the payments.
func TestCreditAndRefundAClosedBill(t *testing.T) {
	s := projectTaxed(t, gst, invoicedBill()...)
	p := plan(t, s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}, {LineID: 3, Qty: 1}}}) // 126.00 + 42.00
	after := projectTaxed(t, gst, append(invoicedBill(), note(p, "CN/26-27/000001"), refund(4, 10000), refund(5, 6800))...)
	if after.Status != StatusClosed || len(after.Invoices) != 1 || !after.Invoiced {
		t.Errorf("status %s, invoices %+v", after.Status, after.Invoices)
	}
	tt := after.Totals
	if tt.Total != inr(42000) || tt.Credited != inr(16800) || tt.Refunded != inr(16800) || !tt.BalanceDue.IsZero() {
		t.Errorf("totals = %+v", tt)
	}
	if len(after.Credits) != 1 || after.Credits[0].CreditNo != "CN/26-27/000001" || after.Credits[0].AgainstInvoiceNo != "NF/26-27/000001" {
		t.Errorf("credits = %+v", after.Credits)
	}
	if c := after.CreditedFor(2); c.Qty != 1 || c.Amount != inr(12600) || c.Components["CGST"] != inr(300) {
		t.Errorf("credited on line 2 = %+v", c)
	}

	// Without a credit note, a closed bill owes nothing to refund.
	if _, err := Project(stream(append(invoicedBill(), refund(4, 100))...), gst); !errors.Is(err, ErrOverRefund) {
		t.Errorf("a refund with no credit: %v", err)
	}
	// A refund of more than the credit.
	if _, err := Project(stream(append(invoicedBill(), note(p, "CN/26-27/000001"), refund(4, 16801))...), gst); !errors.Is(err, ErrOverRefund) {
		t.Errorf("refunding more than the credit: %v", err)
	}
}

func TestCreditNoteRefusals(t *testing.T) {
	s := projectTaxed(t, gst, invoicedBill()...)
	good := note(plan(t, s, CreditRequest{Lines: []CreditItem{{LineID: 2, Qty: 1}}}), "CN/26-27/000001")
	bad := func(edit func(*CreditNoteIssued)) CreditNoteIssued {
		n := good
		n.Lines = cloneCreditLines(good.Lines)
		edit(&n)
		return n
	}
	tests := []struct {
		name string
		n    CreditNoteIssued
		want error
	}{
		{"no number", bad(func(n *CreditNoteIssued) { n.CreditNo = "" }), ErrInvalidPayload},
		{"no reason", bad(func(n *CreditNoteIssued) { n.Reason = "" }), ErrInvalidPayload},
		{"nobody allowed it", bad(func(n *CreditNoteIssued) { n.AuthorisedBy = 0 }), ErrInvalidPayload},
		{"another invoice", bad(func(n *CreditNoteIssued) { n.AgainstInvoiceNo = "NF/26-27/000009" }), ErrInvoiceNotFound},
		{"totals that do not add up", bad(func(n *CreditNoteIssued) { n.Total = inr(99) }), ErrInvalidPayload},
		{"more than the line", bad(func(n *CreditNoteIssued) {
			n.Lines[0].Amount, n.Lines[0].Taxable = inr(40000), inr(40000-600)
			n.Total, n.Taxable = inr(40000), inr(40000-600)
		}), ErrOverCredit},
		{"components that do not add up", bad(func(n *CreditNoteIssued) { n.Lines[0].Components[0].Amount = inr(1) }), ErrInvalidPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(stream(append(invoicedBill(), tt.n)...), gst); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	unpaid := []Payload{opened(), dish("Masala dosa", 1, 12000)}
	if _, err := Project(stream(append(unpaid, good)...), gst); !errors.Is(err, ErrNotInvoiced) {
		t.Errorf("a credit note with no invoice: %v", err)
	}
}

// TestReopenKeepsTheInvoice: a reopen corrects payments, not the sale. A
// payment voided after the invoice returns the balance to due, and the next
// payment settles it with no second number.
func TestReopenKeepsTheInvoice(t *testing.T) {
	reopen := OrderReopened{Reason: "card charged twice", AuthorisedBy: 3}
	voided := PaymentVoided{PaymentID: 4, Reason: "card reversed", AuthorisedBy: 3}
	s := projectTaxed(t, gst, append(invoicedBill(), reopen, voided, pay(PaymentUPI, 30000), closed)...)
	if s.Status != StatusClosed || len(s.Invoices) != 1 || !s.Totals.BalanceDue.IsZero() {
		t.Errorf("after the payment fix: %s, %+v, balance %d", s.Status, s.Invoices, s.Totals.BalanceDue.Minor)
	}
	if _, err := Project(stream(append(invoicedBill(), reopen, invoiceNo("NF/26-27/000002"))...), gst); !errors.Is(err, ErrAlreadyInvoiced) {
		t.Errorf("a second invoice after a reopen: %v", err)
	}
	if _, err := Project(stream(append(invoicedBill(), reopen, dish("Vada", 1, 5000))...), gst); !errors.Is(err, ErrOrderInvoiced) {
		t.Errorf("a new line after a reopen: %v", err)
	}
	// Before the close, a voided payment is settled by the next one.
	open := invoicedBill()[:6]
	s = projectTaxed(t, gst, append(open, voided, pay(PaymentCash, 30000), closed)...)
	if s.Status != StatusClosed || len(s.Invoices) != 1 {
		t.Errorf("void before the close: %s %+v", s.Status, s.Invoices)
	}
}
