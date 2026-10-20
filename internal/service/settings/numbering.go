package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/invoice"
	"github.com/mez-pos/mez/internal/domain/money"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// ErrNumberingLocked refuses a change of financial year start once a number
// has been issued: numbering restarts at that boundary, and moving it
// under issued numbers is an accountant's migration, not a setting.
var ErrNumberingLocked = errors.New("numbers have been issued")

// ErrNumberTaken refuses a series whose next number is one already issued.
var ErrNumberTaken = errors.New("that number has already been issued")

// NumberingInput is how invoices and credit notes are numbered, the
// financial year they restart in, and how cash is rounded.
type NumberingInput struct {
	// InvoiceSeries and CreditSeries name the two counters: capitals and
	// digits. Empty takes INV and CN.
	InvoiceSeries string `json:"invoice_series"`
	CreditSeries  string `json:"credit_series"`
	// InvoicePrefix and CreditPrefix are what {prefix} writes. Empty
	// follows the series code.
	InvoicePrefix string `json:"invoice_prefix"`
	CreditPrefix  string `json:"credit_prefix"`
	// Template is how a number is written, for both: empty takes
	// {prefix}/{fy:short}/{seq:06d}.
	Template string `json:"template"`
	// FinancialYearStartMonth is 1 to 12: numbering restarts at 1 on the 1st.
	FinancialYearStartMonth int `json:"financial_year_start_month"`
	// CashRoundingStep is in minor units: 50 settles cash to the nearest 50
	// paise, 1 rounds nothing.
	CashRoundingStep int64 `json:"cash_rounding_step"`
}

// Numbering is the numbering as it stands, and what it would write next.
type Numbering struct {
	NumberingInput
	Currency string
	// NextInvoiceNo and NextCreditNo are the next numbers each series would
	// issue now, in the financial year it is.
	NextInvoiceNo string
	NextCreditNo  string
	// NumberLimit is the longest number the tax profile allows; 0 for none.
	NumberLimit int
	// FYLocked is true once a number has been issued: the financial year
	// start can no longer change.
	FYLocked bool
	// MaxRoundingStep is the largest cash rounding step: one whole unit of
	// the currency.
	MaxRoundingStep int64
}

// rawVenue is the venue row and its settings, read through q.
func rawVenue(ctx context.Context, q *gen.Queries, venueID int64) (gen.Venue, map[string]string, error) {
	v, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Venue{}, nil, ErrNoVenue
	}
	if err != nil {
		return gen.Venue{}, nil, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	rows, err := q.SettingsForVenue(ctx, venueID)
	if err != nil {
		return gen.Venue{}, nil, fmt.Errorf("reading the settings of venue %d: %w", venueID, err)
	}
	stored := make(map[string]string, len(rows))
	for _, r := range rows {
		stored[r.Key] = r.Value
	}
	return v, stored, nil
}

// Numbering reads how the venue numbers its invoices and credit notes.
func (s *Service) Numbering(ctx context.Context, venueID int64) (Numbering, error) {
	q := s.db.Queries()
	v, stored, err := rawVenue(ctx, q, venueID)
	if err != nil {
		return Numbering{}, err
	}
	return s.numberingOf(ctx, q, v, stored)
}

// numberingOf is the numbering a venue row and its settings describe, and
// what each series would write next. The row and settings may be a draft.
func (s *Service) numberingOf(ctx context.Context, q *gen.Queries, v gen.Venue, stored map[string]string) (Numbering, error) {
	inv, credit, err := svcorder.SeriesOf(v, stored)
	if err != nil {
		return Numbering{}, fmt.Errorf("venue %d's numbering cannot issue a number: %w", v.ID, err)
	}
	issued, err := q.InvoiceSeriesCount(ctx, v.ID)
	if err != nil {
		return Numbering{}, fmt.Errorf("counting the series of venue %d: %w", v.ID, err)
	}
	out := Numbering{
		NumberingInput: NumberingInput{
			InvoiceSeries: inv.Code, CreditSeries: credit.Code, InvoicePrefix: inv.Prefix, CreditPrefix: credit.Prefix,
			Template: inv.Template.String(), FinancialYearStartMonth: int(v.FinancialYearStartMonth),
			CashRoundingStep: v.CashRoundingStepMinor,
		},
		Currency: v.Currency, NumberLimit: inv.MaxLength, FYLocked: issued > 0,
		MaxRoundingStep: maxRoundingStep(v.Currency),
	}
	if out.NextInvoiceNo, err = s.nextNumber(ctx, q, v, inv); err != nil {
		return Numbering{}, err
	}
	if out.NextCreditNo, err = s.nextNumber(ctx, q, v, credit); err != nil {
		return Numbering{}, err
	}
	return out, nil
}

// PreviewNumbering is what saving in would give, and nothing saved: the
// numbering as it would stand, with the next number of each series. It runs
// the checks a save runs, so a draft a save would refuse is refused here with
// the same words and field. A financial year start that a save would refuse
// because numbers are issued is not refused: the screen says so beside it.
func (s *Service) PreviewNumbering(ctx context.Context, venueID int64, in NumberingInput) (Numbering, error) {
	q := s.db.Queries()
	v, stored, err := rawVenue(ctx, q, venueID)
	if err != nil {
		return Numbering{}, err
	}
	in, keys, err := cleanNumbering(in, v.Currency)
	if err != nil {
		return Numbering{}, err
	}
	v.FinancialYearStartMonth, v.CashRoundingStepMinor = int64(in.FinancialYearStartMonth), in.CashRoundingStep
	for k, val := range keys {
		if val == "" {
			delete(stored, k)
		} else {
			stored[k] = val
		}
	}
	inv, credit, err := svcorder.SeriesOf(v, stored)
	if err != nil {
		return Numbering{}, &Invalid{"template", "That cannot issue a number: " + err.Error()}
	}
	var ne *invoice.NumberingError
	if err := invoice.CheckNumbering(inv, credit); errors.As(err, &ne) {
		return Numbering{}, &Invalid{ne.Field, "Change this: " + ne.Reason + "."}
	} else if err != nil {
		return Numbering{}, err
	}
	if err := s.checkNextNumbers(ctx, q, v, inv, credit); err != nil {
		return Numbering{}, err
	}
	return s.numberingOf(ctx, q, v, stored)
}

// maxRoundingStep is one whole unit of the currency, in minor units: cash
// is never rounded by more than that.
func maxRoundingStep(currency string) int64 {
	step := int64(1)
	for i := 0; i < money.Exponent(currency); i++ {
		step *= 10
	}
	return step
}

// nextNumber is the number series would issue now: one past its counter in
// the financial year it is, at the venue.
func (s *Service) nextNumber(ctx context.Context, q *gen.Queries, v gen.Venue, series invoice.Series) (string, error) {
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return "", fmt.Errorf("venue %d's timezone %q: %w", v.ID, v.Timezone, err)
	}
	fy, err := series.FY.YearOf(s.clock.Now(), loc)
	if err != nil {
		return "", fmt.Errorf("the financial year of venue %d: %w", v.ID, err)
	}
	last, err := q.InvoiceSeriesLastSeq(ctx, gen.InvoiceSeriesLastSeqParams{VenueID: v.ID, Series: series.Code, FyStart: int64(fy.StartYear)})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("reading the counter of series %s: %w", series.Code, err)
	}
	no, err := series.Number(fy, last+1)
	if err != nil {
		return "", fmt.Errorf("writing the next number of series %s: %w", series.Code, err)
	}
	return no, nil
}

// cleanNumbering trims and checks what can be checked alone, and says what
// to store: a value that is the default, or a prefix that is the code, is
// not stored, so it keeps following the thing it was derived from.
func cleanNumbering(in NumberingInput, currency string) (NumberingInput, map[string]string, error) {
	in.InvoiceSeries = strings.ToUpper(strings.TrimSpace(in.InvoiceSeries))
	in.CreditSeries = strings.ToUpper(strings.TrimSpace(in.CreditSeries))
	in.InvoicePrefix = strings.TrimSpace(in.InvoicePrefix)
	in.CreditPrefix = strings.TrimSpace(in.CreditPrefix)
	in.Template = strings.TrimSpace(in.Template)
	if in.InvoiceSeries == "" {
		in.InvoiceSeries = svcorder.DefaultSeries
	}
	if in.CreditSeries == "" {
		in.CreditSeries = svcorder.DefaultCreditSeries
	}
	if in.InvoicePrefix == "" {
		in.InvoicePrefix = in.InvoiceSeries
	}
	if in.CreditPrefix == "" {
		in.CreditPrefix = in.CreditSeries
	}
	if in.Template == "" {
		in.Template = svcorder.DefaultTemplate
	}

	checks := []struct {
		field, key, value string
	}{
		{"invoice_series", svcorder.SettingInvoiceSeries, in.InvoiceSeries},
		{"credit_series", svcorder.SettingCreditSeries, in.CreditSeries},
		{"invoice_prefix", svcorder.SettingInvoicePrefix, in.InvoicePrefix},
		{"credit_prefix", svcorder.SettingCreditPrefix, in.CreditPrefix},
		{"template", svcorder.SettingInvoiceTemplate, in.Template},
	}
	for _, c := range checks {
		k, _ := find(c.key)
		if msg := k.Check(c.value); msg != "" {
			return in, nil, &Invalid{c.field, msg}
		}
	}
	switch {
	case in.InvoiceSeries == in.CreditSeries:
		return in, nil, &Invalid{"credit_series", "Invoices and credit notes need different series."}
	case in.FinancialYearStartMonth < 1 || in.FinancialYearStartMonth > 12:
		return in, nil, &Invalid{"financial_year_start_month", "Choose the month the financial year starts in."}
	case in.CashRoundingStep < 1 || in.CashRoundingStep > maxRoundingStep(currency):
		return in, nil, &Invalid{"cash_rounding_step", fmt.Sprintf("Enter a step from 1 to %d minor units (1 rounds nothing).", maxRoundingStep(currency))}
	}

	ifNot := func(value, fallback string) string {
		if value == fallback {
			return ""
		}
		return value
	}
	return in, map[string]string{
		svcorder.SettingInvoiceSeries:   ifNot(in.InvoiceSeries, svcorder.DefaultSeries),
		svcorder.SettingCreditSeries:    ifNot(in.CreditSeries, svcorder.DefaultCreditSeries),
		svcorder.SettingInvoicePrefix:   ifNot(in.InvoicePrefix, in.InvoiceSeries),
		svcorder.SettingCreditPrefix:    ifNot(in.CreditPrefix, in.CreditSeries),
		svcorder.SettingInvoiceTemplate: ifNot(in.Template, svcorder.DefaultTemplate),
	}, nil
}

// UpdateNumbering saves how invoices and credit notes are numbered, the
// financial year start and the cash rounding step, or nothing.
//
// After the write, in the same transaction, it reads the series back as a
// sale does and checks them together (invoice.CheckNumbering), and that the
// next number is not one already issued: a series renamed but left with its
// old prefix would otherwise write INV/26-27/000001 a second time, and the
// invoices table would refuse it at the till.
func (s *Service) UpdateNumbering(ctx context.Context, venueID int64, in NumberingInput) error {
	venue, err := s.db.Queries().VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoVenue
	}
	if err != nil {
		return fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	in, keys, err := cleanNumbering(in, venue.Currency)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, _, err := rawVenue(ctx, q, venueID)
		if err != nil {
			return err
		}
		if int64(in.FinancialYearStartMonth) != v.FinancialYearStartMonth {
			issued, err := q.InvoiceSeriesCount(ctx, venueID)
			if err != nil {
				return fmt.Errorf("counting the series of venue %d: %w", venueID, err)
			}
			if issued > 0 {
				return fmt.Errorf("%w: %w", ErrNumberingLocked, &Invalid{"financial_year_start_month",
					"Numbers have been issued, and numbering restarts on this date. It can no longer change here."})
			}
		}
		if _, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID: venueID, Name: v.Name, LegalName: v.LegalName, Timezone: v.Timezone,
			FinancialYearStartMonth: int64(in.FinancialYearStartMonth), TaxProfile: v.TaxProfile,
			TaxRegistrationID: v.TaxRegistrationID, CashRoundingStepMinor: in.CashRoundingStep, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("updating venue %d: %w", venueID, err)
		}
		if err := saveKeys(ctx, q, venueID, keys, now); err != nil {
			return err
		}

		v, stored, err := rawVenue(ctx, q, venueID)
		if err != nil {
			return err
		}
		inv, credit, err := svcorder.SeriesOf(v, stored)
		if err != nil {
			return &Invalid{"template", "That cannot issue a number: " + err.Error()}
		}
		var ne *invoice.NumberingError
		if err := invoice.CheckNumbering(inv, credit); errors.As(err, &ne) {
			return &Invalid{ne.Field, "Change this: " + ne.Reason + "."}
		} else if err != nil {
			return err
		}
		return s.checkNextNumbers(ctx, q, v, inv, credit)
	})
}

// checkNextNumbers refuses a configuration whose next number, in either
// series, has been issued already.
func (s *Service) checkNextNumbers(ctx context.Context, q *gen.Queries, v gen.Venue, inv, credit invoice.Series) error {
	no, err := s.nextNumber(ctx, q, v, inv)
	if err != nil {
		return err
	}
	taken, err := q.InvoiceNoTaken(ctx, gen.InvoiceNoTakenParams{VenueID: v.ID, InvoiceNo: no})
	if err != nil {
		return fmt.Errorf("checking invoice number %s: %w", no, err)
	}
	if taken {
		return fmt.Errorf("%w: %w", ErrNumberTaken, &Invalid{"invoice_prefix",
			fmt.Sprintf("The next invoice would be %s, which has already been issued. Change the prefix or the series.", no)})
	}
	no, err = s.nextNumber(ctx, q, v, credit)
	if err != nil {
		return err
	}
	taken, err = q.CreditNoTaken(ctx, gen.CreditNoTakenParams{VenueID: v.ID, CreditNo: no})
	if err != nil {
		return fmt.Errorf("checking credit note number %s: %w", no, err)
	}
	if taken {
		return fmt.Errorf("%w: %w", ErrNumberTaken, &Invalid{"credit_prefix",
			fmt.Sprintf("The next credit note would be %s, which has already been issued. Change the prefix or the series.", no)})
	}
	return nil
}
