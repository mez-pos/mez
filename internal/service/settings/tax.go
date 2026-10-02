package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/tax"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// ErrOpenOrders refuses a change of tax profile while a ticket is open: its
// lines would be taxed under one regime and settled under another.
var ErrOpenOrders = errors.New("tickets are still open")

// The tax profiles a venue can choose, as venues.tax_profile stores them.
const (
	ProfileNone  = "none"
	ProfileGST   = "gst_in"
	ProfileVAT   = "vat_uk"
	ProfileSales = "sales_tax_us"
)

// The two ways a price relates to its tax.
const (
	Inclusive = string(tax.Inclusive)
	Exclusive = string(tax.Exclusive)
)

// TaxInput is the tax profile as the owner sets it. What a profile does not
// use is ignored and cleared, so switching profile leaves nothing behind.
type TaxInput struct {
	Profile string `json:"profile"`
	// RegistrationID is the GSTIN under GST and the VAT number under VAT.
	RegistrationID string `json:"registration_id"`
	// Pricing is inclusive or exclusive. Empty takes the profile's usual:
	// inclusive, or exclusive for US sales tax, which has no other way.
	Pricing   string `json:"pricing"`
	StateCode string `json:"state_code"` // GST: two digits; empty takes the GSTIN's
	// Jurisdiction is US sales tax: each authority's rate where the sale
	// happens, in thousandths of a percent.
	Jurisdiction []tax.Component `json:"jurisdiction"`
}

// ItemRef names an item on the menu.
type ItemRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TaxSetup is the venue's tax profile, and what the menu still owes it.
type TaxSetup struct {
	TaxInput
	// NeedsHSN is whether invoices print an HSN or SAC code on every line.
	NeedsHSN bool
	// ItemsWithoutHSN are the items on the menu with no valid code, when
	// NeedsHSN. They sell, and their invoices fall short, until each is edited.
	ItemsWithoutHSN []ItemRef
	Classes         []TaxClass
}

// Tax is the venue's tax profile as it stands.
func (s *Service) Tax(ctx context.Context, venueID int64) (TaxSetup, error) {
	q := s.db.Queries()
	v, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return TaxSetup{}, ErrNoVenue
	}
	if err != nil {
		return TaxSetup{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	rows, err := q.SettingsForVenue(ctx, venueID)
	if err != nil {
		return TaxSetup{}, fmt.Errorf("reading the settings of venue %d: %w", venueID, err)
	}
	stored := make(map[string]string, len(rows))
	for _, r := range rows {
		stored[r.Key] = r.Value
	}

	out := TaxSetup{TaxInput: TaxInput{
		Profile: v.TaxProfile, RegistrationID: v.TaxRegistrationID, Jurisdiction: []tax.Component{},
	}}
	if out.Profile != ProfileNone {
		out.Pricing = stored[svcorder.SettingTaxPricing]
		if out.Pricing == "" {
			out.Pricing = Inclusive
		}
	}
	switch out.Profile {
	case ProfileGST:
		out.StateCode = stored[svcorder.SettingTaxStateCode]
	case ProfileSales:
		out.Pricing = Exclusive
		if raw := stored[svcorder.SettingTaxJurisdiction]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &out.Jurisdiction); err != nil {
				return TaxSetup{}, fmt.Errorf("%s of venue %d is not a list of rates: %w", svcorder.SettingTaxJurisdiction, venueID, err)
			}
		}
	}

	profile, _ := profileOf(out.Profile)
	out.NeedsHSN = tax.NeedsHSN(profile)
	out.ItemsWithoutHSN = []ItemRef{}
	if out.NeedsHSN {
		items, err := q.ItemsForHSN(ctx, venueID)
		if err != nil {
			return TaxSetup{}, fmt.Errorf("reading the items of venue %d: %w", venueID, err)
		}
		for _, it := range items {
			if !tax.ValidHSN(it.Hsn) {
				out.ItemsWithoutHSN = append(out.ItemsWithoutHSN, ItemRef{ID: it.ID, Name: it.Name})
			}
		}
	}
	if out.Classes, err = s.taxClasses(ctx, q, venueID); err != nil {
		return TaxSetup{}, err
	}
	return out, nil
}

// profileOf is the regime for a stored profile name; nil for none.
func profileOf(name string) (tax.TaxProfile, bool) {
	switch name {
	case ProfileNone:
		return nil, true
	case ProfileGST:
		return tax.GST{}, true
	case ProfileVAT:
		return tax.VAT{}, true
	case ProfileSales:
		return tax.USSalesTax{}, true
	}
	return nil, false
}

// cleanTax checks what a profile needs and returns the venue columns and
// settings keys to save. The profile's own rules have the last word: its
// Compute, run on no lines, refuses a context it could not sell under.
func cleanTax(in TaxInput, currency string) (TaxInput, map[string]string, error) {
	in.Profile = strings.TrimSpace(in.Profile)
	in.RegistrationID = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.RegistrationID), " ", ""))
	in.Pricing = strings.TrimSpace(in.Pricing)
	in.StateCode = strings.TrimSpace(in.StateCode)

	profile, ok := profileOf(in.Profile)
	if !ok {
		return in, nil, &Invalid{"profile", "Choose no tax, GST, VAT or US sales tax."}
	}
	keys := map[string]string{
		svcorder.SettingTaxPricing: "", svcorder.SettingTaxStateCode: "", svcorder.SettingTaxJurisdiction: "",
	}
	if in.Profile == ProfileNone {
		return TaxInput{Profile: ProfileNone, Jurisdiction: []tax.Component{}}, keys, nil
	}

	pricing := tax.Pricing(in.Pricing)
	switch {
	case in.Profile == ProfileSales && in.Pricing == Inclusive:
		return in, nil, &Invalid{"pricing", "US sales tax is added on top of the price: choose exclusive."}
	case in.Pricing != "" && in.Pricing != Inclusive && in.Pricing != Exclusive:
		return in, nil, &Invalid{"pricing", "Choose whether prices include tax or tax is added on top."}
	}
	if pricing == "" {
		pricing = tax.Inclusive
	}
	if in.Profile == ProfileSales {
		pricing, in.Pricing = tax.Exclusive, ""
	}
	keys[svcorder.SettingTaxPricing] = in.Pricing

	ctx := tax.VenueContext{Currency: currency, Pricing: pricing}
	switch in.Profile {
	case ProfileGST:
		if in.RegistrationID == "" {
			return in, nil, &Invalid{"registration_id", "Enter the venue's GSTIN: invoices print it."}
		}
		if in.StateCode == "" && len(in.RegistrationID) >= 2 {
			in.StateCode = in.RegistrationID[:2]
		}
		ctx.GSTIN, ctx.StateCode = in.RegistrationID, in.StateCode
		keys[svcorder.SettingTaxStateCode] = in.StateCode
	case ProfileVAT:
		if in.RegistrationID == "" {
			return in, nil, &Invalid{"registration_id", "Enter the venue's VAT number: receipts print it."}
		}
		ctx.VATNumber = in.RegistrationID
	case ProfileSales:
		if len(in.Jurisdiction) == 0 {
			return in, nil, &Invalid{"jurisdiction", "Add the rate where the venue is, like CA at 7.25%."}
		}
		comps := make([]tax.Component, len(in.Jurisdiction))
		for i, c := range in.Jurisdiction {
			c.Name = strings.TrimSpace(c.Name)
			if c.Name == "" || c.Rate.Validate() != nil {
				return in, nil, &Invalid{"jurisdiction", "Each rate needs a name and a rate from 0% to 100%."}
			}
			comps[i] = c
		}
		in.Jurisdiction, ctx.Jurisdiction = comps, comps
		raw, err := json.Marshal(comps)
		if err != nil {
			return in, nil, fmt.Errorf("encoding the jurisdiction rates: %w", err)
		}
		keys[svcorder.SettingTaxJurisdiction] = string(raw)
	}
	if _, err := profile.Compute(nil, ctx); err != nil {
		return in, nil, &Invalid{contextField(err), contextMessage(err)}
	}
	if in.Profile != ProfileGST {
		in.StateCode = ""
	}
	if in.Profile != ProfileSales {
		in.Jurisdiction = []tax.Component{}
	}
	return in, keys, nil
}

// contextField is the field a profile's refusal is about.
func contextField(err error) string {
	switch msg := err.Error(); {
	case strings.Contains(msg, "state code"), strings.Contains(msg, "registered in state"):
		return "state_code"
	case strings.Contains(msg, "jurisdiction"):
		return "jurisdiction"
	}
	return "registration_id"
}

// contextMessage is a profile's refusal without the sentinel in front.
func contextMessage(err error) string {
	msg := strings.TrimPrefix(err.Error(), tax.ErrInvalidContext.Error()+": ")
	return strings.ToUpper(msg[:1]) + msg[1:] + "."
}

// UpdateTax saves the tax profile and what it needs, or nothing. Changing the
// profile itself is refused while tickets are open; correcting a GSTIN or a
// rate under the same profile is not.
func (s *Service) UpdateTax(ctx context.Context, venueID int64, in TaxInput) error {
	venue, err := s.db.Queries().VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoVenue
	}
	if err != nil {
		return fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	in, keys, err := cleanTax(in, venue.Currency)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.VenueByID(ctx, venueID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoVenue
		}
		if err != nil {
			return fmt.Errorf("reading venue %d: %w", venueID, err)
		}
		if v.TaxProfile != in.Profile {
			open, err := q.UnsettledOrderCount(ctx, venueID)
			if err != nil {
				return fmt.Errorf("counting the open tickets of venue %d: %w", venueID, err)
			}
			if open > 0 {
				return fmt.Errorf("%w: %d", ErrOpenOrders, open)
			}
		}
		if _, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID: venueID, Name: v.Name, LegalName: v.LegalName, Timezone: v.Timezone,
			FinancialYearStartMonth: v.FinancialYearStartMonth, TaxProfile: in.Profile,
			TaxRegistrationID: in.RegistrationID, CashRoundingStepMinor: v.CashRoundingStepMinor, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("updating venue %d: %w", venueID, err)
		}
		if err := saveKeys(ctx, q, venueID, keys, now); err != nil {
			return err
		}
		// The sale path's own reading of the venue, on what was just written:
		// if it would refuse to sell, so do we, and nothing is kept.
		if _, err := svcorder.VenueTaxes(ctx, q, venueID); err != nil {
			return &Invalid{"profile", "The venue could not sell with that: " + err.Error()}
		}
		return nil
	})
}
