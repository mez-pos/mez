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
	// CodeGSTComposition is a venue under the GST composition scheme
	// (ADR-401): registered for GST, it charges no tax and issues a bill of
	// supply that says why.
	CodeGSTComposition = "gst_in_composition"
)

// CompositionDeclaration is what CGST Rule 5(1)(g) has a composition
// taxable person print at the top of every bill of supply (ADR-401).
const CompositionDeclaration = "Composition taxable person, not eligible to collect tax on supplies"

// ProfileByCode is the regime for a stored profile code: nil and true for
// a code that charges no tax (none, and the composition scheme), false for
// a code this build does not know.
func ProfileByCode(code string) (TaxProfile, bool) {
	switch code {
	case CodeNone, CodeGSTComposition:
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

// UnderGST reports whether a profile's documents are held to GST's rules:
// they show a GSTIN, called that, and their numbers are 16 characters at
// most. Regular GST and the composition scheme both are.
func UnderGST(code string) bool { return code == CodeGST || code == CodeGSTComposition }

// ValidGSTIN reports whether s has a GSTIN's shape: 15 capital letters and
// digits, the first two the state's digits.
func ValidGSTIN(s string) bool { return validGSTIN(s) }
