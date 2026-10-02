package order

import (
	"fmt"
	"slices"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// CreditRequest is what a credit note should take off the bill: whole units
// of lines, or an amount. Exactly one of the two.
type CreditRequest struct {
	Lines  []CreditItem  `json:"lines,omitempty"`
	Amount *money.Amount `json:"amount,omitempty"`
}

// CreditItem is some units of one line: "one of the two coffees".
type CreditItem struct {
	LineID int64 `json:"line_id"`
	Qty    int64 `json:"qty"`
}

// CreditPlan is a credit note's figures, before it has a number: what the
// planner decided, and what the till shows before anyone approves it.
type CreditPlan struct {
	Lines      []CreditLine
	Taxable    money.Amount
	Tax        money.Amount
	Total      money.Amount
	Components []tax.ComponentTotal
}

// PlanCredit works out a credit note against the bill's invoice.
//
// By units, a line's share is its Due times the units over its quantity,
// rounded once, half to even, and so is its tax; the units that finish a line
// take exactly what is left of it, so a line credited in parts comes to its
// Due and its tax to the paisa.
//
// By amount, the amount is spread over the lines by what each has left to
// credit (money.Allocate), and each line's share carries tax in proportion
// to the tax it has left. A share that finishes a line takes exactly what is
// left.
//
// Either way a line's tax is split over its components (CGST and SGST, say)
// in proportion to what is left of each, so the note's rate-wise summary
// reads like the invoice's.
func PlanCredit(s State, req CreditRequest) (CreditPlan, error) {
	if !s.Invoiced || len(s.Invoices) == 0 {
		return CreditPlan{}, ErrNotInvoiced
	}
	var (
		lines []CreditLine
		err   error
	)
	switch {
	case len(req.Lines) > 0 && req.Amount != nil:
		return CreditPlan{}, invalid("credit by lines or by an amount, not both")
	case len(req.Lines) > 0:
		lines, err = creditUnits(s, req.Lines)
	case req.Amount != nil:
		lines, err = creditAmount(s, *req.Amount)
	default:
		return CreditPlan{}, invalid("a credit note credits nothing")
	}
	if err != nil {
		return CreditPlan{}, err
	}
	return summarise(s, lines), nil
}

// left is what is still creditable on a line: units, value, tax, and tax by
// component, in the line's component order.
type left struct {
	line       Line
	qty        int64
	amount     money.Amount
	tax        money.Amount
	components []tax.ComponentAmount
}

func leftOn(s State, id int64) (left, error) {
	i := slices.IndexFunc(s.Lines, func(l Line) bool { return l.ID == id })
	if i < 0 {
		return left{}, fmt.Errorf("%w: %d", ErrLineNotFound, id)
	}
	l := s.Lines[i]
	if l.Status != LineStatusActive {
		return left{}, fmt.Errorf("%w: line %d is %s", ErrLineNotActive, id, l.Status)
	}
	done := s.CreditedFor(id)
	out := left{
		line: l, qty: l.Qty - done.Qty,
		amount: money.Amount{Minor: l.Due.Minor - done.Amount.Minor, Currency: s.Currency},
		tax:    money.Amount{Minor: l.Tax.Minor - done.Tax.Minor, Currency: s.Currency},
	}
	for _, lt := range s.TaxDetail.Lines {
		if lt.LineID != id {
			continue
		}
		for _, c := range lt.Components {
			rest := c.Amount.Minor - done.Components[c.Name].Minor
			out.components = append(out.components, tax.ComponentAmount{Name: c.Name, Rate: c.Rate, Amount: money.Amount{Minor: rest, Currency: s.Currency}})
		}
	}
	return out, nil
}

func creditUnits(s State, items []CreditItem) ([]CreditLine, error) {
	var out []CreditLine
	seen := map[int64]bool{}
	for _, it := range items {
		if seen[it.LineID] {
			return nil, invalid("line %d is asked for twice", it.LineID)
		}
		seen[it.LineID] = true
		l, err := leftOn(s, it.LineID)
		if err != nil {
			return nil, err
		}
		if it.Qty < 1 {
			return nil, invalid("line %d: credit at least one", it.LineID)
		}
		if it.Qty > l.qty {
			return nil, fmt.Errorf("%w: line %d has %d left to credit, asked for %d", ErrOverCredit, it.LineID, l.qty, it.Qty)
		}
		amount, taxPart := l.amount, l.tax
		if it.Qty < l.qty {
			if amount, err = l.line.Due.Scale(it.Qty, l.line.Qty); err != nil {
				return nil, err
			}
			if taxPart, err = l.line.Tax.Scale(it.Qty, l.line.Qty); err != nil {
				return nil, err
			}
			amount.Minor, taxPart.Minor = min(amount.Minor, l.amount.Minor), min(taxPart.Minor, l.tax.Minor)
		}
		cl, err := creditLine(l, it.Qty, amount, taxPart)
		if err != nil {
			return nil, err
		}
		out = append(out, cl)
	}
	return out, nil
}

func creditAmount(s State, amount money.Amount) ([]CreditLine, error) {
	if amount.Currency != s.Currency {
		return nil, fmt.Errorf("%w: a credit in %s on a bill in %s", money.ErrCurrencyMismatch, amount.Currency, s.Currency)
	}
	if amount.Minor <= 0 {
		return nil, invalid("a credit of %d", amount.Minor)
	}
	var (
		rest    []left
		weights []int64
		total   int64
	)
	for _, l := range s.Lines {
		if l.Status != LineStatusActive {
			continue
		}
		lf, err := leftOn(s, l.ID)
		if err != nil {
			return nil, err
		}
		if lf.amount.Minor > 0 {
			rest = append(rest, lf)
			weights = append(weights, lf.amount.Minor)
			total += lf.amount.Minor
		}
	}
	if amount.Minor > total {
		return nil, fmt.Errorf("%w: the bill has %d left to credit, asked for %d", ErrOverCredit, total, amount.Minor)
	}
	shares, err := money.Allocate(amount, weights)
	if err != nil {
		return nil, err
	}
	var out []CreditLine
	for i, share := range shares {
		if share.Minor == 0 {
			continue
		}
		l := rest[i]
		taxPart := l.tax
		if share.Minor < l.amount.Minor {
			if taxPart, err = l.tax.Scale(share.Minor, l.amount.Minor); err != nil {
				return nil, err
			}
		}
		cl, err := creditLine(l, 0, share, taxPart)
		if err != nil {
			return nil, err
		}
		out = append(out, cl)
	}
	return out, nil
}

// creditLine builds one line of the note, splitting its tax over the line's
// components by what is left of each.
func creditLine(l left, qty int64, amount, taxPart money.Amount) (CreditLine, error) {
	cl := CreditLine{
		LineID: l.line.ID, Name: l.line.Name, Qty: qty, Amount: amount, Tax: taxPart,
		Taxable: money.Amount{Minor: amount.Minor - taxPart.Minor, Currency: amount.Currency},
	}
	if len(l.components) == 0 || taxPart.Minor == 0 {
		return cl, nil
	}
	weights := make([]int64, len(l.components))
	for i, c := range l.components {
		weights[i] = c.Amount.Minor
	}
	parts := make([]money.Amount, len(l.components))
	if taxPart == l.tax {
		for i, c := range l.components {
			parts[i] = c.Amount
		}
	} else {
		var err error
		if parts, err = money.Allocate(taxPart, weights); err != nil {
			return CreditLine{}, err
		}
	}
	for i, c := range l.components {
		cl.Components = append(cl.Components, tax.ComponentAmount{Name: c.Name, Rate: c.Rate, Amount: parts[i]})
	}
	return cl, nil
}

// summarise adds the lines up, and sums the tax by component and rate the
// way an invoice's summary does.
func summarise(s State, lines []CreditLine) CreditPlan {
	z := money.Amount{Minor: 0, Currency: s.Currency}
	p := CreditPlan{Lines: lines, Taxable: z, Tax: z, Total: z}
	type key struct {
		name string
		rate tax.Rate
	}
	index := map[key]int{}
	for _, l := range lines {
		p.Taxable.Minor += l.Taxable.Minor
		p.Tax.Minor += l.Tax.Minor
		p.Total.Minor += l.Amount.Minor
		for _, c := range l.Components {
			k := key{c.Name, c.Rate}
			i, ok := index[k]
			if !ok {
				i = len(p.Components)
				index[k] = i
				p.Components = append(p.Components, tax.ComponentTotal{Name: c.Name, Rate: c.Rate, Taxable: z, Amount: z})
			}
			p.Components[i].Taxable.Minor += l.Taxable.Minor
			p.Components[i].Amount.Minor += c.Amount.Minor
		}
	}
	return p
}
