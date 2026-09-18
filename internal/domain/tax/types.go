package tax

import (
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
)

// RateScale is 100%, as a Rate.
const RateScale = 100000

// Rate is a tax rate in thousandths of a percent: 5000 is 5%, 8875 is
// 8.875%. It is finer than the basis points money uses because real US rates
// need three decimal places (New York City is 8.875%, of which 0.375% is the
// transit district), and because 0.25% GST must split evenly into CGST and
// SGST. ADR-049.
type Rate int32

// RateFromBP converts basis points (1800 = 18%) to a Rate.
func RateFromBP(bp int32) Rate { return Rate(bp) * 10 }

// Validate refuses a rate below 0% or above 100%.
func (r Rate) Validate() error {
	if r < 0 || r > RateScale {
		return fmt.Errorf("%w: %d is not 0 to %d", ErrInvalidRate, r, RateScale)
	}
	return nil
}

// String writes the rate as a percentage with no trailing zeros: "5%",
// "8.875%", "0.25%".
func (r Rate) String() string {
	whole, frac := int32(r)/1000, int32(r)%1000
	if frac == 0 {
		return fmt.Sprintf("%d%%", whole)
	}
	s := fmt.Sprintf("%d.%03d", whole, frac)
	for s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return s + "%"
}

// Pricing says whether menu prices already include tax.
type Pricing string

// A venue prices one way or the other. India and the UK usually print
// inclusive prices; the US always adds tax at the till.
const (
	Inclusive Pricing = "inclusive"
	Exclusive Pricing = "exclusive"
)

// Line is one line of a bill as tax sees it: what it is worth after every
// discount, and the tax facts snapshotted when it was added. It is not
// order.Line: this package must not import the order package, which will
// import it.
type Line struct {
	ID     int64        // the order line it came from
	Name   string       // for the invoice
	HSN    string       // GST: HSN or SAC code; other profiles ignore it
	Amount money.Amount // after line and order discounts; includes tax if pricing is inclusive
	Rate   Rate         // GST and VAT: the item's rate. US sales tax uses the jurisdiction instead
	Exempt bool         // no tax at all, whatever the rate
}

// VenueContext is the venue's side of the calculation. Each profile reads the
// fields it needs and refuses a context missing them.
type VenueContext struct {
	Currency string
	Pricing  Pricing

	// GST (India)
	GSTIN         string // the venue's GSTIN, printed on every invoice
	StateCode     string // the venue's state, two digits: "29" is Karnataka
	PlaceOfSupply string // the buyer's state; empty means the venue's own

	// VAT
	VATNumber string

	// US sales tax: the components of the rate where the sale happens, each
	// reported to its own authority.
	Jurisdiction []Component
}

// Component is one named part of a tax: CGST, SGST, IGST, VAT, or a US state,
// county or district rate.
type Component struct {
	Name string `json:"name"`
	Rate Rate   `json:"rate"`
}

// ComponentAmount is a component as charged on one line.
type ComponentAmount struct {
	Name   string       `json:"name"`
	Rate   Rate         `json:"rate"`
	Amount money.Amount `json:"amount"`
}

// LineTax is the tax on one line. Taxable is the line's value excluding tax;
// for an exempt or nil-rated line that is its whole value. Taxable plus Tax
// is Total, exactly, and under inclusive pricing Total is the line's Amount,
// to the paisa.
type LineTax struct {
	LineID     int64             `json:"line_id"`
	HSN        string            `json:"hsn,omitempty"`
	Rate       Rate              `json:"rate"`
	Taxable    money.Amount      `json:"taxable"`
	Components []ComponentAmount `json:"components"`
	Tax        money.Amount      `json:"tax"`
	Total      money.Amount      `json:"total"`
}

// ComponentTotal is one component summed over the order: the rate-wise
// summary an invoice prints and a tax register reports. Taxable counts only
// the lines this component applies to.
type ComponentTotal struct {
	Name    string       `json:"name"`
	Rate    Rate         `json:"rate"`
	Taxable money.Amount `json:"taxable"`
	Amount  money.Amount `json:"amount"`
}

// TaxResult is the tax on an order. It is also the snapshot an issued
// invoice carries, so everything used to compute it is in it.
type TaxResult struct {
	Profile    string           `json:"profile"`
	Pricing    Pricing          `json:"pricing"`
	Lines      []LineTax        `json:"lines"`
	Components []ComponentTotal `json:"components"`
	Gross      money.Amount     `json:"gross"`   // the lines' Amounts, as given
	Taxable    money.Amount     `json:"taxable"` // the lines' value excluding tax
	Tax        money.Amount     `json:"tax"`
	Total      money.Amount     `json:"total"` // Taxable + Tax: what the guest pays
}

// FieldScope says whether a field is printed once or on every line.
type FieldScope string

// Invoice fields are printed once, line fields per line.
const (
	ScopeInvoice FieldScope = "invoice"
	ScopeLine    FieldScope = "line"
)

// Field is something a profile requires on a printed invoice.
type Field struct {
	Key   string
	Label string
	Scope FieldScope
}

// Column is something a profile requires in the tax register export.
type Column struct {
	Key   string
	Label string
}

// TaxProfile is a tax regime, from docs/02-domain.md. It decides how tax is
// computed, what an invoice must show, and what the tax register needs.
// Nothing outside a profile may know which regime a venue is under.
type TaxProfile interface {
	Name() string
	Compute(lines []Line, ctx VenueContext) (TaxResult, error)
	InvoiceFields() []Field
	RegisterColumns() []Column
}
