package order

import (
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
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
		ID:           e.Seq,
		Method:       pl.Method,
		Amount:       pl.Amount,
		Tip:          pl.Tip,
		Reference:    pl.Reference,
		TenderedBy:   pl.TenderedBy,
		AuthorisedBy: pl.AuthorisedBy,
		Status:       PaymentStatusCaptured,
		Refunded:     p.zero(),
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
	if !t.Refunded.IsZero() {
		return fmt.Errorf("%w: payment %d has %d refunded", ErrPaymentNotCaptured, t.ID, t.Refunded.Minor)
	}
	status, err := TransitionPayment(t.Status, PaymentStatusVoided)
	if err != nil {
		return fmt.Errorf("%w: payment %d: %w", ErrPaymentNotCaptured, t.ID, err)
	}
	if err := needReason(pl.Reason, pl.AuthorisedBy); err != nil {
		return err
	}
	t.Status = status
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
	// A price snapshot must be of this bill: an invoice that prints one total
	// and settles another is worse than one with no snapshot.
	if pl.Prices != nil && pl.Prices.Total != p.s.Totals.Total {
		return invalid("the invoice's prices total %d, the bill is %d", pl.Prices.Total.Minor, p.s.Totals.Total.Minor)
	}
	p.invoiced = true
	p.s.Invoiced = true
	p.s.Invoices = append(p.s.Invoices, Invoice{Seq: e.Seq, InvoiceNo: pl.InvoiceNo, Series: pl.Series})
	return nil
}

// refund gives money back against a tender. It is only possible while the
// venue owes the guest, which is a negative balance: a void or comp before
// the invoice, or a credit note after it (ADR-083). A closed order takes a
// refund only after a credit note on it has made the balance negative.
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
		if t.Status, err = TransitionPayment(t.Status, PaymentStatusRefunded); err != nil {
			return err
		}
	}
	p.s.Refunds = append(p.s.Refunds, Refund{
		Seq: e.Seq, Amount: pl.Amount, AgainstPaymentID: pl.AgainstPaymentID, Reason: pl.Reason, AuthorisedBy: pl.AuthorisedBy,
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
	// The invoice stands (ADR-083): a reopen corrects payments, and what was
	// sold is corrected by a credit note, never by a second invoice.
	p.closed = false
	p.reopened = true
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

// reprint records a copy of an issued invoice. It changes no money and
// allocates no number; it only counts, so a Z-report can show reprints.
func (p *projector) reprint(pl InvoiceReprinted) error {
	if err := needReason(pl.Reason, pl.AuthorisedBy); err != nil {
		return err
	}
	for i := range p.s.Invoices {
		if p.s.Invoices[i].InvoiceNo == pl.InvoiceNo {
			p.s.Invoices[i].Reprints++
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrInvoiceNotFound, pl.InvoiceNo)
}
