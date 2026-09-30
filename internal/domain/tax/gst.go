package tax

import (
	"fmt"
	"slices"
)

// GST is India's Goods and Services Tax.
//
// Within the venue's state a line's rate is split in half between CGST
// (central) and SGST (state), or UTGST in a union territory without its own
// legislature. Across states the whole rate is IGST. Every line needs an HSN
// or SAC code; restaurant service is SAC 996331.
//
// Rates are the item's own, snapshotted on the line, and are not checked
// against a list: the slabs changed in 2025 and will change again, and a
// hardcoded list would refuse a legal sale on the day they do.
type GST struct{}

// unionTerritories are the state codes that charge UTGST instead of SGST:
// Chandigarh, Dadra and Nagar Haveli and Daman and Diu, Lakshadweep, the
// Andaman and Nicobar Islands, and Ladakh. Delhi (07) and Puducherry (34)
// have legislatures and charge SGST.
var unionTerritories = []string{"04", "26", "31", "35", "38"}

// Name implements TaxProfile.
func (GST) Name() string { return "GST (India)" }

// Compute implements TaxProfile.
func (g GST) Compute(lines []Line, ctx VenueContext) (TaxResult, error) {
	if err := checkGSTContext(ctx); err != nil {
		return TaxResult{}, err
	}
	supply := ctx.PlaceOfSupply
	if supply == "" {
		supply = ctx.StateCode
	}
	intraState := supply == ctx.StateCode
	stateTax := "SGST"
	if slices.Contains(unionTerritories, ctx.StateCode) {
		stateTax = "UTGST"
	}

	return compute(g.Name(), lines, ctx, func(l Line) ([]Component, error) {
		if !ValidHSN(l.HSN) {
			return nil, fmt.Errorf("%w: %q", ErrMissingHSN, l.HSN)
		}
		if err := l.Rate.Validate(); err != nil {
			return nil, err
		}
		switch {
		case l.Rate == 0:
			return nil, nil // nil-rated: reported, but no tax
		case !intraState:
			return []Component{{Name: "IGST", Rate: l.Rate}}, nil
		case l.Rate%2 != 0:
			return nil, fmt.Errorf("%w: %s cannot be split evenly into CGST and %s", ErrInvalidRate, l.Rate, stateTax)
		default:
			half := l.Rate / 2
			return []Component{{Name: "CGST", Rate: half}, {Name: stateTax, Rate: half}}, nil
		}
	})
}

// checkGSTContext refuses a venue that cannot issue a GST invoice.
func checkGSTContext(ctx VenueContext) error {
	if !validStateCode(ctx.StateCode) {
		return fmt.Errorf("%w: state code %q", ErrInvalidContext, ctx.StateCode)
	}
	if ctx.PlaceOfSupply != "" && !validStateCode(ctx.PlaceOfSupply) {
		return fmt.Errorf("%w: place of supply %q", ErrInvalidContext, ctx.PlaceOfSupply)
	}
	if !validGSTIN(ctx.GSTIN) {
		return fmt.Errorf("%w: GSTIN %q is not 15 uppercase letters and digits", ErrInvalidContext, ctx.GSTIN)
	}
	if ctx.GSTIN[:2] != ctx.StateCode {
		return fmt.Errorf("%w: GSTIN %q is registered in state %s, the venue is in %s", ErrInvalidContext, ctx.GSTIN, ctx.GSTIN[:2], ctx.StateCode)
	}
	return nil
}

// validStateCode accepts the two-digit GST state codes, 01 to 38, and 97
// (other territory).
func validStateCode(s string) bool {
	if len(s) != 2 || !isDigits(s) {
		return false
	}
	n := int(s[0]-'0')*10 + int(s[1]-'0')
	return (n >= 1 && n <= 38) || n == 97
}

// validGSTIN checks the shape of a GSTIN: 15 characters, uppercase letters
// and digits, starting with the state code. The check digit is not verified
// here; ADR-049 notes it.
func validGSTIN(s string) bool {
	if len(s) != 15 || !isDigits(s[:2]) {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

// ValidHSN accepts an HSN or SAC code of 4, 6 or 8 digits: what GST
// refuses a line without. The menu editor checks with it too, so an item
// that could never be invoiced is refused when it is saved, not at the till.
func ValidHSN(s string) bool {
	return (len(s) == 4 || len(s) == 6 || len(s) == 8) && isDigits(s)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// InvoiceFields implements TaxProfile: what a GST tax invoice must print.
func (GST) InvoiceFields() []Field {
	return []Field{
		{Key: "gstin", Label: "GSTIN", Scope: ScopeInvoice},
		{Key: "invoice_no", Label: "Invoice No.", Scope: ScopeInvoice},
		{Key: "invoice_date", Label: "Date", Scope: ScopeInvoice},
		{Key: "place_of_supply", Label: "Place of supply", Scope: ScopeInvoice},
		{Key: "hsn", Label: "HSN/SAC", Scope: ScopeLine},
		{Key: "taxable_value", Label: "Taxable value", Scope: ScopeLine},
		{Key: "rate", Label: "Rate", Scope: ScopeLine},
		{Key: "cgst", Label: "CGST", Scope: ScopeLine},
		{Key: "sgst", Label: "SGST/UTGST", Scope: ScopeLine},
		{Key: "igst", Label: "IGST", Scope: ScopeLine},
	}
}

// RegisterColumns implements TaxProfile: the rate-wise columns a GST sales
// register and GSTR-1 need.
func (GST) RegisterColumns() []Column {
	return []Column{
		{Key: "invoice_no", Label: "Invoice No."},
		{Key: "invoice_date", Label: "Invoice date"},
		{Key: "place_of_supply", Label: "Place of supply"},
		{Key: "hsn", Label: "HSN/SAC"},
		{Key: "rate", Label: "Rate"},
		{Key: "taxable_value", Label: "Taxable value"},
		{Key: "cgst", Label: "CGST"},
		{Key: "sgst", Label: "SGST/UTGST"},
		{Key: "igst", Label: "IGST"},
		{Key: "invoice_value", Label: "Invoice value"},
	}
}
