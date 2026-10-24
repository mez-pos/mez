package tax

// The codes a venue's tax profile is stored under (venues.tax_profile), and
// what each is. They are the settings' words and the order's (ADR-374): an
// order opened under one carries the code, and is projected under that
// profile for the rest of its life.
const (
	CodeNone  = "none"
	CodeGST   = "gst_in"
	CodeVAT   = "vat_uk"
	CodeSales = "sales_tax_us"
)

// ProfileByCode is the regime for a stored profile code: nil and true for
// none, false for a code this build does not know.
func ProfileByCode(code string) (TaxProfile, bool) {
	switch code {
	case CodeNone:
		return nil, true
	case CodeGST:
		return GST{}, true
	case CodeVAT:
		return VAT{}, true
	case CodeSales:
		return USSalesTax{}, true
	}
	return nil, false
}
