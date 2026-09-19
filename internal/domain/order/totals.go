package order

import (
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// recompute derives every figure in State from the lines, discounts and
// tenders, then derives the status and checks the move against the order
// machine. It runs after every event, so a figure is never stale and an event
// that breaks an invariant fails at its own seq.
//
// The order of operations is the rule: line gross, then line discounts off
// each line, then order discounts off what is left. Each percentage is
// rounded once, half to even, by money.Mul. Nothing here is a float.
func (p *projector) recompute() error {
	if err := p.lineFigures(); err != nil {
		return err
	}
	t, err := p.orderFigures()
	if err != nil {
		return err
	}
	p.s.Totals = t
	return p.moveTo(p.status())
}

// moveTo sets the derived status, checking the change against the order
// machine. A legal stream never fails here; if one does, the projection and
// the machine disagree, and refusing the event is safer than guessing which
// is right.
func (p *projector) moveTo(next Status) error {
	if p.s.Status != "" && next != p.s.Status {
		if _, err := TransitionOrder(p.s.Status, next); err != nil {
			return err
		}
	}
	p.s.Status = next
	return nil
}

// lineFigures sets Gross, Discount and Net on every line and Value on every
// line discount.
func (p *projector) lineFigures() error {
	cur := p.s.Currency
	for i := range p.s.Lines {
		l := &p.s.Lines[i]
		unit, err := money.Sum(cur, l.UnitPrice)
		for _, m := range l.Modifiers {
			if err == nil {
				unit, err = unit.Add(m.Price)
			}
		}
		if err != nil {
			return fmt.Errorf("pricing line %d: %w", l.ID, err)
		}
		if l.Gross, err = unit.Times(l.Qty); err != nil {
			return fmt.Errorf("pricing line %d: %w", l.ID, err)
		}
		l.Discount, l.Net = p.zero(), p.zero()
	}

	for i := range p.s.Discounts {
		d := &p.s.Discounts[i]
		if d.Scope != ScopeLine {
			continue
		}
		l, err := p.line(d.LineID)
		if err != nil {
			return err
		}
		d.Value = p.zero()
		if l.Status != LineStatusActive {
			continue
		}
		if d.Value, err = discountValue(*d, l.Gross); err != nil {
			return err
		}
		if l.Discount, err = l.Discount.Add(d.Value); err != nil {
			return err
		}
	}

	for i := range p.s.Lines {
		l := &p.s.Lines[i]
		if l.Status != LineStatusActive {
			continue
		}
		if l.Discount.Minor > l.Gross.Minor {
			return fmt.Errorf("%w: line %d is worth %d, discounted %d", ErrDiscountExceedsValue, l.ID, l.Gross.Minor, l.Discount.Minor)
		}
		net, err := l.Gross.Sub(l.Discount)
		if err != nil {
			return err
		}
		l.Net = net
	}
	return nil
}

// orderFigures sums the lines, applies order discounts, and settles the
// balance against the tenders and refunds.
func (p *projector) orderFigures() (Totals, error) {
	var sum summer
	t := Totals{}
	z := p.zero()
	t.Gross, t.LineDiscounts, t.Voided, t.Comped = z, z, z, z
	for _, l := range p.s.Lines {
		switch l.Status {
		case LineStatusActive:
			t.Gross = sum.add(t.Gross, l.Gross)
			t.LineDiscounts = sum.add(t.LineDiscounts, l.Discount)
		case LineStatusVoided:
			t.Voided = sum.add(t.Voided, l.Gross)
		case LineStatusComped:
			t.Comped = sum.add(t.Comped, l.Gross)
		}
	}
	base := sum.sub(t.Gross, t.LineDiscounts)
	if sum.err != nil {
		return Totals{}, sum.err
	}

	orderDiscounts, err := p.orderDiscounts(base)
	if err != nil {
		return Totals{}, err
	}
	t.OrderDiscounts = orderDiscounts
	t.Subtotal = sum.sub(base, t.OrderDiscounts)
	if sum.err != nil {
		return Totals{}, sum.err
	}
	if err := p.taxFigures(&t); err != nil {
		return Totals{}, err
	}

	t.Paid, t.Tips, t.Refunded = z, z, z
	for _, tn := range p.s.Tenders {
		if tn.Status == PaymentStatusVoided {
			continue
		}
		t.Paid = sum.add(t.Paid, tn.Amount)
		t.Tips = sum.add(t.Tips, tn.Tip)
	}
	for _, r := range p.s.Refunds {
		t.Refunded = sum.add(t.Refunded, r.Amount)
	}
	t.NetPaid = sum.sub(t.Paid, t.Refunded)
	t.BalanceDue = sum.sub(t.Total, t.NetPaid)
	return t, sum.err
}

// orderDiscounts values every order discount against base, the order after
// line discounts, and checks they do not take off more than base.
func (p *projector) orderDiscounts(base money.Amount) (money.Amount, error) {
	total := p.zero()
	for i := range p.s.Discounts {
		d := &p.s.Discounts[i]
		if d.Scope != ScopeOrder {
			continue
		}
		v, err := discountValue(*d, base)
		if err != nil {
			return money.Amount{}, err
		}
		d.Value = v
		if total, err = total.Add(v); err != nil {
			return money.Amount{}, err
		}
	}
	if total.Minor > base.Minor {
		return money.Amount{}, fmt.Errorf("%w: order is worth %d, discounted %d", ErrDiscountExceedsValue, base.Minor, total.Minor)
	}
	return total, nil
}

// taxFigures spreads the order discounts over the active lines, in
// proportion to what each is worth, then asks the venue's tax profile for the
// tax on what each line is left with. It sets Taxable, Tax and Total, each
// line's OrderDiscount and Tax, and the TaxDetail snapshot.
func (p *projector) taxFigures(t *Totals) error {
	var active []*Line
	for i := range p.s.Lines {
		l := &p.s.Lines[i]
		l.OrderDiscount, l.Tax = p.zero(), p.zero()
		if l.Status == LineStatusActive {
			active = append(active, l)
		}
	}
	weights := make([]int64, len(active))
	for i, l := range active {
		weights[i] = l.Net.Minor
	}
	shares, err := money.Allocate(t.OrderDiscounts, weights)
	if err != nil {
		return fmt.Errorf("spreading order discounts: %w", err)
	}
	for i, l := range active {
		l.OrderDiscount = shares[i]
	}

	if p.taxes.Profile == nil {
		t.Taxable, t.Tax, t.Total = t.Subtotal, p.zero(), t.Subtotal
		p.s.TaxDetail = tax.TaxResult{}
		return nil
	}
	lines := make([]tax.Line, len(active))
	for i, l := range active {
		amount, err := l.Net.Sub(l.OrderDiscount)
		if err != nil {
			return err
		}
		lines[i] = tax.Line{ID: l.ID, Name: l.Name, HSN: l.HSN, Amount: amount, Rate: l.TaxRate, Exempt: l.TaxExempt}
	}
	res, err := p.taxes.Profile.Compute(lines, p.taxes.Venue)
	if err != nil {
		return fmt.Errorf("computing tax: %w", err)
	}
	for i, l := range active {
		l.Tax = res.Lines[i].Tax
	}
	// The profile sees exactly the subtotal, split by line, so one of these
	// must hold. If neither does, the bill and its tax disagree.
	if res.Gross != t.Subtotal {
		return fmt.Errorf("%w: tax was computed on %d, the subtotal is %d", ErrTaxConfig, res.Gross.Minor, t.Subtotal.Minor)
	}
	t.Taxable, t.Tax, t.Total = res.Taxable, res.Tax, res.Total
	p.s.TaxDetail = res
	return nil
}

// discountValue is what a discount takes off base. Percentages are taken of
// base and rounded half to even; amounts are what they say.
func discountValue(d Discount, base money.Amount) (money.Amount, error) {
	if d.Kind == DiscountPercent {
		return base.Mul(d.BasisPoints)
	}
	return *d.Amount, nil
}

// status derives the order's status. Closed wins; then a reopen, until the
// next close; then any live tender makes it settling.
func (p *projector) status() Status {
	switch {
	case p.closed:
		return StatusClosed
	case p.reopened:
		return StatusReopened
	}
	for _, t := range p.s.Tenders {
		if t.Status == PaymentStatusCaptured {
			return StatusSettling
		}
	}
	return StatusOpen
}

// summer chains money additions and keeps the first error, so a column of
// totals reads as a column and not as a ladder of if err != nil.
type summer struct{ err error }

func (s *summer) add(a, b money.Amount) money.Amount {
	if s.err != nil {
		return a
	}
	r, err := a.Add(b)
	if err != nil {
		s.err = err
		return a
	}
	return r
}

func (s *summer) sub(a, b money.Amount) money.Amount {
	if s.err != nil {
		return a
	}
	r, err := a.Sub(b)
	if err != nil {
		s.err = err
		return a
	}
	return r
}
