package firstrun

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var at = time.Date(2026, 10, 7, 4, 30, 0, 0, time.UTC)

type fixed time.Time

func (f fixed) Now() time.Time { return time.Time(f) }

// answers are what an owner in Bengaluru would give.
func answers() Answers {
	return Answers{
		VenueName: "Filter Coffee House", Currency: "INR", Timezone: "Asia/Kolkata", FYStartMonth: 4,
		TaxProfile: TaxGSTIndia, TaxRegistration: "29ABCDE1234F1Z5",
		OwnerName: "Priya", OwnerPIN: "246813",
		DeviceName: "Counter 1", DeviceKind: auth.KindTerminal,
	}
}

func emptyDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return d
}

func service(t *testing.T, d *db.DB) (*Service, *auth.Devices) {
	t.Helper()
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatalf("NewDevices: %v", err)
	}
	s, err := New(d, devices, fixed(at))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, devices
}

func venueCount(t *testing.T, d *db.DB) int {
	t.Helper()
	venues, err := d.Queries().ListVenues(context.Background())
	if err != nil {
		t.Fatalf("ListVenues: %v", err)
	}
	return len(venues)
}

// TestRunMakesAVenueThatCanSell: what Run answers with has to work at a
// till. The code pairs a device and the owner signs in on it with the PIN
// given, holding every permission.
func TestRunMakesAVenueThatCanSell(t *testing.T) {
	ctx := context.Background()
	d := emptyDB(t)
	s, devices := service(t, d)

	if needed, err := s.Needed(ctx); err != nil || !needed {
		t.Fatalf("Needed on an empty database = %v, %v", needed, err)
	}
	res, err := s.Run(ctx, answers())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if needed, err := s.Needed(ctx); err != nil || needed {
		t.Errorf("Needed after Run = %v, %v", needed, err)
	}

	venue, err := d.Queries().VenueByID(ctx, res.VenueID)
	if err != nil {
		t.Fatalf("VenueByID: %v", err)
	}
	if venue.PublicID != res.VenuePublicID || venue.Name != "Filter Coffee House" ||
		venue.LegalName != "Filter Coffee House" || venue.Currency != "INR" || venue.Timezone != "Asia/Kolkata" ||
		venue.FinancialYearStartMonth != 4 || venue.TaxProfile != TaxGSTIndia ||
		venue.TaxRegistrationID != "29ABCDE1234F1Z5" || !venue.CreatedAt.Equal(at) {
		t.Errorf("venue = %+v", venue)
	}
	// The GST state code comes from the GSTIN and is not asked for.
	if state, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: res.VenueID, Key: "tax.state_code"}); err != nil || state.Value != "29" {
		t.Errorf("state code = %+v, %v", state, err)
	}
	if roles, err := d.Queries().RolesForVenue(ctx, res.VenueID); err != nil || len(roles) != len(auth.DefaultRoles()) {
		t.Errorf("%d roles (%v), want %d", len(roles), err, len(auth.DefaultRoles()))
	}

	paired, err := devices.Redeem(ctx, res.Code.Code)
	if err != nil {
		t.Fatalf("redeeming the code: %v", err)
	}
	if paired.Device.Name != "Counter 1" || paired.Device.Kind != auth.KindTerminal {
		t.Errorf("paired = %+v", paired.Device)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	in, err := staff.SignIn(ctx, paired.Device, res.OwnerPublicID, "246813")
	if err != nil {
		t.Fatalf("the owner cannot sign in: %v", err)
	}
	held, err := staff.Permissions(ctx, in.Session.Staff.RoleID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range auth.Permissions() {
		if !held.Has(p) {
			t.Errorf("the owner cannot %s", p)
		}
	}
	// The PIN is not stored as itself.
	row, err := d.Queries().StaffByPublicID(ctx, res.OwnerPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(row.PinHash, "246813") || !strings.HasPrefix(row.PinHash, "$argon2id$") {
		t.Errorf("pin_hash = %q", row.PinHash)
	}
}

func TestRunRefusesASecondVenue(t *testing.T) {
	ctx := context.Background()
	d := emptyDB(t)
	s, _ := service(t, d)
	if _, err := s.Run(ctx, answers()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Refused for being set up, before the answers are looked at: a second
	// caller is not told what was wrong with a form nobody will use.
	bad := answers()
	bad.VenueName = ""
	if _, err := s.Run(ctx, bad); !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("second run: err = %v, want ErrAlreadySetUp", err)
	}
	if n := venueCount(t, d); n != 1 {
		t.Errorf("%d venues, want 1", n)
	}
}

// TestRunRacing: the check that no venue exists is inside the transaction,
// so of several first runs at once exactly one makes a venue.
func TestRunRacing(t *testing.T) {
	d := emptyDB(t)
	s, _ := service(t, d)

	const runners = 6
	errs := make([]error, runners)
	var wg sync.WaitGroup
	for i := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.Run(context.Background(), answers())
		}()
	}
	wg.Wait()

	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, ErrAlreadySetUp):
			t.Errorf("a losing run: %v, want ErrAlreadySetUp", err)
		}
	}
	if won != 1 || venueCount(t, d) != 1 {
		t.Errorf("%d runs won and %d venues exist, want 1 and 1", won, venueCount(t, d))
	}
}

func TestCheckRefusals(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*Answers)
		field string
		says  string
	}{
		{"no name", func(a *Answers) { a.VenueName = "   " }, "name", "needs a name"},
		{"a currency that is a word", func(a *Answers) { a.Currency = "rupees" }, "currency", "three letter code"},
		{"a currency of digits", func(a *Answers) { a.Currency = "356" }, "currency", "three letter code"},
		{"no timezone", func(a *Answers) { a.Timezone = "" }, "timezone", "IANA"},
		{"the machine's own zone", func(a *Answers) { a.Timezone = "Local" }, "timezone", "IANA"},
		{"a timezone that does not exist", func(a *Answers) { a.Timezone = "Mars/Olympus" }, "timezone", "Mars/Olympus"},
		{"a financial year starting in month 0", func(a *Answers) { a.FYStartMonth = 0 }, "fy_start_month", "month 0"},
		{"a financial year starting in month 13", func(a *Answers) { a.FYStartMonth = 13 }, "fy_start_month", "month 13"},
		{"no tax profile", func(a *Answers) { a.TaxProfile = "" }, "tax_profile", "tax profile"},
		{"a tax profile that does not exist", func(a *Answers) { a.TaxProfile = "gst_uae" }, "tax_profile", "gst_uae"},
		{"GST with no GSTIN", func(a *Answers) { a.TaxRegistration = "" }, "tax_registration", "GSTIN"},
		{"the composition scheme with no GSTIN", func(a *Answers) { a.TaxProfile, a.TaxRegistration = TaxGSTComposition, "" }, "tax_registration", "GSTIN"},
		{"the composition scheme with a VAT number", func(a *Answers) { a.TaxProfile, a.TaxRegistration = TaxGSTComposition, "GB123456789" }, "tax_registration", "GSTIN"},
		{"GST with a GSTIN padded to length", func(a *Answers) { a.TaxRegistration = " 29ABCDE1234F1Z" }, "tax_registration", "GSTIN"},
		{"a GSTIN that does not begin with a state", func(a *Answers) { a.TaxRegistration = "ABABCDE1234F1Z5" }, "tax_registration", "GSTIN"},
		{"a GSTIN of fifteen bytes that are not fifteen characters", func(a *Answers) { a.TaxRegistration = "é29ABCDE1234F1" }, "tax_registration", "GSTIN"},
		{"a GSTIN with punctuation", func(a *Answers) { a.TaxRegistration = "29ABCDE-234F1Z5" }, "tax_registration", "GSTIN"},
		{"a registration that goes on", func(a *Answers) { a.TaxProfile, a.TaxRegistration = TaxVATUK, strings.Repeat("9", 41) }, "tax_registration", "up to 40"},
		{"a name that opens the drawer", func(a *Answers) { a.VenueName = "Cafe\x1bp\x00\x19" }, "name", "plain text"},
		{"a name that is not text", func(a *Answers) { a.VenueName = "Cafe\xfa" }, "name", "plain text"},
		{"a name that goes on", func(a *Answers) { a.VenueName = strings.Repeat("a", 61) }, "name", "up to 60"},
		{"a legal name with a line break", func(a *Answers) { a.LegalName = "Cafe\nPvt Ltd" }, "legal_name", "plain text"},
		{"a legal name that goes on", func(a *Answers) { a.LegalName = strings.Repeat("a", 121) }, "legal_name", "up to 120"},
		{"an owner with a control character", func(a *Answers) { a.OwnerName = "Pri\x1dya" }, "owner_name", "plain text"},
		{"an owner whose name goes on", func(a *Answers) { a.OwnerName = strings.Repeat("a", 61) }, "owner_name", "up to 60"},
		{"VAT with no number", func(a *Answers) { a.TaxProfile, a.TaxRegistration = TaxVATUK, "  " }, "tax_registration", "VAT number"},
		{"pricing that is neither", func(a *Answers) { a.TaxPricing = "sometimes" }, "tax_pricing", "inclusive or exclusive"},
		{"no owner", func(a *Answers) { a.OwnerName = " " }, "owner_name", "owner needs a name"},
		{"no PIN", func(a *Answers) { a.OwnerPIN = "" }, "owner_pin", "PIN"},
		{"a PIN with a letter", func(a *Answers) { a.OwnerPIN = "12a456" }, "owner_pin", "PIN"},
		{"a PIN of four digits", func(a *Answers) { a.OwnerPIN = "1234" }, "owner_pin", "6 to 8 digits"},
		{"a PIN of nine digits", func(a *Answers) { a.OwnerPIN = "123456789" }, "owner_pin", "PIN"},
		{"a first device that is a guest", func(a *Answers) { a.DeviceKind = auth.KindGuest }, "device_kind", "cannot be a"},
		{"a first device of no kind", func(a *Answers) { a.DeviceKind = "" }, "device_kind", "cannot be a"},
		{"a first device with no name", func(a *Answers) { a.DeviceName = "  " }, "device_name", "first device"},
		{"a first device with a name too long", func(a *Answers) { a.DeviceName = strings.Repeat("x", 200) }, "device_name", "first device"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := answers()
			tt.edit(&a)
			_, err := Check(a)
			var invalid *Invalid
			if !errors.As(err, &invalid) {
				t.Fatalf("Check = %v, want an *Invalid", err)
			}
			if invalid.Field != tt.field || !strings.Contains(invalid.Message, tt.says) {
				t.Errorf("Invalid = %q on %q, want %q on %q", invalid.Message, invalid.Field, tt.says, tt.field)
			}

			// And Run writes nothing for it, so the next attempt is not
			// told the machine is set up.
			d := emptyDB(t)
			s, _ := service(t, d)
			if _, err := s.Run(context.Background(), a); !errors.As(err, &invalid) {
				t.Errorf("Run = %v, want an *Invalid", err)
			}
			if n := venueCount(t, d); n != 0 {
				t.Errorf("%d venues after a refused run", n)
			}
		})
	}
}

func TestCheckTidies(t *testing.T) {
	a := answers()
	a.VenueName, a.LegalName, a.Currency, a.Timezone = "  Filter Coffee House ", "", " gbp ", " Europe/London "
	a.TaxProfile, a.TaxRegistration, a.OwnerName = TaxVATUK, " GB123456789 ", " Priya "
	got, err := Check(a)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.VenueName != "Filter Coffee House" || got.LegalName != "Filter Coffee House" || got.Currency != "GBP" ||
		got.Timezone != "Europe/London" || got.TaxRegistration != "GB123456789" || got.OwnerName != "Priya" {
		t.Errorf("Check = %+v", got)
	}

	// A GSTIN typed in small letters is the same GSTIN.
	gst := answers()
	gst.TaxRegistration = "29abcde1234f1z5"
	if got, err := Check(gst); err != nil || got.TaxRegistration != "29ABCDE1234F1Z5" {
		t.Errorf("GSTIN = %q, %v", got.TaxRegistration, err)
	}
	// The composition scheme keeps the GSTIN, in capitals, and no pricing:
	// nothing is added to a price (ADR-401).
	comp := answers()
	comp.TaxProfile, comp.TaxRegistration, comp.TaxPricing = TaxGSTComposition, "29abcde1234f1z5", PricingExclusive
	if got, err := Check(comp); err != nil || got.TaxRegistration != "29ABCDE1234F1Z5" || got.TaxPricing != "" {
		t.Errorf("composition = %q, pricing %q, %v", got.TaxRegistration, got.TaxPricing, err)
	}
	// Sixty characters is sixty characters, in any script.
	hindi := answers()
	hindi.VenueName = strings.Repeat("क", 60)
	if _, err := Check(hindi); err != nil {
		t.Errorf("a sixty character name in Devanagari: %v", err)
	}

	a.LegalName = " Filter Coffee House Ltd "
	if got, err := Check(a); err != nil || got.LegalName != "Filter Coffee House Ltd" {
		t.Errorf("legal name = %q, %v", got.LegalName, err)
	}
}

func TestRunWritesOnlyTheSettingsItHas(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Answers)
		want map[string]string
	}{
		{"an untaxed venue has none", func(a *Answers) { a.TaxProfile, a.TaxRegistration = TaxNone, "" }, map[string]string{}},
		{"GST keeps its state", func(*Answers) {}, map[string]string{"tax.state_code": "29"}},
		{"pricing is kept when it was chosen", func(a *Answers) { a.TaxPricing = PricingExclusive },
			map[string]string{"tax.state_code": "29", "tax.pricing": "exclusive"}},
		{"US sales tax asks for no number", func(a *Answers) { a.TaxProfile, a.TaxRegistration, a.Currency = TaxSalesTaxUS, "", "USD" },
			map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			d := emptyDB(t)
			s, _ := service(t, d)
			a := answers()
			tt.edit(&a)
			res, err := s.Run(ctx, a)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			rows, err := d.Queries().SettingsForVenue(ctx, res.VenueID)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, row := range rows {
				got[row.Key] = row.Value
			}
			if len(got) != len(tt.want) {
				t.Fatalf("settings = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("settings[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestRunExtras: an extra writes with the venue or not at all.
func TestRunExtras(t *testing.T) {
	t.Run("it is given the venue and the roles", func(t *testing.T) {
		d := emptyDB(t)
		s, _ := service(t, d)
		var venue int64
		var roles map[string]int64
		res, err := s.Run(context.Background(), answers(), func(_ context.Context, _ *gen.Queries, venueID int64, r map[string]int64, now time.Time) error {
			venue, roles = venueID, r
			if !now.Equal(at) {
				t.Errorf("an extra's clock = %v, want %v", now, at)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if venue != res.VenueID || len(roles) != len(auth.DefaultRoles()) || roles["owner"] == 0 {
			t.Errorf("extra saw venue %d and roles %v", venue, roles)
		}
	})

	t.Run("one that fails leaves no venue", func(t *testing.T) {
		d := emptyDB(t)
		s, _ := service(t, d)
		boom := errors.New("the menu would not write")
		_, err := s.Run(context.Background(), answers(), func(context.Context, *gen.Queries, int64, map[string]int64, time.Time) error {
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("Run = %v, want the extra's error", err)
		}
		if n := venueCount(t, d); n != 0 {
			t.Errorf("%d venues after a failed extra", n)
		}
	})
}

// TestStarterMenu: a venue starts with the one station and one course every
// item needs, and no tax class, since a rate is not guessed (ADR-400).
func TestStarterMenu(t *testing.T) {
	d := emptyDB(t)
	s, _ := service(t, d)
	ctx := context.Background()
	res, err := s.Run(ctx, answers(), StarterMenu)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	m, err := d.Menu(ctx, res.VenueID)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(m.Stations) != 1 || m.Stations[0].Code != "kitchen" || m.Stations[0].Name != "Kitchen" {
		t.Errorf("stations = %+v, want a Kitchen", m.Stations)
	}
	if len(m.Courses) != 1 || m.Courses[0].Number != 1 || m.Courses[0].Name != "Main" {
		t.Errorf("courses = %+v, want course 1, Main", m.Courses)
	}
	// Written as the demo menu is, so the version is bumped once, as its is.
	if len(m.Items) != 0 || len(m.Categories) != 0 || m.Version != 2 {
		t.Errorf("menu has %d items, %d categories, version %d; want none and version 2", len(m.Items), len(m.Categories), m.Version)
	}
	classes, err := d.Queries().TaxClassesForVenue(ctx, res.VenueID)
	if err != nil {
		t.Fatalf("TaxClassesForVenue: %v", err)
	}
	if len(classes) != 0 {
		t.Errorf("a new GST venue has %d tax classes, want none: its rates are its own", len(classes))
	}
}

// TestStarterMenuUnderNoTax (ADR-401): a venue whose profile charges no tax
// starts with the one class every item needs, "No tax", exempt; a venue
// that charges tax is left to set its own rates.
func TestStarterMenuUnderNoTax(t *testing.T) {
	tests := []struct {
		profile, registration string
		noTax                 bool
	}{
		{TaxNone, "", true},
		{TaxGSTComposition, "29ABCDE1234F1Z5", true},
		{TaxGSTIndia, "29ABCDE1234F1Z5", false},
		{TaxVATUK, "GB123456789", false},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			d := emptyDB(t)
			s, _ := service(t, d)
			ctx := context.Background()
			a := answers()
			a.TaxProfile, a.TaxRegistration = tt.profile, tt.registration
			if tt.profile == TaxVATUK {
				a.Currency = "GBP"
			}
			res, err := s.Run(ctx, a, StarterMenu)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			classes, err := d.Queries().TaxClassesForVenue(ctx, res.VenueID)
			if err != nil {
				t.Fatalf("TaxClassesForVenue: %v", err)
			}
			switch {
			case !tt.noTax && len(classes) != 0:
				t.Errorf("%d tax classes, want none", len(classes))
			case tt.noTax && (len(classes) != 1 || classes[0].Name != "No tax" || !classes[0].Exempt || classes[0].Rate != 0 || !classes[0].Active):
				t.Errorf("tax classes %+v, want one, No tax, exempt", classes)
			}
		})
	}
}

// noCodes is device pairing that is broken.
type noCodes struct{}

func (noCodes) CreatePairingCode(context.Context, auth.NewCode) (auth.PairingCode, error) {
	return auth.PairingCode{}, errors.New("no entropy")
}

// TestRunSaysWhenOnlyTheCodeFailed: the venue exists, so the caller must be
// told that and not that nothing happened.
func TestRunSaysWhenOnlyTheCodeFailed(t *testing.T) {
	d := emptyDB(t)
	s, err := New(d, noCodes{}, fixed(at))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), answers())
	if !errors.Is(err, ErrNoCode) {
		t.Fatalf("Run = %v, want ErrNoCode", err)
	}
	if res.VenuePublicID == "" || venueCount(t, d) != 1 {
		t.Errorf("result = %+v with %d venues: the venue should exist and be named", res, venueCount(t, d))
	}
}

func TestNewNeedsItsParts(t *testing.T) {
	d := emptyDB(t)
	if _, err := New(nil, noCodes{}, nil); err == nil {
		t.Error("New took no database")
	}
	if _, err := New(d, nil, nil); err == nil {
		t.Error("New took no device pairing")
	}
	if s, err := New(d, noCodes{}, nil); err != nil || s.clock == nil {
		t.Errorf("New with no clock = %v, %v", s, err)
	}
}
