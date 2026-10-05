package zreport

import (
	"cmp"
	"slices"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// methods is the order payments are reported in.
var methods = []order.PaymentMethod{order.PaymentCash, order.PaymentCard, order.PaymentUPI, order.PaymentOther}

// builder is Build's working state. Money is added through add and sub,
// which keep the first error, so a mismatched currency anywhere fails the
// report instead of being skipped.
type builder struct {
	in   Input
	zero money.Amount
	err  error
	r    Report

	categories, items, channels, services, staff map[string]*Group
	removed                                      map[int64]*StaffTally
	tips                                         map[int64]*StaffTally
	payments                                     map[order.PaymentMethod]*Payment
	taxes                                        map[taxKey]*TaxLine
	invoices                                     []string
}

type taxKey struct {
	name string
	rate tax.Rate
}

func newBuilder(in Input, zero money.Amount) *builder {
	z := zero
	return &builder{
		in:   in,
		zero: zero,
		r: Report{
			Currency:    in.Currency,
			Sales:       Sales{Gross: z, Discounts: z, Net: z, Taxable: z, Tax: z, Total: z, NetOfCredits: z},
			Discounts:   Tally{Amount: z},
			Comps:       Tally{Amount: z},
			Voids:       Tally{Amount: z},
			CreditNotes: Tally{Amount: z},
			Refunds:     Tally{Amount: z},
			Tips:        z,
		},
		categories: map[string]*Group{},
		items:      map[string]*Group{},
		channels:   map[string]*Group{},
		services:   map[string]*Group{},
		staff:      map[string]*Group{},
		removed:    map[int64]*StaffTally{},
		tips:       map[int64]*StaffTally{},
		payments:   map[order.PaymentMethod]*Payment{},
		taxes:      map[taxKey]*TaxLine{},
	}
}

func (b *builder) add(to *money.Amount, x money.Amount) {
	if b.err != nil {
		return
	}
	*to, b.err = to.Add(x)
}

func (b *builder) sub(to *money.Amount, x money.Amount) {
	if b.err != nil {
		return
	}
	*to, b.err = to.Sub(x)
}

func (b *builder) tally(t *Tally, x money.Amount) error {
	t.Count++
	b.add(&t.Amount, x)
	return b.err
}

func (b *builder) group(in map[string]*Group, key, name string) *Group {
	g, ok := in[key]
	if !ok {
		z := b.zero
		g = &Group{Key: key, Name: name, Gross: z, Discounts: z, Net: z, Tax: z, Total: z}
		in[key] = g
	}
	return g
}

func (b *builder) payment(m order.PaymentMethod) *Payment {
	p, ok := b.payments[m]
	if !ok {
		z := b.zero
		p = &Payment{Method: m, Taken: z, Voided: z, Refunded: z, Net: z}
		b.payments[m] = p
	}
	return p
}

func (b *builder) taxLine(name string, rate tax.Rate) *TaxLine {
	k := taxKey{name, rate}
	t, ok := b.taxes[k]
	if !ok {
		t = &TaxLine{Name: name, Rate: rate, Taxable: b.zero, Amount: b.zero}
		b.taxes[k] = t
	}
	return t
}

// finish works out the nets, puts every list in its order and balances
// payments against sales.
func (b *builder) finish() (Report, error) {
	r := b.r
	b.sub(&r.Sales.Net, r.Sales.Discounts)
	b.add(&r.Sales.Net, r.Sales.Gross)
	b.add(&r.Sales.NetOfCredits, r.Sales.Total)
	b.sub(&r.Sales.NetOfCredits, r.CreditNotes.Amount)

	r.ByCategory = b.groups(b.categories)
	r.ByItem = b.groups(b.items)
	r.ByChannel = b.groups(b.channels)
	r.ByService = b.groups(b.services)
	r.ByStaff = b.groups(b.staff)

	r.Removed = []StaffTally{}
	for _, t := range b.removed {
		r.Removed = append(r.Removed, *t)
	}
	slices.SortFunc(r.Removed, func(x, y StaffTally) int { return cmp.Compare(x.StaffID, y.StaffID) })

	r.TipsBy = []StaffTally{}
	for _, t := range b.tips {
		// Someone whose every tipped payment was voided took no tips.
		if t.Count > 0 || t.Amount.Minor != 0 {
			r.TipsBy = append(r.TipsBy, *t)
		}
	}
	slices.SortFunc(r.TipsBy, func(x, y StaffTally) int {
		return cmp.Or(cmp.Compare(y.Amount.Minor, x.Amount.Minor), cmp.Compare(x.StaffID, y.StaffID))
	})

	r.Taxes = []TaxLine{}
	for _, t := range b.taxes {
		r.Taxes = append(r.Taxes, *t)
	}
	slices.SortFunc(r.Taxes, func(x, y TaxLine) int {
		return cmp.Or(cmp.Compare(x.Rate, y.Rate), cmp.Compare(x.Name, y.Name))
	})

	r.Payments = []Payment{}
	r.Balance = Balance{Payments: b.zero, Sales: r.Sales.NetOfCredits, Difference: b.zero}
	for _, m := range methods {
		p, ok := b.payments[m]
		if !ok {
			continue
		}
		b.add(&p.Net, p.Taken)
		b.sub(&p.Net, p.Voided)
		b.sub(&p.Net, p.Refunded)
		b.add(&r.Balance.Payments, p.Net)
		r.Payments = append(r.Payments, *p)
	}
	b.add(&r.Balance.Difference, r.Balance.Payments)
	b.sub(&r.Balance.Difference, r.Balance.Sales)

	r.Invoices = invoiceRun(b.invoices)
	if b.err != nil {
		return Report{}, b.err
	}
	return r, nil
}

// groups is a heading's groups with their nets, largest total first.
func (b *builder) groups(in map[string]*Group) []Group {
	out := make([]Group, 0, len(in))
	for _, g := range in {
		b.add(&g.Net, g.Gross)
		b.sub(&g.Net, g.Discounts)
		out = append(out, *g)
	}
	slices.SortFunc(out, func(x, y Group) int {
		return cmp.Or(cmp.Compare(y.Total.Minor, x.Total.Minor), cmp.Compare(x.Name, y.Name), cmp.Compare(x.Key, y.Key))
	})
	return out
}

// invoiceRun is the count and the first and last numbers. Numbers in one
// series and year are zero-padded, so they sort as they were issued.
func invoiceRun(numbers []string) Invoices {
	if len(numbers) == 0 {
		return Invoices{}
	}
	sorted := slices.Clone(numbers)
	slices.Sort(sorted)
	return Invoices{Count: int64(len(numbers)), First: sorted[0], Last: sorted[len(sorted)-1]}
}
