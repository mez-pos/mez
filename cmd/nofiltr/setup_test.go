package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
)

// setupArgs are the flags a real first run passes, in a temp data directory.
func setupArgs(t *testing.T, extra ...string) (dir string, args []string) {
	t.Helper()
	dir = t.TempDir()
	return dir, append([]string{
		"-data-dir", dir,
		"-name", "Filter Coffee House",
		"-owner", "Priya",
		"-timezone", "Asia/Kolkata",
	}, extra...)
}

// runSetupIn runs the command and returns what it printed.
func runSetupIn(t *testing.T, args []string) string {
	t.Helper()
	var out strings.Builder
	if err := runSetup(args, &out); err != nil {
		t.Fatalf("setup: %v\n%s", err, out.String())
	}
	return out.String()
}

// opened is the database setup wrote, opened again.
func opened(t *testing.T, dir string) *db.DB {
	t.Helper()
	d, err := db.Open(context.Background(), db.Options{Path: filepath.Join(dir, db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	return d
}

// field reads a value out of the printed summary: "  Venue       ven_X".
func field(t *testing.T, out, label string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if after, found := strings.CutPrefix(strings.TrimSpace(line), label); found {
			return strings.Fields(strings.TrimSpace(after))[0]
		}
	}
	t.Fatalf("no %q in the summary:\n%s", label, out)
	return ""
}

// TestSetupMakesAVenueThatCanSellIsTheWholeBootstrap: the command's output
// is carried to a till, so what it prints has to work. The owner it names
// signs in with the PIN it drew, and the code it printed pairs a device.
func TestSetupMakesAVenueThatCanSell(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t, "-tax-profile", "gst_in", "-tax-registration", "29ABCDE1234F1Z5")
	out := runSetupIn(t, args)

	d := opened(t, dir)
	venue, err := d.Queries().VenueByID(ctx, 1)
	if err != nil {
		t.Fatalf("VenueByID: %v", err)
	}
	if venue.Name != "Filter Coffee House" || venue.LegalName != "Filter Coffee House" ||
		venue.Currency != "INR" || venue.Timezone != "Asia/Kolkata" ||
		venue.FinancialYearStartMonth != 4 || venue.TaxProfile != "gst_in" {
		t.Errorf("venue = %+v", venue)
	}
	if venue.PublicID != field(t, out, "Venue") {
		t.Errorf("the summary names %q, the database holds %q", field(t, out, "Venue"), venue.PublicID)
	}

	// The GST state code comes from the GSTIN rather than being asked for.
	setting, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: 1, Key: "tax.state_code"})
	if err != nil || setting.Value != "29" {
		t.Errorf("state code = %+v, %v", setting, err)
	}

	roles, err := d.Queries().RolesForVenue(ctx, 1)
	if err != nil || len(roles) != len(auth.DefaultRoles()) {
		t.Fatalf("%d roles (%v), want %d", len(roles), err, len(auth.DefaultRoles()))
	}

	// The owner signs in with the PIN that was printed, and may do anything.
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(ctx, field(t, out, "Pairing code"))
	if err != nil {
		t.Fatalf("redeeming the printed code: %v", err)
	}
	if paired.Device.Name != "Counter 1" || paired.Device.Kind != auth.KindTerminal {
		t.Errorf("paired = %+v", paired.Device)
	}

	in, err := staff.SignIn(ctx, paired.Device, field(t, out, "Owner"), field(t, out, "Owner PIN"))
	if err != nil {
		t.Fatalf("the owner cannot sign in with the printed PIN: %v", err)
	}
	if in.Session.Staff.Name != "Priya" {
		t.Errorf("signed in as %q", in.Session.Staff.Name)
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
}

func TestSetupRefusesToRunTwice(t *testing.T) {
	dir, args := setupArgs(t)
	runSetupIn(t, args)

	err := runSetup(args, &strings.Builder{})
	if !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("second run: err = %v, want ErrAlreadySetUp", err)
	}

	// And nothing was added by the attempt.
	d := opened(t, dir)
	venues, err := d.Queries().ListVenues(context.Background())
	if err != nil || len(venues) != 1 {
		t.Errorf("%d venues (%v), want 1", len(venues), err)
	}
}

func TestSetupPINs(t *testing.T) {
	t.Run("a drawn PIN is six digits and printed", func(t *testing.T) {
		_, args := setupArgs(t)
		pin := field(t, runSetupIn(t, args), "Owner PIN")
		if len(pin) != generatedPINDigits || strings.Trim(pin, "0123456789") != "" {
			t.Errorf("PIN = %q", pin)
		}
	})

	t.Run("a chosen PIN is never printed", func(t *testing.T) {
		dir, args := setupArgs(t, "-owner-pin", "246813")
		out := runSetupIn(t, args)
		if strings.Contains(out, "246813") || strings.Contains(out, "Owner PIN") {
			t.Errorf("the summary shows a PIN the operator already knows:\n%s", out)
		}
		// It is the PIN that works, and it is not stored as itself.
		d := opened(t, dir)
		staffRow, err := d.Queries().StaffByPublicID(context.Background(), field(t, out, "Owner"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(staffRow.PinHash, "246813") || !strings.HasPrefix(staffRow.PinHash, "$argon2id$") {
			t.Errorf("pin_hash = %q", staffRow.PinHash)
		}
	})
}

func TestSetupRefusals(t *testing.T) {
	tests := []struct {
		name  string
		extra []string
		says  string
	}{
		{"no name", []string{"-name", "   "}, "needs a name"},
		{"a currency that is not a code", []string{"-currency", "rupees"}, "three letter code"},
		{"a timezone that does not exist", []string{"-timezone", "Mars/Olympus"}, "timezone"},
		{"a financial year starting in month 13", []string{"-fy-start-month", "13"}, "month 13"},
		{"a tax profile that does not exist", []string{"-tax-profile", "gst_uae"}, "tax profile"},
		{"GST with no GSTIN", []string{"-tax-profile", "gst_in"}, "GSTIN"},
		{"GST with a GSTIN of the wrong length", []string{"-tax-profile", "gst_in", "-tax-registration", "29ABC"}, "GSTIN"},
		{"VAT with no number", []string{"-tax-profile", "vat_uk"}, "VAT number"},
		{"pricing that is neither", []string{"-tax-pricing", "sometimes"}, "inclusive or exclusive"},
		{"a first device that is a guest", []string{"-device-kind", "guest"}, "cannot be a"},
		{"a PIN with letters in it", []string{"-owner-pin", "12a4"}, "PIN"},
		{"a PIN too short", []string{"-owner-pin", "123"}, "PIN"},
		{"an argument that is not a flag", []string{"today"}, "unexpected argument"},
		{"the rupee demo menu in pounds", []string{"-currency", "GBP", "-demo-menu"}, "demo menu is priced in INR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, args := setupArgs(t, tt.extra...)
			err := runSetup(args, &strings.Builder{})
			if err == nil {
				t.Fatal("setup accepted it")
			}
			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("error = %v, want one mentioning %q", err, tt.says)
			}
			// A refused setup leaves nothing behind to refuse a second run.
			d := opened(t, dir)
			venues, err := d.Queries().ListVenues(context.Background())
			if err == nil && len(venues) != 0 {
				t.Errorf("%d venues after a refused setup", len(venues))
			}
		})
	}
}

func TestSetupDefaults(t *testing.T) {
	dir, args := setupArgs(t, "-legal-name", "Filter Coffee House Pvt Ltd",
		"-currency", "gbp", "-fy-start-month", "1", "-device-name", "Kitchen", "-device-kind", "kitchen")
	out := runSetupIn(t, args)

	d := opened(t, dir)
	venue, err := d.Queries().VenueByID(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if venue.LegalName != "Filter Coffee House Pvt Ltd" || venue.Currency != "GBP" ||
		venue.FinancialYearStartMonth != 1 || venue.TaxProfile != "none" {
		t.Errorf("venue = %+v", venue)
	}
	// An untaxed venue has no tax settings to write.
	settings, err := d.Queries().SettingsForVenue(context.Background(), 1)
	if err != nil || len(settings) != 0 {
		t.Errorf("settings = %+v, %v", settings, err)
	}

	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(context.Background(), field(t, out, "Pairing code"))
	if err != nil {
		t.Fatal(err)
	}
	if paired.Device.Name != "Kitchen" || paired.Device.Kind != auth.KindKitchen {
		t.Errorf("paired = %+v", paired.Device)
	}
}

// TestSetupDemoMenu: the flag leaves a menu the till can ring up from, and
// without it the venue starts with none.
func TestSetupDemoMenu(t *testing.T) {
	dir, args := setupArgs(t, "-demo-menu")
	out := runSetupIn(t, args)
	want := countItems(demoMenu())
	if got := field(t, out, "Demo menu"); got != fmt.Sprint(want) {
		t.Errorf("summary says %s items, want %d", got, want)
	}

	m, err := opened(t, dir).Menu(context.Background(), 1)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(m.Items) != want || len(m.Stations) != 2 || len(m.Courses) != 2 || m.Version != 2 {
		t.Fatalf("menu has %d items, %d stations, %d courses, version %d",
			len(m.Items), len(m.Stations), len(m.Courses), m.Version)
	}
	required, defaults, negative := 0, 0, 0
	for _, g := range m.Groups {
		if g.MinSelect > 0 {
			required++
		}
		for _, md := range g.Modifiers {
			if md.IsDefault {
				defaults++
			}
			if md.Price.Minor < 0 {
				negative++
			}
		}
	}
	if required == 0 || defaults == 0 || negative != 0 {
		t.Errorf("the demo menu should have required groups and defaults, and no negative price: %d %d %d",
			required, defaults, negative)
	}
	for _, it := range m.Items {
		if it.HSN != sacRestaurant || it.TaxRate == 0 || it.Price.Currency != demoMenuCurrency {
			t.Errorf("item %q = %+v", it.Name, it)
		}
	}

	plainDir, plainArgs := setupArgs(t)
	if out := runSetupIn(t, plainArgs); strings.Contains(out, "Demo menu") {
		t.Errorf("a plain setup mentions a demo menu: %q", out)
	}
	if m, err := opened(t, plainDir).Menu(context.Background(), 1); err != nil || len(m.Items) != 0 {
		t.Errorf("a plain setup has a menu: %d items, %v", len(m.Items), err)
	}
}
