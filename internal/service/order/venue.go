package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/invoice"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/payment"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// The venue settings this package reads, from the settings table. Anything
// not set takes the default noted.
const (
	// SettingTaxPricing is "inclusive" or "exclusive". Default: inclusive,
	// except US sales tax, which is always exclusive.
	SettingTaxPricing = "tax.pricing"

	// SettingTaxStateCode is the venue's two digit GST state code, "29" for
	// Karnataka. Required under GST.
	SettingTaxStateCode = "tax.state_code"

	// SettingTaxJurisdiction is the US sales tax rates where the venue is, as
	// JSON: [{"name":"CA","rate":7250}, ...], rates in thousandths of a
	// percent as tax.Rate. Required under US sales tax.
	SettingTaxJurisdiction = "tax.jurisdiction"

	// SettingTicketLateMinutes is how long an open ticket may run before the
	// till shows its time in vermilion. Default: 20. ADR-074.
	SettingTicketLateMinutes = "terminal.ticket_late_minutes"

	// SettingInvoiceSeries is the code of the series invoices are issued in.
	// Default: INV.
	SettingInvoiceSeries = "invoice.series"

	// SettingInvoicePrefix is written by {prefix}. Default: the series code.
	SettingInvoicePrefix = "invoice.prefix"

	// SettingInvoiceTemplate is how a number is written. Default:
	// {prefix}/{fy:short}/{seq:06d}, which is 16 characters, GST's limit.
	SettingInvoiceTemplate = "invoice.template"
)

const (
	// SettingCreditSeries is the code of the series credit notes are issued
	// in (ADR-083), and SettingCreditPrefix what {prefix} writes for them.
	// They use the invoice template. Defaults: "CN", and the code.
	SettingCreditSeries = "credit_note.series"
	SettingCreditPrefix = "credit_note.prefix"

	// SettingUPIAddress is the venue's UPI address, "cafe@okbank": where a
	// guest paying from a phone is sent (ADR-215). Unset, no bill offers
	// UPI. SettingUPIPayee is the name the guest's app shows; unset, it is
	// the venue's.
	SettingUPIAddress = "payment.upi_address"
	SettingUPIPayee   = "payment.upi_payee"

	// SettingPayMode is when a guest ordering from a phone pays: PayAtTable,
	// the default, or PayFirst (ADR-225).
	SettingPayMode = "payment.mode"

	// SettingTipMaxPercent is the largest tip a payment may carry without a
	// manager, as a whole percent of the payment: 0 to MaxTipMaxPercent, 0
	// meaning no tip without one. Default: DefaultTipMaxPercent. ADR-261.
	SettingTipMaxPercent = "tips.max_percent"

	// GSTMaxInvoiceNo is the longest invoice number GST allows.
	GSTMaxInvoiceNo = 16
)

// What a series is when the venue has set none: the code invoices are issued
// in, the code credit notes are, and how a number is written.
const (
	DefaultSeries       = "INV"
	DefaultCreditSeries = "CN"
	DefaultTemplate     = "{prefix}/{fy:short}/{seq:06d}"
)

// ErrVenueConfig means the venue's settings cannot sell: a tax profile
// missing what it needs, or an invoice series that cannot issue a number.
// It is the back office's to fix, not the till's.
var ErrVenueConfig = errors.New("venue is not configured to sell")

// venue is everything a command needs to know about where it is being sold.
type venue struct {
	ID       int64
	Currency string
	Location *time.Location
	Taxes    order.Taxes
	// TaxCode is the profile's stored code, which an order opening records
	// with its taxes (ADR-374).
	TaxCode   string
	Series    invoice.Series
	Credits   invoice.Series // the credit note series
	LateAfter time.Duration
	Seller    order.InvoiceSeller // as an invoice issued now names the venue
	PayMode   PayMode             // when a guest ordering from a phone pays
	// TipMaxPercent is the tip ceiling: a tip over this percent of its
	// payment needs a manager (ADR-261).
	TipMaxPercent int64
}

// VenueTaxes is how the venue's orders are taxed, read through q, the same
// way every command reads it: what a Z-report projects its orders under.
func VenueTaxes(ctx context.Context, q *gen.Queries, venueID int64) (order.Taxes, error) {
	v, err := loadVenue(ctx, q, venueID)
	if err != nil {
		return order.Taxes{}, err
	}
	return v.Taxes, nil
}

// loadVenue reads a venue and its settings. It runs on every command: two
// indexed reads against a local file, cheaper than keeping a cache coherent
// with the back office.
func loadVenue(ctx context.Context, q *gen.Queries, venueID int64) (venue, error) {
	row, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return venue{}, fmt.Errorf("%w: no venue %d", ErrVenueConfig, venueID)
	}
	if err != nil {
		return venue{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	rows, err := q.SettingsForVenue(ctx, venueID)
	if err != nil {
		return venue{}, fmt.Errorf("reading the settings of venue %d: %w", venueID, err)
	}
	settings := make(map[string]string, len(rows))
	for _, r := range rows {
		settings[r.Key] = r.Value
	}
	return venueFrom(row, settings)
}

// venueFrom builds a venue from its row and settings, checking that what is
// configured can actually be used.
func venueFrom(row gen.Venue, settings map[string]string) (venue, error) {
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return venue{}, fmt.Errorf("%w: timezone %q: %v", ErrVenueConfig, row.Timezone, err)
	}
	taxes, err := taxesFor(row, settings)
	if err != nil {
		return venue{}, err
	}
	series, err := seriesFor(row, settings)
	if err != nil {
		return venue{}, err
	}
	credits, err := creditSeriesFor(row, settings, series)
	if err != nil {
		return venue{}, err
	}
	return venue{
		ID: row.ID, Currency: row.Currency, Location: loc, Taxes: taxes, TaxCode: row.TaxProfile, Series: series, Credits: credits,
		LateAfter: lateAfter(settings[SettingTicketLateMinutes]),
		Seller:    sellerFor(row, settings),
		PayMode:   payModeOf(settings[SettingPayMode]),

		TipMaxPercent: int64(TipMaxPercentOf(settings[SettingTipMaxPercent])),
	}, nil
}

// sellerFor is the venue as its invoices name it. The legal name falls back
// to the trading name, which is what setup does when none is given.
func sellerFor(row gen.Venue, settings map[string]string) order.InvoiceSeller {
	s := order.InvoiceSeller{
		Name: row.Name, LegalName: row.LegalName, TaxProfile: row.TaxProfile, Registration: row.TaxRegistrationID,
	}
	if s.LegalName == "" {
		s.LegalName = row.Name
	}
	if row.TaxProfile == "gst_in" {
		s.StateCode = settings[SettingTaxStateCode]
	}
	return s
}

// taxesFor builds the venue's tax regime from its profile, registration and
// settings. The profile then checks the context on every computation; this
// only refuses what is certain to fail, so the owner hears it at setup.
func taxesFor(row gen.Venue, settings map[string]string) (order.Taxes, error) {
	ctx := tax.VenueContext{Currency: row.Currency, Pricing: tax.Inclusive}
	if p, ok := settings[SettingTaxPricing]; ok {
		ctx.Pricing = tax.Pricing(p)
		if ctx.Pricing != tax.Inclusive && ctx.Pricing != tax.Exclusive {
			return order.Taxes{}, fmt.Errorf("%w: %s is %q, not inclusive or exclusive", ErrVenueConfig, SettingTaxPricing, p)
		}
	}

	var profile tax.TaxProfile
	switch row.TaxProfile {
	case "none":
		return order.Taxes{}, nil
	case "gst_in":
		profile = tax.GST{}
		ctx.GSTIN = row.TaxRegistrationID
		ctx.StateCode = settings[SettingTaxStateCode]
		if ctx.GSTIN == "" || len(ctx.StateCode) != 2 {
			return order.Taxes{}, fmt.Errorf("%w: GST needs a GSTIN and a two digit %s", ErrVenueConfig, SettingTaxStateCode)
		}
	case "vat_uk":
		profile = tax.VAT{}
		ctx.VATNumber = row.TaxRegistrationID
		if ctx.VATNumber == "" {
			return order.Taxes{}, fmt.Errorf("%w: VAT needs a VAT number", ErrVenueConfig)
		}
	case "sales_tax_us":
		profile = tax.USSalesTax{}
		if _, set := settings[SettingTaxPricing]; !set {
			ctx.Pricing = tax.Exclusive
		}
		if err := json.Unmarshal([]byte(settings[SettingTaxJurisdiction]), &ctx.Jurisdiction); err != nil || len(ctx.Jurisdiction) == 0 {
			return order.Taxes{}, fmt.Errorf("%w: US sales tax needs %s as a JSON list of rates", ErrVenueConfig, SettingTaxJurisdiction)
		}
	default:
		return order.Taxes{}, fmt.Errorf("%w: unknown tax profile %q", ErrVenueConfig, row.TaxProfile)
	}
	return order.Taxes{Profile: profile, Venue: ctx}, nil
}

// seriesFor builds the invoice series the venue issues in.
func seriesFor(row gen.Venue, settings map[string]string) (invoice.Series, error) {
	code := strings.TrimSpace(settings[SettingInvoiceSeries])
	if code == "" {
		code = DefaultSeries
	}
	prefix, ok := settings[SettingInvoicePrefix]
	if !ok {
		prefix = code
	}
	template := settings[SettingInvoiceTemplate]
	if template == "" {
		template = DefaultTemplate
	}
	maxLength := 0
	if row.TaxProfile == "gst_in" {
		maxLength = GSTMaxInvoiceNo
	}
	fy := invoice.FYRule{StartMonth: time.Month(row.FinancialYearStartMonth), StartDay: 1}
	s, err := invoice.NewSeries(code, prefix, fy, template, maxLength)
	if err != nil {
		return invoice.Series{}, fmt.Errorf("%w: invoice series: %w", ErrVenueConfig, err)
	}
	return s, nil
}

// creditSeriesFor builds the credit note series. It must not be the invoice
// series: the two share the counters table, and one run of numbers for both
// would put credit notes in the gaps of the invoice register.
func creditSeriesFor(row gen.Venue, settings map[string]string, invoices invoice.Series) (invoice.Series, error) {
	code := strings.TrimSpace(settings[SettingCreditSeries])
	if code == "" {
		code = DefaultCreditSeries
	}
	if code == invoices.Code {
		return invoice.Series{}, fmt.Errorf("%w: credit notes and invoices both use series %q", ErrVenueConfig, code)
	}
	prefix, ok := settings[SettingCreditPrefix]
	if !ok {
		prefix = code
	}
	template := settings[SettingInvoiceTemplate]
	if template == "" {
		template = DefaultTemplate
	}
	maxLength := 0
	if row.TaxProfile == "gst_in" {
		maxLength = GSTMaxInvoiceNo
	}
	fy := invoice.FYRule{StartMonth: time.Month(row.FinancialYearStartMonth), StartDay: 1}
	s, err := invoice.NewSeries(code, prefix, fy, template, maxLength)
	if err != nil {
		return invoice.Series{}, fmt.Errorf("%w: credit note series: %w", ErrVenueConfig, err)
	}
	return s, nil
}

// DefaultLateMinutes is how long a ticket runs before it shows as late, for a
// venue that has set none. The settings registry takes its default from here.
const DefaultLateMinutes = 20

// lateAfter reads SettingTicketLateMinutes. A value that is not a whole
// number of minutes from 1 to 600 falls back to the default rather than
// refusing to sell: it only colours a clock.
func lateAfter(v string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 || n > 600 {
		return DefaultLateMinutes * time.Minute
	}
	return time.Duration(n) * time.Minute
}

// The tip ceiling's default and its bounds (ADR-261). Half the payment is a
// generous tip that a slip of the keypad rarely reaches; ten times it is as
// far as a venue may wave tips through.
const (
	DefaultTipMaxPercent = 50
	MaxTipMaxPercent     = 1000
)

// TipMaxPercentOf reads SettingTipMaxPercent. Unset, or a value the settings
// screen would have refused, is the default: the registry refuses a bad value
// on the way in, and a ceiling must not stop the till selling.
func TipMaxPercentOf(stored string) int {
	n, err := strconv.Atoi(strings.TrimSpace(stored))
	if err != nil || n < 0 || n > MaxTipMaxPercent {
		return DefaultTipMaxPercent
	}
	return n
}

// SeriesOf is the series invoices and credit notes are issued in, read from a
// venue row and its settings the way every sale reads them, so a settings
// screen shows and checks what the till will do. It does not read the tax
// profile: numbering can be fixed on a venue whose tax is not yet set up.
func SeriesOf(row gen.Venue, settings map[string]string) (invoices, credits invoice.Series, err error) {
	invoices, err = seriesFor(row, settings)
	if err != nil {
		return invoice.Series{}, invoice.Series{}, err
	}
	credits, err = creditSeriesFor(row, settings, invoices)
	if err != nil {
		return invoice.Series{}, invoice.Series{}, err
	}
	return invoices, credits, nil
}

// UPIAccount is where the venue is paid by UPI, or an account with no
// address when it has not said. The payee is the venue's name unless the
// owner gave another.
func (s *Service) UPIAccount(ctx context.Context, venueID int64) (payment.UPIAccount, error) {
	q := s.db.Queries()
	row, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return payment.UPIAccount{}, fmt.Errorf("%w: no venue %d", ErrVenueConfig, venueID)
	}
	if err != nil {
		return payment.UPIAccount{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	rows, err := q.SettingsForVenue(ctx, venueID)
	if err != nil {
		return payment.UPIAccount{}, fmt.Errorf("reading the settings of venue %d: %w", venueID, err)
	}
	account := payment.UPIAccount{Payee: row.Name}
	for _, r := range rows {
		switch value := strings.TrimSpace(r.Value); {
		case r.Key == SettingUPIAddress:
			account.Address = value
		case r.Key == SettingUPIPayee && value != "":
			account.Payee = value
		}
	}
	return account, nil
}

// PayMode is when a guest ordering from their phone pays.
type PayMode string

// The modes. The strings are what is stored and sent.
const (
	// PayAtTable: order now, pay when leaving. The first order of a
	// sitting is a request staff accept (ADR-209).
	PayAtTable PayMode = "pay_at_table"
	// PayFirst: an order is sent to the kitchen once it is paid for, and
	// needs no acceptance: paying is what says someone is there.
	PayFirst PayMode = "prepay"
)

// PayModes is every mode, for the generated vocabulary.
func PayModes() []PayMode { return []PayMode{PayAtTable, PayFirst} }

// payModeOf reads the setting. Unset, or anything this build does not
// know, is PayAtTable: the mode that holds nobody's order for a payment.
func payModeOf(stored string) PayMode {
	if PayMode(strings.TrimSpace(stored)) == PayFirst {
		return PayFirst
	}
	return PayAtTable
}
