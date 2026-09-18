package tax

import (
	"fmt"
	"strings"
)

// USSalesTax is US sales tax: always added at the till, never inclusive, at
// the combined rate of the jurisdiction where the sale happens. Each part of
// that rate (state, county, city, district) is computed and reported
// separately, because each goes to a different authority.
//
// The line's own Rate is ignored: in the US the place sets the rate, not the
// item. An item that is not taxed there (unprepared food in many states) is
// marked Exempt.
type USSalesTax struct{}

// Name implements TaxProfile.
func (USSalesTax) Name() string { return "US sales tax" }

// Compute implements TaxProfile.
func (u USSalesTax) Compute(lines []Line, ctx VenueContext) (TaxResult, error) {
	if ctx.Pricing != Exclusive {
		return TaxResult{}, fmt.Errorf("%w: %q", ErrPricingNotSupported, ctx.Pricing)
	}
	if len(ctx.Jurisdiction) == 0 {
		return TaxResult{}, fmt.Errorf("%w: no jurisdiction rates", ErrInvalidContext)
	}
	for _, c := range ctx.Jurisdiction {
		if strings.TrimSpace(c.Name) == "" {
			return TaxResult{}, fmt.Errorf("%w: a jurisdiction rate has no name", ErrInvalidContext)
		}
		if err := c.Rate.Validate(); err != nil {
			return TaxResult{}, fmt.Errorf("%w: %s: %w", ErrInvalidContext, c.Name, err)
		}
	}
	comps := append([]Component(nil), ctx.Jurisdiction...)
	return compute(u.Name(), lines, ctx, func(Line) ([]Component, error) {
		return comps, nil
	})
}

// InvoiceFields implements TaxProfile: a US receipt shows the subtotal, the
// tax and the total; the breakdown by authority is optional on the receipt
// but kept for the return.
func (USSalesTax) InvoiceFields() []Field {
	return []Field{
		{Key: "invoice_no", Label: "Receipt No.", Scope: ScopeInvoice},
		{Key: "invoice_date", Label: "Date", Scope: ScopeInvoice},
		{Key: "subtotal", Label: "Subtotal", Scope: ScopeInvoice},
		{Key: "tax", Label: "Sales tax", Scope: ScopeInvoice},
		{Key: "total", Label: "Total", Scope: ScopeInvoice},
	}
}

// RegisterColumns implements TaxProfile: one row per authority for the
// sales tax return.
func (USSalesTax) RegisterColumns() []Column {
	return []Column{
		{Key: "invoice_date", Label: "Date"},
		{Key: "jurisdiction", Label: "Jurisdiction"},
		{Key: "rate", Label: "Rate"},
		{Key: "taxable", Label: "Taxable sales"},
		{Key: "exempt", Label: "Exempt sales"},
		{Key: "tax", Label: "Tax collected"},
	}
}
