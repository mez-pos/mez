package order

import (
	"errors"
	"fmt"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// Credit notes (phase 07, Task 6a, ADR-083). When money goes back to a guest
// on a bill that has its invoice, the invoice stands and a credit note says
// what was credited against it: which lines, how many of each, and the tax.
// The refund that pays it out is its own event, after it.

// The credit note refusals.
var (
	// ErrNotInvoiced means a credit note for a bill with no invoice to
	// correct. A bill not yet invoiced is corrected with a void or a comp.
	ErrNotInvoiced = errors.New("the bill has no invoice to credit")

	// ErrOverCredit means a credit for more of a line, or more of the bill,
	// than is left to credit.
	ErrOverCredit = errors.New("credit exceeds what is left to credit")
)

// CreditNoteIssued records a credit note against the bill's invoice. Its
// number is from the venue's credit note series, allocated in the same
// transaction. Everything it credits is snapshotted, so a reprint shows what
// was credited then, whatever the bill says later.
type CreditNoteIssued struct {
	CreditNo         string               `json:"credit_no"`
	Series           string               `json:"series"`
	AgainstInvoiceNo string               `json:"against_invoice_no"`
	Reason           string               `json:"reason"`
	AuthorisedBy     int64                `json:"authorised_by"`
	Lines            []CreditLine         `json:"lines"`
	Taxable          money.Amount         `json:"taxable"`
	Tax              money.Amount         `json:"tax"`
	Total            money.Amount         `json:"total"` // Taxable + Tax: what goes back to the guest
	Components       []tax.ComponentTotal `json:"components,omitempty"`
	Seller           *InvoiceSeller       `json:"seller,omitempty"`
}

// CreditLine is what a credit note takes off one line. Qty is how many of
// the line it credits whole; zero for a credit of an amount, which takes a
// share of the line's value and not any units of it.
type CreditLine struct {
	LineID     int64                 `json:"line_id"`
	Name       string                `json:"name"`
	Qty        int64                 `json:"qty"`
	Taxable    money.Amount          `json:"taxable"`
	Tax        money.Amount          `json:"tax"`
	Amount     money.Amount          `json:"amount"` // Taxable + Tax
	Components []tax.ComponentAmount `json:"components,omitempty"`
}

// Credit is a credit note as issued on the order.
type Credit struct {
	Seq              int64
	CreditNo         string
	Series           string
	AgainstInvoiceNo string
	Reason           string
	AuthorisedBy     int64
	Lines            []CreditLine
	Taxable          money.Amount
	Tax              money.Amount
	Total            money.Amount
}

// Credited is how much of a line has been credited so far, across every
// credit note: units, value, tax, and tax by component name.
type Credited struct {
	Qty        int64
	Amount     money.Amount
	Tax        money.Amount
	Components map[string]money.Amount
}

// CreditedFor sums what the order's credit notes have taken off a line.
func (s State) CreditedFor(lineID int64) Credited {
	z := money.Amount{Minor: 0, Currency: s.Currency}
	c := Credited{Amount: z, Tax: z, Components: map[string]money.Amount{}}
	for _, cr := range s.Credits {
		for _, l := range cr.Lines {
			if l.LineID != lineID {
				continue
			}
			c.Qty += l.Qty
			c.Amount.Minor += l.Amount.Minor
			c.Tax.Minor += l.Tax.Minor
			for _, comp := range l.Components {
				a := c.Components[comp.Name]
				a.Currency = s.Currency
				a.Minor += comp.Amount.Minor
				c.Components[comp.Name] = a
			}
		}
	}
	return c
}

// credit records a credit note. It checks the note against the bill: the
// current invoice, lines that exist and are active, no more of any line
// than is left to credit, and figures that add up. The rounding that made
// them is the planner's (PlanCredit); this holds the limits.
func (p *projector) credit(e Event, pl CreditNoteIssued) error {
	if strings.TrimSpace(pl.CreditNo) == "" || strings.TrimSpace(pl.Series) == "" {
		return invalid("a credit note needs a number and a series")
	}
	if err := needReason(pl.Reason, pl.AuthorisedBy); err != nil {
		return err
	}
	if !p.invoiced || len(p.s.Invoices) == 0 {
		return ErrNotInvoiced
	}
	if current := p.s.Invoices[len(p.s.Invoices)-1].InvoiceNo; pl.AgainstInvoiceNo != current {
		return fmt.Errorf("%w: %q is not this bill's invoice %q", ErrInvoiceNotFound, pl.AgainstInvoiceNo, current)
	}
	if len(pl.Lines) == 0 {
		return invalid("a credit note credits nothing")
	}
	sum := p.zero()
	taxSum := p.zero()
	seen := map[int64]bool{}
	for _, cl := range pl.Lines {
		if seen[cl.LineID] {
			return invalid("line %d is credited twice in one note", cl.LineID)
		}
		seen[cl.LineID] = true
		if err := p.checkCreditLine(cl); err != nil {
			return err
		}
		sum.Minor += cl.Amount.Minor
		taxSum.Minor += cl.Tax.Minor
	}
	if err := p.checkPositive("credit", pl.Total); err != nil {
		return err
	}
	if sum != pl.Total || taxSum != pl.Tax || pl.Taxable.Minor+pl.Tax.Minor != pl.Total.Minor {
		return invalid("the credit note's lines come to %d with tax %d, its totals say %d with tax %d", sum.Minor, taxSum.Minor, pl.Total.Minor, pl.Tax.Minor)
	}
	p.s.Credits = append(p.s.Credits, Credit{
		Seq: e.Seq, CreditNo: pl.CreditNo, Series: pl.Series, AgainstInvoiceNo: pl.AgainstInvoiceNo,
		Reason: pl.Reason, AuthorisedBy: pl.AuthorisedBy, Lines: cloneCreditLines(pl.Lines),
		Taxable: pl.Taxable, Tax: pl.Tax, Total: pl.Total,
	})
	return nil
}

func (p *projector) checkCreditLine(cl CreditLine) error {
	var line *Line
	for i := range p.s.Lines {
		if p.s.Lines[i].ID == cl.LineID {
			line = &p.s.Lines[i]
		}
	}
	if line == nil {
		return fmt.Errorf("%w: %d", ErrLineNotFound, cl.LineID)
	}
	if line.Status != LineStatusActive {
		return fmt.Errorf("%w: line %d is %s", ErrLineNotActive, line.ID, line.Status)
	}
	for _, a := range []money.Amount{cl.Amount, cl.Tax, cl.Taxable} {
		if err := p.checkPrice("credit", a); err != nil {
			return err
		}
	}
	if cl.Amount.Minor == 0 || cl.Qty < 0 || cl.Taxable.Minor+cl.Tax.Minor != cl.Amount.Minor {
		return invalid("line %d: a credit of %d with tax %d and %d units does not add up", cl.LineID, cl.Amount.Minor, cl.Tax.Minor, cl.Qty)
	}
	var compSum int64
	for _, c := range cl.Components {
		compSum += c.Amount.Minor
	}
	if len(cl.Components) > 0 && compSum != cl.Tax.Minor {
		return invalid("line %d: its tax components come to %d, its tax is %d", cl.LineID, compSum, cl.Tax.Minor)
	}
	done := p.s.CreditedFor(cl.LineID)
	switch {
	case done.Qty+cl.Qty > line.Qty:
		return fmt.Errorf("%w: line %d has %d of %d left to credit, asked for %d", ErrOverCredit, line.ID, line.Qty-done.Qty, line.Qty, cl.Qty)
	case done.Amount.Minor+cl.Amount.Minor > line.Due.Minor:
		return fmt.Errorf("%w: line %d has %d left to credit, asked for %d", ErrOverCredit, line.ID, line.Due.Minor-done.Amount.Minor, cl.Amount.Minor)
	case done.Tax.Minor+cl.Tax.Minor > line.Tax.Minor:
		return fmt.Errorf("%w: line %d has tax %d left to credit, asked for %d", ErrOverCredit, line.ID, line.Tax.Minor-done.Tax.Minor, cl.Tax.Minor)
	}
	return nil
}

func cloneCreditLines(lines []CreditLine) []CreditLine {
	out := make([]CreditLine, len(lines))
	for i, l := range lines {
		l.Components = append([]tax.ComponentAmount(nil), l.Components...)
		out[i] = l
	}
	return out
}
