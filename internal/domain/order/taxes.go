package order

import (
	"fmt"
	"slices"

	"github.com/mez-pos/mez/internal/domain/tax"
)

// TaxSnapshot is how the venue was taxed when the order opened (ADR-374): the
// profile by its code and the venue's side of the calculation. It rides in
// order.opened and in the State, so that the order is projected under the
// taxes it was sold under for the rest of its life, whatever the owner
// changes in the settings after. An order opened before it existed has none,
// and is projected under the venue's taxes as they are now, as before.
type TaxSnapshot struct {
	// Profile is the code the venue's profile is stored under: "none",
	// "gst_in", "vat_uk", "sales_tax_us".
	Profile       string          `json:"profile"`
	Currency      string          `json:"currency"`
	Pricing       tax.Pricing     `json:"pricing"`
	GSTIN         string          `json:"gstin,omitempty"`
	StateCode     string          `json:"state_code,omitempty"`
	PlaceOfSupply string          `json:"place_of_supply,omitempty"`
	VATNumber     string          `json:"vat_number,omitempty"`
	Jurisdiction  []tax.Component `json:"jurisdiction,omitempty"`
}

// SnapshotTaxes is the venue's taxes as an order opening now carries them.
// code is the profile's stored code; an untaxed venue is a snapshot all the
// same, so that a venue taxed later does not re-price the orders before.
func SnapshotTaxes(code string, t Taxes) *TaxSnapshot {
	if t.Profile == nil {
		code = tax.CodeNone
	}
	return &TaxSnapshot{
		Profile: code, Currency: t.Venue.Currency, Pricing: t.Venue.Pricing,
		GSTIN: t.Venue.GSTIN, StateCode: t.Venue.StateCode, PlaceOfSupply: t.Venue.PlaceOfSupply,
		VATNumber: t.Venue.VATNumber, Jurisdiction: slices.Clone(t.Venue.Jurisdiction),
	}
}

// Taxes is the snapshot as the projection uses it. A code this build does
// not know is ErrTaxConfig: an order from a newer build than this one.
func (s *TaxSnapshot) Taxes() (Taxes, error) {
	profile, ok := tax.ProfileByCode(s.Profile)
	if !ok {
		return Taxes{}, fmt.Errorf("%w: the order was opened under tax profile %q, which this build does not know", ErrTaxConfig, s.Profile)
	}
	if profile == nil {
		return Taxes{}, nil
	}
	return Taxes{Profile: profile, Venue: tax.VenueContext{
		Currency: s.Currency, Pricing: s.Pricing, GSTIN: s.GSTIN, StateCode: s.StateCode,
		PlaceOfSupply: s.PlaceOfSupply, VATNumber: s.VATNumber, Jurisdiction: slices.Clone(s.Jurisdiction),
	}}, nil
}

// clone is a copy that aliases nothing of s.
func (s *TaxSnapshot) clone() *TaxSnapshot {
	if s == nil {
		return nil
	}
	c := *s
	c.Jurisdiction = slices.Clone(s.Jurisdiction)
	return &c
}
