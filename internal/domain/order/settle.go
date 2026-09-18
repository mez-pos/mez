package order

import (
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
)

// The handlers that move money or settle the order, and the checks the
// handlers share.

func (p *projector) tender(e Event, pl PaymentTendered) error {
	if !validMethod(pl.Method) {
		return invalid("unknown payment method %q", pl.Method)
	}
	if err := p.checkPositive("tender", pl.Amount); err != nil {
		return err
	}
	if err := p.checkPrice("tip", pl.Tip); err != nil {
		return err
	}
	if due := p.s.Totals.BalanceDue; pl.Amount.Minor > due.Minor {
		return fmt.Errorf("%w: tendered %d, balance due %d", ErrOverTender, pl.Amount.Minor, due.Minor)
	}
	p.s.Tenders = append(p.s.Tenders, Tender{
		ID:         e.Seq,
		Method:     pl.Method,
		Amount:     pl.Amount,
		Tip:        pl.Tip,
		Reference:  pl.Reference,
		TenderedBy: pl.TenderedBy,
		Status:     PaymentStatusCaptured,
		Refunded:   p.zero(),
	})
	return nil
}

// voidPayment cancels a tender outright. A tender with any refund against it
// cannot be voided: the refund already moved money, and voiding on top of it
// would count that money back twice.
func (p *projector) voidPayment(pl PaymentVoided) error {
	t, err := p.findTender(pl.PaymentID)
	if err != nil {
		return err
	}
	if t.Status != PaymentStatusCaptured || !t.Refunded.IsZero() {
		return fmt.Errorf("%w: payment %d is %s with %d refunded", ErrPaymentNotCaptured, t.ID, t.Status, t.Refunded.Minor)
	}
	if err := needReason(pl.Reason, pl.AuthorisedBy); err != nil {
		return err
	}
	t.Status = PaymentStatusVoided
	t.VoidReason = pl.Reason
	return nil
}

// invoice records an issued invoice. The bill must be settled to the paisa,
// and there is one invoice per settlement: a second needs a reopen first.
func (p *projector) invoice(e Event, pl InvoiceIssued) error {
	if pl.InvoiceNo == "" || pl.Series == "" {
		return invalid("an invoice needs a number and a series")
	}
	if p.invoiced {
		return ErrAlreadyInvoiced
	}
	if due := p.s.Totals.BalanceDue; !due.IsZero() {
		return fmt.Errorf("%w: %d outstanding", ErrBalanceDue, due.Minor)
	}
	p.invoiced = true
	p.s.Invoices = append(p.s.Invoices, Invoice{Seq: e.Seq, InvoiceNo: pl.InvoiceNo, Series: pl.Series})
	return nil
}

// refund gives money back against a tender. It is only possible while the
// venue owes the guest, which is a negative balance: a void or comp after
// payment creates one. A closed order owes nothing, so refunding one means
// reopening it and recording why the guest is owed. See ADR-046.
func (p *projector) refund(e Event, pl RefundIssued) error {
	t, err := p.findTender(pl.AgainstPaymentID)
	if err != nil {
		return err
	}
	if t.Status != PaymentStatusCaptured {
		return fmt.Errorf("%w: payment %d is %s", ErrPaymentNotCaptured, t.ID, t.Status)
	}
	if err := p.checkPositive("refund", pl.Amount); err != nil {
		return err
	}
	if strings.TrimSpace(pl.Reason) == "" {
		return invalid("refund has no reason")
	}
	if left := t.Amount.Minor - t.Refunded.Minor; pl.Amount.Minor > left {
		return fmt.Errorf("%w: payment %d has %d left to refund, asked for %d", ErrOverRefund, t.ID, left, pl.Amount.Minor)
	}
	if owed := -p.s.Totals.BalanceDue.Minor; pl.Amount.Minor > owed {
		return fmt.Errorf("%w: the order owes the guest %d, asked for %d", ErrOverRefund, max(owed, 0), pl.Amount.Minor)
	}
	refunded, err := t.Refunded.Add(pl.Amount)
	if err != nil {
		return err
	}
	t.Refunded = refunded
	if refunded == t.Amount {
		t.Status = PaymentStatusRefunded
	}
	p.s.Refunds = append(p.s.Refunds, Refund{
		Seq: e.Seq, Amount: pl.Amount, AgainstPaymentID: pl.AgainstPaymentID, Reason: pl.Reason,
	})
	return nil
}

func (p *projector) close(e Event, pl OrderClosed) error {
	if due := p.s.Totals.BalanceDue; !due.IsZero() {
		return fmt.Errorf("%w: %d outstanding", ErrBalanceDue, due.Minor)
	}
	p.closed = true
	p.reopened = false
	p.s.ClosedBy = pl.ClosedBy
	p.s.ClosedAt = e.At.UTC()
	return nil
}

func (p *projector) reopen(pl OrderReopened) error {
	if !p.closed {
		return ErrOrderNotClosed
	}
	if err := needReason(pl.Reason, pl.AuthorisedBy); err != nil {
		return err
	}
	p.closed = false
	p.reopened = true
	p.invoiced = false
	p.s.Reopens++
	p.s.ClosedBy = 0
	p.s.ClosedAt = time.Time{}
	return nil
}

// validMethod reports whether m is a payment method the venue can take.
func validMethod(m PaymentMethod) bool {
	switch m {
	case PaymentCash, PaymentCard, PaymentUPI, PaymentOther:
		return true
	}
	return false
}

// zero is nothing, in the order's currency.
func (p *projector) zero() money.Amount {
	return money.Amount{Minor: 0, Currency: p.s.Currency}
}

// checkPrice refuses money in another currency, or below zero. A price, a
// modifier or a tip may be zero.
func (p *projector) checkPrice(what string, a money.Amount) error {
	if a.Currency != p.s.Currency {
		return invalid("%s is in %q, the order is in %q", what, a.Currency, p.s.Currency)
	}
	if a.Minor < 0 {
		return invalid("%s %d is negative", what, a.Minor)
	}
	return nil
}

// checkPositive is checkPrice for amounts that must be more than zero:
// tenders, refunds and amount discounts.
func (p *projector) checkPositive(what string, a money.Amount) error {
	if err := p.checkPrice(what, a); err != nil {
		return err
	}
	if a.Minor == 0 {
		return invalid("%s is zero", what)
	}
	return nil
}

// checkScope checks the scope and line of a discount or note. A line-scoped
// discount needs an active line; a note may go on any line, voided ones
// included, because "sent back: too salty" belongs on the voided line.
func (p *projector) checkScope(scope Scope, lineID int64, needActive bool) error {
	switch scope {
	case ScopeOrder:
		if lineID != 0 {
			return invalid("an order-scoped event names line %d", lineID)
		}
		return nil
	case ScopeLine:
		if needActive {
			_, err := p.activeLine(lineID)
			return err
		}
		_, err := p.line(lineID)
		return err
	default:
		return invalid("unknown scope %q", scope)
	}
}

// line finds a line by ID, which is the seq of its line.added.
func (p *projector) line(id int64) (*Line, error) {
	for i := range p.s.Lines {
		if p.s.Lines[i].ID == id {
			return &p.s.Lines[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %d", ErrLineNotFound, id)
}

// activeLine finds a line that is neither voided nor comped.
func (p *projector) activeLine(id int64) (*Line, error) {
	l, err := p.line(id)
	if err != nil {
		return nil, err
	}
	if l.Status != LineStatusActive {
		return nil, fmt.Errorf("%w: line %d is %s", ErrLineNotActive, id, l.Status)
	}
	return l, nil
}

// findTender finds a tender by ID, which is the seq of its payment.tendered.
func (p *projector) findTender(id int64) (*Tender, error) {
	for i := range p.s.Tenders {
		if p.s.Tenders[i].ID == id {
			return &p.s.Tenders[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %d", ErrPaymentNotFound, id)
}
