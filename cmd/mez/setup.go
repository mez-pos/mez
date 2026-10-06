package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/logging"
	"github.com/byjoellobo/mez/internal/service/firstrun"
)

// generatedPINDigits is how long a PIN this command draws is. Six rather
// than four, because nobody chose it and nobody will remember it: it is
// meant to be changed in the back office on the first day.
const generatedPINDigits = 6

// ErrAlreadySetUp means the database already has a venue. Setting up twice
// would leave two venues on a machine that serves one.
var ErrAlreadySetUp = firstrun.ErrAlreadySetUp

// setupOptions is everything `mez setup` takes.
// envOwnerPIN is the environment variable setup reads the owner's PIN from
// when -owner-pin is not given, so the PIN need not be typed on a command
// line that the shell remembers.
const envOwnerPIN = "MEZ_OWNER_PIN"

type setupOptions struct {
	DataDir string

	VenueName       string
	LegalName       string
	Currency        string
	Timezone        string
	FYStartMonth    int
	TaxProfile      string
	TaxRegistration string
	TaxPricing      string

	OwnerName string
	OwnerPIN  string

	DeviceName string
	DeviceKind auth.DeviceKind

	// DemoMenu makes the venue a demo cafe, for trying mez out: demoMenu, the
	// demoFloor of twelve tables, and a demoStaff member for each role. The
	// flag keeps the name it had when the menu was all it added.
	DemoMenu bool
}

// setupResult is what setUp created, for the summary printed afterwards.
type setupResult struct {
	VenuePublicID string
	OwnerPublicID string
	OwnerPIN      string
	PINGenerated  bool
	Code          auth.PairingCode
	DBPath        string
	MenuItems     int
	Tables        int
	DemoStaff     []demoPerson

	// Device and Zone are for the closing lines: which app takes the code,
	// and the venue's clock the code's expiry is read on.
	Device auth.DeviceKind
	Zone   *time.Location
}

// runSetup is `mez setup`: the one command run on the venue's own
// machine before anything else works.
//
// It exists because the API cannot bootstrap itself. Making a pairing code
// needs a permission, a permission needs a signed-in staff member, and
// signing in needs a paired device. Something outside that circle has to
// create the first venue, the first owner and the first code, and being sat
// at the machine is the trust that lets this do it.
func runSetup(args []string, stdout io.Writer) error {
	opts, err := parseSetupFlags(args, stdout)
	if err != nil {
		return err
	}

	// Only the flags the server shares are passed on, so the data directory
	// resolves through config.toml and the environment exactly as it does
	// when serving.
	var shared []string
	if opts.DataDir != "" {
		shared = append(shared, "-data-dir", opts.DataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	if opts.Timezone == "" {
		opts.Timezone = cfg.Timezone.String()
	}

	ctx := context.Background()
	log := logging.Bootstrap()
	// Made here rather than by db.Open so that it is restricted before the
	// database is in it (ADR-259): the venue's data is for the account
	// running setup and the administrators, not every user on the computer.
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("creating data directory %s: %w", cfg.DataDir, err)
	}
	restrict(log, "data directory", cfg.DataDir)
	database, err := db.Open(ctx, db.Options{Path: filepath.Join(cfg.DataDir, db.FileName), Log: log})
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(context.Background()); err != nil {
			log.Warn("closing the database", "error", err)
		}
	}()
	if err := database.Migrate(ctx); err != nil {
		return err
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: database, Log: log})
	if err != nil {
		return err
	}

	res, err := setUp(ctx, database, devices, opts, time.Now())
	if err != nil {
		return err
	}
	res.DBPath = database.Path()
	writeSetupSummary(stdout, res)
	return nil
}

// parseSetupFlags reads the command line for setup.
func parseSetupFlags(args []string, out io.Writer) (setupOptions, error) {
	var o setupOptions
	set := flag.NewFlagSet("mez setup", flag.ContinueOnError)
	set.SetOutput(out)

	set.StringVar(&o.DataDir, "data-dir", "", "directory holding the database (default: as the server resolves it)")
	set.StringVar(&o.VenueName, "name", "", "the venue's name, as staff and guests see it (required)")
	set.StringVar(&o.LegalName, "legal-name", "", "the registered name for invoices, if it differs")
	set.StringVar(&o.Currency, "currency", "INR", "ISO 4217 code: INR, GBP, USD")
	set.StringVar(&o.Timezone, "timezone", "", "IANA timezone, for example Asia/Kolkata")
	set.IntVar(&o.FYStartMonth, "fy-start-month", 4, "month the financial year starts: 4 in India, 1 in much of the world")
	set.StringVar(&o.TaxProfile, "tax-profile", "none", "none, gst_in, vat_uk or sales_tax_us")
	set.StringVar(&o.TaxRegistration, "tax-registration", "", "GSTIN or VAT number, required by those profiles")
	set.StringVar(&o.TaxPricing, "tax-pricing", "", "inclusive or exclusive (default: the profile's own)")
	set.StringVar(&o.OwnerName, "owner", "Owner", "the owner's name")
	set.StringVar(&o.OwnerPIN, "owner-pin", "", "the owner's PIN, 6 to 8 digits (default: MEZ_OWNER_PIN, or one is drawn and printed)")
	set.StringVar(&o.DeviceName, "device-name", "Counter 1", "what the first device will be called")
	kind := set.String("device-kind", string(auth.KindTerminal), "terminal, kitchen or back_office")
	set.BoolVar(&o.DemoMenu, "demo-menu", false, "make this a demo cafe: a menu priced in rupees, twelve tables and a person for each role (public PINs: never for a real venue)")

	if err := set.Parse(args); err != nil {
		return setupOptions{}, err
	}
	if set.NArg() > 0 {
		return setupOptions{}, fmt.Errorf("setup: unexpected argument %q", set.Arg(0))
	}
	// A PIN on the command line is in the shell's history and in the process
	// list while setup runs. The environment is neither, so it is offered
	// too; drawing one is better than both.
	if o.OwnerPIN == "" {
		o.OwnerPIN = os.Getenv(envOwnerPIN)
	}
	// Trimmed once, here, so every check and every write below sees the
	// same value: a GSTIN with a leading space must not pass a length check
	// and then yield a state code of " 2".
	o.TaxRegistration = strings.TrimSpace(o.TaxRegistration)
	o.DeviceKind = auth.DeviceKind(*kind)
	return o, nil
}

// setUp makes the venue through firstrun, the one place a venue is made
// (ADR-266), adding what only this command has: a PIN drawn when none was
// given, and the demo cafe.
func setUp(ctx context.Context, database *db.DB, devices *auth.Devices, o setupOptions, now time.Time) (setupResult, error) {
	res := setupResult{Device: o.DeviceKind, OwnerPIN: o.OwnerPIN}
	if res.OwnerPIN == "" {
		pin, err := drawPIN()
		if err != nil {
			return setupResult{}, fmt.Errorf("setup: %w", err)
		}
		res.OwnerPIN, res.PINGenerated = pin, true
	}
	answers, err := firstrun.Check(firstrun.Answers{
		VenueName: o.VenueName, LegalName: o.LegalName, Currency: o.Currency, Timezone: o.Timezone,
		FYStartMonth: o.FYStartMonth, TaxProfile: o.TaxProfile, TaxRegistration: o.TaxRegistration,
		TaxPricing: o.TaxPricing, OwnerName: o.OwnerName, OwnerPIN: res.OwnerPIN,
		DeviceName: o.DeviceName, DeviceKind: o.DeviceKind,
	})
	if err != nil {
		return setupResult{}, setupError(err)
	}
	if o.DemoMenu && answers.Currency != demoMenuCurrency {
		return setupResult{}, fmt.Errorf("setup: the demo menu is priced in %s, and this venue is in %s: leave out -demo-menu", demoMenuCurrency, answers.Currency)
	}
	// Check has already loaded it, so this cannot fail.
	if res.Zone, err = time.LoadLocation(answers.Timezone); err != nil {
		return setupResult{}, fmt.Errorf("setup: %w", err)
	}

	var extras []firstrun.Extra
	if o.DemoMenu {
		extras = append(extras, func(ctx context.Context, q *gen.Queries, venueID int64, roles map[string]int64, now time.Time) error {
			spec := demoMenu()
			if _, err := db.WriteMenu(ctx, q, venueID, spec, now); err != nil {
				return fmt.Errorf("setup: the demo menu: %w", err)
			}
			res.MenuItems = countItems(spec)
			var err error
			if res.DemoStaff, err = writeDemoVenue(ctx, q, venueID, roles, now); err != nil {
				return fmt.Errorf("setup: the demo venue: %w", err)
			}
			res.Tables = countTables(demoFloor)
			return nil
		})
	}

	first, err := firstrun.New(database, devices, fixedClock(now))
	if err != nil {
		return setupResult{}, err
	}
	made, err := first.Run(ctx, answers, extras...)
	if err != nil {
		return setupResult{}, setupError(err)
	}
	res.VenuePublicID, res.OwnerPublicID, res.Code = made.VenuePublicID, made.OwnerPublicID, made.Code
	return res, nil
}

// fixedClock is the moment setup was run, as the clock firstrun reads.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// setupFlags names the flag behind each answer firstrun can refuse, so a
// refusal at a terminal says what to type.
var setupFlags = map[string]string{
	"name": "-name", "currency": "-currency", "timezone": "-timezone (or set MEZ_TIMEZONE)",
	"fy_start_month": "-fy-start-month", "tax_profile": "-tax-profile", "tax_registration": "-tax-registration",
	"tax_pricing": "-tax-pricing", "owner_name": "-owner", "owner_pin": "-owner-pin",
	"device_name": "-device-name", "device_kind": "-device-kind",
}

// setupError puts firstrun's refusals into this command's words.
func setupError(err error) error {
	var invalid *firstrun.Invalid
	if errors.As(err, &invalid) {
		if flag, ok := setupFlags[invalid.Field]; ok {
			return fmt.Errorf("setup: %w: pass %s", err, flag)
		}
	}
	return fmt.Errorf("setup: %w", err)
}

// drawPIN draws a PIN from crypto/rand. Every digit is equally likely, and
// the leading one may be zero: a PIN is typed, not read as a number.
func drawPIN() (string, error) {
	var b strings.Builder
	for range generatedPINDigits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("drawing a PIN: %w", err)
		}
		fmt.Fprintf(&b, "%d", n.Int64())
	}
	return b.String(), nil
}

// writeSetupSummary prints what an operator needs to carry to the till. It
// is the only place the owner's PIN is ever shown, and only when this
// command drew it.
func writeSetupSummary(w io.Writer, res setupResult) {
	fmt.Fprintf(w, "\nmez is set up.\n\n")
	fmt.Fprintf(w, "  Database    %s\n", res.DBPath)
	fmt.Fprintf(w, "  Venue       %s\n", res.VenuePublicID)
	fmt.Fprintf(w, "  Owner       %s\n", res.OwnerPublicID)
	if res.PINGenerated {
		fmt.Fprintf(w, "  Owner PIN   %s   (drawn now: write it down, then change it in the back office)\n", res.OwnerPIN)
	}
	fmt.Fprintf(w, "  Pairing code %s   (expires %s)\n", res.Code.Code, expiryIn(res.Code, res.Zone))
	if res.MenuItems > 0 {
		fmt.Fprintf(w, "  Demo menu   %d items\n", res.MenuItems)
		fmt.Fprintf(w, "  Demo floor  %d tables\n", res.Tables)
		writeDemoStaff(w, res.DemoStaff)
	}
	fmt.Fprintf(w, "\nStart the server with `mez`, then %s and enter the pairing code.\n", openHint(res.Device))
	fmt.Fprintf(w, "Sign in as the owner to make more codes, or run `mez pair` at this machine.\n\n")
}

// writeDemoStaff lists the demo venue's people and the PINs drawn for them.
// This is the one time they are shown, as with the owner's.
func writeDemoStaff(w io.Writer, people []demoPerson) {
	if len(people) == 0 {
		return
	}
	fmt.Fprintf(w, "  Demo staff  (PINs drawn now and shown once: write them down, or reset them in the back office)\n")
	for _, p := range people {
		fmt.Fprintf(w, "    %-9s %-8s PIN %s\n", p.Name, p.Role, p.PIN)
	}
}

// expiryIn is when a code stops working, on the venue's clock. The machine's
// own zone is not it: a Docker container is on UTC.
func expiryIn(code auth.PairingCode, zone *time.Location) string {
	if zone == nil {
		zone = time.Local
	}
	return code.ExpiresAt.In(zone).Format("15:04 MST")
}

// openHint says where a paired device's app is, by the kind of device.
func openHint(kind auth.DeviceKind) string {
	switch kind {
	case auth.KindBackOffice:
		return "open the back office (/backoffice/ on this machine's address)"
	case auth.KindKitchen:
		return "open the kitchen display (/kitchen on this machine's address)"
	default:
		return "open the terminal on the till"
	}
}

// countItems is how many items a menu spec adds, for the summary.
func countItems(spec db.MenuSpec) int {
	n := 0
	for _, c := range spec.Categories {
		n += len(c.Items)
	}
	return n
}
