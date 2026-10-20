package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var now = time.Date(2026, 10, 2, 6, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

func newService(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_TEST0001", Name: "Cafe", LegalName: "Cafe Pvt Ltd", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 50, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{DB: d, Clock: clock{}})
	if err != nil {
		t.Fatal(err)
	}
	return s, d
}

func TestKeysAreUniqueAndDefaultsPassTheirOwnCheck(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k.Name] {
			t.Errorf("%s is listed twice", k.Name)
		}
		seen[k.Name] = true
		if k.Default != "" {
			if msg := k.Check(k.Default); msg != "" {
				t.Errorf("%s: its default %q is refused: %s", k.Name, k.Default, msg)
			}
		}
	}
}

func TestReadListsEveryKeyWithItsDefault(t *testing.T) {
	s, _ := newService(t)
	o, err := s.Read(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if o.Venue != (Venue{Name: "Cafe", LegalName: "Cafe Pvt Ltd", Currency: "INR", Timezone: "Asia/Kolkata"}) {
		t.Fatalf("venue: %+v", o.Venue)
	}
	if len(o.Entries) != len(keys) {
		t.Fatalf("%d entries for %d keys", len(o.Entries), len(keys))
	}
	for _, e := range o.Entries {
		if e.Set || e.Value != "" {
			t.Errorf("%s reads as set on a new venue: %+v", e.Key, e)
		}
	}
	if _, err := s.Read(context.Background(), 99); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

func TestUpdateDetails(t *testing.T) {
	tests := []struct {
		name  string
		in    Details
		field string // the refused field; "" when saved
	}{
		{"saves", Details{Name: "  Blue Door ", LegalName: "Blue Door LLP", Timezone: "Europe/London"}, ""},
		{"legal name may be empty", Details{Name: "Blue Door", Timezone: "Asia/Kolkata"}, ""},
		{"no name", Details{Name: "  ", Timezone: "Asia/Kolkata"}, "name"},
		{"a long name", Details{Name: string(make([]rune, maxName+1)), Timezone: "Asia/Kolkata"}, "name"},
		{"a long legal name", Details{Name: "A", LegalName: string(make([]rune, maxLegalName+1)), Timezone: "Asia/Kolkata"}, "legal_name"},
		{"no timezone", Details{Name: "A"}, "timezone"},
		{"local is the machine, not a place", Details{Name: "A", Timezone: "Local"}, "timezone"},
		{"not a zone", Details{Name: "A", Timezone: "Mars/Olympus"}, "timezone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			err := s.UpdateDetails(context.Background(), 1, tt.in)
			var inv *Invalid
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
			o, err := s.Read(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if tt.field != "" {
				if o.Venue.Name != "Cafe" || o.Venue.Timezone != "Asia/Kolkata" {
					t.Fatalf("a refused change was saved: %+v", o.Venue)
				}
				return
			}
			if o.Venue.Name != "Blue Door" || o.Venue.Currency != "INR" {
				t.Fatalf("saved: %+v", o.Venue)
			}
		})
	}
}

func TestUpdateDetailsLeavesTheRestOfTheVenueAlone(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if err := s.UpdateDetails(ctx, 1, Details{Name: "Blue Door", Timezone: "Asia/Kolkata"}); err != nil {
		t.Fatal(err)
	}
	v, err := d.Queries().VenueByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.TaxProfile != "gst_in" || v.TaxRegistrationID != "29ABCDE1234F1Z5" || v.CashRoundingStepMinor != 50 ||
		v.FinancialYearStartMonth != 4 || v.Currency != "INR" || !v.UpdatedAt.Equal(now) {
		t.Fatalf("venue after a rename: %+v", v)
	}
	if err := s.UpdateDetails(ctx, 99, Details{Name: "A", Timezone: "Asia/Kolkata"}); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

func TestSetChecksEveryValue(t *testing.T) {
	tests := []struct {
		name, key, value string
		ok               bool
	}{
		{"pricing", "tax.pricing", "exclusive", true},
		{"pricing, a guess", "tax.pricing", "gross", false},
		{"state code", "tax.state_code", "29", true},
		{"state code, a name", "tax.state_code", "KA", false},
		{"state code, three digits", "tax.state_code", "291", false},
		{"jurisdiction", "tax.jurisdiction", `[{"name":"CA","rate":7250}]`, true},
		{"jurisdiction, empty list", "tax.jurisdiction", `[]`, false},
		{"jurisdiction, not json", "tax.jurisdiction", `CA 7.25`, false},
		{"jurisdiction, no name", "tax.jurisdiction", `[{"name":" ","rate":1}]`, false},
		{"series", "invoice.series", "NF2", true},
		{"series, lower case", "invoice.series", "inv", false},
		{"series, too long", "invoice.series", "ABCDEFGHI", false},
		{"prefix", "invoice.prefix", "NF", true},
		{"prefix, braces", "invoice.prefix", "{x}", false},
		{"template", "invoice.template", "{prefix}/{fy:short}/{seq:06d}", true},
		{"template, no seq", "invoice.template", "{prefix}/{fy}", false},
		{"credit series", "credit_note.series", "CR", true},
		{"late minutes", "terminal.ticket_late_minutes", "15", true},
		{"late minutes, zero", "terminal.ticket_late_minutes", "0", false},
		{"late minutes, a word", "terminal.ticket_late_minutes", "soon", false},
		{"late minutes, padded zero", "terminal.ticket_late_minutes", "015", false},
		{"variance", "shift.variance_threshold_minor", "5000", true},
		{"variance, negative", "shift.variance_threshold_minor", "-1", false},
		{"variance, a decimal", "shift.variance_threshold_minor", "50.5", false},
		{"tip rule", "tips.rule", "hours", true},
		{"tip rule, unknown", "tips.rule", "random", false},
		{"role weights", "tips.role_weights", `{"manager":2,"cashier":1}`, true},
		{"role weights, zero", "tips.role_weights", `{"manager":0}`, false},
		{"role weights, not json", "tips.role_weights", `manager=2`, false},
		{"an unknown key", "tax.prcing", "inclusive", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			ctx := context.Background()
			err := s.Set(ctx, 1, map[string]string{tt.key: tt.value})
			var inv *Invalid
			if tt.ok != (err == nil) || (!tt.ok && (!errors.As(err, &inv) || inv.Field != tt.key)) {
				t.Fatalf("Set(%s=%q): %v", tt.key, tt.value, err)
			}
			o, err := s.Read(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range o.Entries {
				if e.Key == tt.key && e.Set != tt.ok {
					t.Fatalf("after Set: %+v", e)
				}
			}
		})
	}
}

func TestSetIsAllOrNothingAndClears(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.Set(ctx, 1, map[string]string{"tips.rule": "hours", "terminal.ticket_late_minutes": " 15 "}); err != nil {
		t.Fatal(err)
	}
	value := func(key string) Entry {
		t.Helper()
		o, err := s.Read(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range o.Entries {
			if e.Key == key {
				return e
			}
		}
		t.Fatalf("no key %s", key)
		return Entry{}
	}
	if e := value("terminal.ticket_late_minutes"); e.Value != "15" || !e.Set || e.Default != "20" {
		t.Fatalf("a value is trimmed: %+v", e)
	}

	// One good value and one bad: neither is saved.
	err := s.Set(ctx, 1, map[string]string{"tips.rule": "even", "terminal.ticket_late_minutes": "0"})
	if err == nil {
		t.Fatal("a bad value was saved")
	}
	if e := value("tips.rule"); e.Value != "hours" {
		t.Fatalf("the good half of a refused save was kept: %+v", e)
	}

	// An empty value puts a key back to its default.
	if err := s.Set(ctx, 1, map[string]string{"tips.rule": ""}); err != nil {
		t.Fatal(err)
	}
	if e := value("tips.rule"); e.Set || e.Value != "" || e.Default != "even" {
		t.Fatalf("cleared: %+v", e)
	}
	if err := s.Set(ctx, 99, map[string]string{"tips.rule": "even"}); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

// TestVenueIdentity: what a paired device may know of the venue is its name,
// for the lock screen, the zone its clock reads in, for the status bar, and
// the tip ceiling, so the payment screen asks for a manager before the
// server has to refuse (ADR-261).
func TestVenueIdentity(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if id, err := s.Identity(ctx, 1); err != nil || id != (Identity{Name: "Cafe", Timezone: "Asia/Kolkata", TipsMaxPercent: 50}) {
		t.Fatalf("%+v, %v", id, err)
	}
	addRoles(t, d, "Manager")
	if err := s.UpdateRules(ctx, 1, rulesWith(func(in *RulesInput) { in.TipsMaxPercent = percent(0) })); err != nil {
		t.Fatal(err)
	}
	if id, err := s.Identity(ctx, 1); err != nil || id.TipsMaxPercent != 0 {
		t.Fatalf("after a ceiling of 0: %+v, %v", id, err)
	}
	if _, err := s.Identity(ctx, 99); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}
