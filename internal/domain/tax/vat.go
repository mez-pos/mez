package tax

import (
	"fmt"
	"strings"
)

// VAT is value added tax as the UK and EU charge it: one rate per item,
// usually displayed inclusive, and the venue's VAT number on every receipt.
// A zero-rated or exempt line carries no VAT.
type VAT struct{}

// Name implements TaxProfile.
func (VAT) Name() string { return "VAT" }

// Compute implements TaxProfile.
func (v VAT) Compute(lines []Line, ctx VenueContext) (TaxResult, error) {
	if strings.TrimSpace(ctx.VATNumber) == "" {
		return TaxResult{}, fmt.Errorf("%w: no VAT number", ErrInvalidContext)
	}
	return compute(v.Name(), lines, ctx, func(l Line) ([]Component, error) {
		if err := l.Rate.Validate(); err != nil {
			return nil, err
		}
		if l.Rate == 0 {
			return nil, nil
		}
		return []Component{{Name: "VAT", Rate: l.Rate}}, nil
	})
}

// InvoiceFields implements TaxProfile: what a VAT receipt must print.
func (VAT) InvoiceFields() []Field {
	return []Field{
		{Key: "vat_number", Label: "VAT No.", Scope: ScopeInvoice},
		{Key: "invoice_no", Label: "Receipt No.", Scope: ScopeInvoice},
		{Key: "invoice_date", Label: "Date", Scope: ScopeInvoice},
		{Key: "rate", Label: "VAT rate", Scope: ScopeLine},
		{Key: "net", Label: "Net", Scope: ScopeInvoice},
		{Key: "vat", Label: "VAT", Scope: ScopeInvoice},
		{Key: "gross", Label: "Total", Scope: ScopeInvoice},
	}
}

// RegisterColumns implements TaxProfile.
func (VAT) RegisterColumns() []Column {
	return []Column{
		{Key: "invoice_no", Label: "Receipt No."},
		{Key: "invoice_date", Label: "Date"},
		{Key: "rate", Label: "VAT rate"},
		{Key: "net", Label: "Net"},
		{Key: "vat", Label: "VAT"},
		{Key: "gross", Label: "Gross"},
	}
}
