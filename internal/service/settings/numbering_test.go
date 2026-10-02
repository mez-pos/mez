package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// base is the numbering a new venue has, as the screen would send it.
func base() NumberingInput {
	return NumberingInput{
		InvoiceSeries: "INV", CreditSeries: "CN", Template: "{prefix}/{fy:short}/{seq:06d}",
		FinancialYearStartMonth: 4, CashRoundingStep: 50,
	}
}

func with(f func(*NumberingInput)) NumberingInput {
	in := base()
	f(&in)
	return in
}

// stock is what the venue's numbering reads as with nothing changed.
func stock() NumberingInput {
	in := base()
	in.InvoicePrefix, in.CreditPrefix = "INV", "CN"
	return in
}

// issue gives the venue an invoice in series code, at its next sequence, in
// the financial year starting 2026: as a sale would have.
func issue(t *testing.T, d *db.DB, code, no string) {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
		 VALUES (?, 1, 'counter', 'closed', 1, ?, ?)`, "ord_"+no[:3]+no[len(no)-2:], now, now); err != nil {
		t.Fatal(err)
	}
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		seq, err := q.NextInvoiceSeq(ctx, gen.NextInvoiceSeqParams{VenueID: 1, Series: code, FyStart: 2026})
		if err != nil {
			return err
		}
		_, err = q.CreateInvoice(ctx, gen.CreateInvoiceParams{
			VenueID: 1, OrderID: 1, Series: code, FyStart: 2026, Seq: seq, InvoiceNo: no, IssuedAt: now,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNumberingOfANewVenue(t *testing.T) {
	s, _ := newService(t)
	got, err := s.Numbering(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := Numbering{
		NumberingInput: stock(),
		Currency:       "INR", NextInvoiceNo: "INV/26-27/000001", NextCreditNo: "CN/26-27/000001",
		NumberLimit: 16, FYLocked: false, MaxRoundingStep: 100,
	}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := s.Numbering(context.Background(), 99); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

func TestUpdateNumberingRefusals(t *testing.T) {
	tests := []struct {
		name  string
		in    NumberingInput
		field string
	}{
		{"lower case codes are made capitals", with(func(in *NumberingInput) { in.InvoiceSeries = "nf" }), ""},
		{"the same series for both", with(func(in *NumberingInput) { in.CreditSeries = "INV" }), "credit_series"},
		{"a series code with a slash", with(func(in *NumberingInput) { in.InvoiceSeries = "A/B" }), "invoice_series"},
		{"a long prefix", with(func(in *NumberingInput) { in.InvoicePrefix = "ABCDEFGHI" }), "invoice_prefix"},
		{"braces in a prefix", with(func(in *NumberingInput) { in.CreditPrefix = "{x}" }), "credit_prefix"},
		{"a template that does not parse", with(func(in *NumberingInput) { in.Template = "{prefix}/{seq" }), "template"},
		{"a template with no year", with(func(in *NumberingInput) { in.Template = "{prefix}-{seq:06d}" }), "template"},
		{"a template with no prefix", with(func(in *NumberingInput) { in.Template = "{fy:short}/{seq:06d}" }), "template"},
		{"a template with two sequences", with(func(in *NumberingInput) { in.Template = "{prefix}/{seq}/{seq}" }), "template"},
		{"a number too long for GST", with(func(in *NumberingInput) { in.Template = "{prefix}/{fy}/{seq:06d}" }), "template"},
		{"one prefix for both", with(func(in *NumberingInput) { in.InvoicePrefix, in.CreditPrefix = "X", "X" }), "credit_prefix"},
		{"month 13", with(func(in *NumberingInput) { in.FinancialYearStartMonth = 13 }), "financial_year_start_month"},
		{"month 0", with(func(in *NumberingInput) { in.FinancialYearStartMonth = 0 }), "financial_year_start_month"},
		{"a rounding step of 0", with(func(in *NumberingInput) { in.CashRoundingStep = 0 }), "cash_rounding_step"},
		{"a step over a rupee", with(func(in *NumberingInput) { in.CashRoundingStep = 101 }), "cash_rounding_step"},
		{"a step of a rupee", with(func(in *NumberingInput) { in.CashRoundingStep = 100 }), ""},
		{"no rounding", with(func(in *NumberingInput) { in.CashRoundingStep = 1 }), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			ctx := context.Background()
			err := s.UpdateNumbering(ctx, 1, tt.in)
			var inv *Invalid
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
			if tt.field == "" {
				return
			}
			got, err := s.Numbering(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got.NumberingInput != stock() {
				t.Fatalf("a refused save was kept: %+v", got.NumberingInput)
			}
		})
	}
}

func TestDefaultsAreNotStoredSoPrefixesKeepFollowingTheirCode(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdateNumbering(ctx, 1, base()); err != nil {
		t.Fatal(err)
	}
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range o.Entries {
		if e.Set {
			t.Errorf("%s was stored though it is the default", e.Key)
		}
	}

	// Renaming the series with no prefix given: the prefix comes along.
	if err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.InvoiceSeries = "NF" })); err != nil {
		t.Fatal(err)
	}
	got, err := s.Numbering(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.InvoicePrefix != "NF" || got.NextInvoiceNo != "NF/26-27/000001" {
		t.Fatalf("%+v", got)
	}
}

func TestNumberingAfterNumbersAreIssued(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	issue(t, d, "INV", "INV/26-27/000001")

	got, err := s.Numbering(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.FYLocked || got.NextInvoiceNo != "INV/26-27/000002" {
		t.Fatalf("%+v", got)
	}

	// The financial year start is fixed now.
	err = s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.FinancialYearStartMonth = 1 }))
	var inv *Invalid
	if !errors.Is(err, ErrNumberingLocked) || !errors.As(err, &inv) || inv.Field != "financial_year_start_month" {
		t.Fatalf("moving the financial year: %v", err)
	}
	if got, _ = s.Numbering(ctx, 1); got.FinancialYearStartMonth != 4 {
		t.Fatal("the refused year was kept")
	}

	// A prefix change continues the same counter.
	if err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.InvoicePrefix = "AB" })); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Numbering(ctx, 1); got.NextInvoiceNo != "AB/26-27/000002" {
		t.Fatalf("after a prefix change: %s", got.NextInvoiceNo)
	}

	// So does a template change, and the rounding changes freely.
	err = s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) {
		in.Template, in.CashRoundingStep = "{prefix}-{fy:short}-{seq:04d}", 1
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Numbering(ctx, 1); got.NextInvoiceNo != "INV-26-27-0002" || got.CashRoundingStep != 1 {
		t.Fatalf("after a template change: %+v", got)
	}
}

func TestANewSeriesMustNotRepeatAnIssuedNumber(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	issue(t, d, "INV", "INV/26-27/000001")

	// A new code starts at 1. Keeping the old prefix would write the first
	// number a second time.
	err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.InvoiceSeries, in.InvoicePrefix = "NF", "INV" }))
	var inv *Invalid
	if !errors.Is(err, ErrNumberTaken) || !errors.As(err, &inv) || inv.Field != "invoice_prefix" {
		t.Fatalf("a renamed series on the old prefix: %v", err)
	}
	if got, _ := s.Numbering(ctx, 1); got.InvoiceSeries != "INV" {
		t.Fatalf("the refused series was kept: %+v", got)
	}

	// With its own prefix it is a new run at 1.
	if err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.InvoiceSeries = "NF" })); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Numbering(ctx, 1); got.NextInvoiceNo != "NF/26-27/000001" {
		t.Fatalf("%s", got.NextInvoiceNo)
	}

	// Going back to INV continues INV's own counter: its number 1 is used.
	if err := s.UpdateNumbering(ctx, 1, base()); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Numbering(ctx, 1); got.NextInvoiceNo != "INV/26-27/000002" {
		t.Fatalf("back to INV: %s", got.NextInvoiceNo)
	}
}

func TestACreditSeriesMustNotRepeatAnIssuedNumber(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	issue(t, d, "INV", "INV/26-27/000001")
	if _, err := d.Writer().ExecContext(ctx, `INSERT INTO invoice_series (venue_id, series, fy_start, last_seq) VALUES (1, 'CN', 2026, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO credit_notes (venue_id, order_id, series, fy_start, seq, credit_no, against_invoice_no, total_minor, issued_at)
		 VALUES (1, 1, 'CN', 2026, 1, 'CN/26-27/000001', 'INV/26-27/000001', 100, ?)`, now); err != nil {
		t.Fatal(err)
	}
	err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.CreditSeries, in.CreditPrefix = "CR", "CN" }))
	var inv *Invalid
	if !errors.Is(err, ErrNumberTaken) || !errors.As(err, &inv) || inv.Field != "credit_prefix" {
		t.Fatalf("a renamed credit series on the old prefix: %v", err)
	}
}

func TestTheFinancialYearCanMoveBeforeAnythingIsIssued(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.FinancialYearStartMonth = 1 })); err != nil {
		t.Fatal(err)
	}
	got, err := s.Numbering(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.FinancialYearStartMonth != 1 || got.NextInvoiceNo != "INV/26/000001" {
		t.Fatalf("%+v", got)
	}
}

func TestNumberingKeepsTheRestOfTheVenue(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if err := s.UpdateNumbering(ctx, 1, with(func(in *NumberingInput) { in.CashRoundingStep = 10 })); err != nil {
		t.Fatal(err)
	}
	v, err := d.Queries().VenueByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Cafe" || v.TaxProfile != "gst_in" || v.TaxRegistrationID != gstin || v.Timezone != "Asia/Kolkata" || v.CashRoundingStepMinor != 10 {
		t.Fatalf("%+v", v)
	}
	if err := s.UpdateNumbering(ctx, 99, base()); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}
