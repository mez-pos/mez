package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/logging"
)

// ErrNoVenue means there is no venue to pair a device to: setup has not run.
var ErrNoVenue = errors.New("this database has no venue: run `mez setup` first")

// pairOptions is everything `mez pair` takes.
type pairOptions struct {
	DataDir    string
	DeviceName string
	DeviceKind auth.DeviceKind
}

// pairResult is the code pair made, for the line printed after it.
type pairResult struct {
	Code       auth.PairingCode
	DeviceName string
	Device     auth.DeviceKind
	Zone       *time.Location
}

// defaultDeviceNames is what a device is called when -device-name is left
// out: the name of the thing it is, which the owner can change in the back
// office's Devices screen.
var defaultDeviceNames = map[auth.DeviceKind]string{
	auth.KindTerminal:   "Counter",
	auth.KindKitchen:    "Kitchen",
	auth.KindBackOffice: "Back office",
}

// runPair is `mez pair`: a pairing code made at the machine itself.
//
// Setup makes the first code, and after that only a signed-in person with
// the permission can make more. A venue whose first device was a till has no
// way into the back office from there, and a venue whose owner forgot to pair
// one has none at all. Being sat at the machine is the same trust setup
// relies on, so the same door is open here, and no more of it: this makes a
// code and changes no venue data. (It does bring the schema up to date, as
// every start of the server does.)
func runPair(args []string, stdout io.Writer) error {
	opts, err := parsePairFlags(args, stdout)
	if err != nil {
		return err
	}

	var shared []string
	if opts.DataDir != "" {
		shared = append(shared, "-data-dir", opts.DataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}

	// db.Open creates a file that is not there, and a mistyped -data-dir
	// should be told so rather than given an empty database and a venue
	// error that sends the operator looking elsewhere.
	path := filepath.Join(cfg.DataDir, db.FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("pair: there is no database at %s: run `mez setup` first, or pass -data-dir", path)
		}
		return fmt.Errorf("pair: %w", err)
	}

	ctx := context.Background()
	log := logging.Bootstrap()
	database, err := db.Open(ctx, db.Options{Path: path, Log: log})
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

	res, err := pairDevice(ctx, database, devices, opts)
	if err != nil {
		return err
	}
	writePairSummary(stdout, res)
	return nil
}

// parsePairFlags reads the command line for pair.
func parsePairFlags(args []string, out io.Writer) (pairOptions, error) {
	var o pairOptions
	set := flag.NewFlagSet("mez pair", flag.ContinueOnError)
	set.SetOutput(out)

	set.StringVar(&o.DataDir, "data-dir", "", "directory holding the database (default: as the server resolves it)")
	set.StringVar(&o.DeviceName, "device-name", "", "what the device will be called (default: by its kind)")
	kind := set.String("device-kind", string(auth.KindTerminal), "terminal, kitchen or back_office")

	if err := set.Parse(args); err != nil {
		return pairOptions{}, err
	}
	if set.NArg() > 0 {
		return pairOptions{}, fmt.Errorf("pair: unexpected argument %q", set.Arg(0))
	}
	o.DeviceKind = auth.DeviceKind(*kind)
	fallback, known := defaultDeviceNames[o.DeviceKind]
	if !known {
		return pairOptions{}, fmt.Errorf("pair: a device cannot be a %q: use terminal, kitchen or back_office", *kind)
	}
	if o.DeviceName == "" {
		o.DeviceName = fallback
	}
	return o, nil
}

// pairDevice finds the venue and makes a code for a device of it. The code is
// not attributed to anyone: it records no person who did not make it.
func pairDevice(ctx context.Context, database *db.DB, devices *auth.Devices, o pairOptions) (pairResult, error) {
	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		return pairResult{}, fmt.Errorf("pair: reading the venue: %w", err)
	}
	if len(venues) == 0 {
		return pairResult{}, ErrNoVenue
	}
	venue := venues[0]

	zone, err := time.LoadLocation(venue.Timezone)
	if err != nil {
		zone = time.Local
	}
	code, err := devices.CreatePairingCode(ctx, auth.NewCode{
		VenueID: venue.ID, DeviceName: o.DeviceName, DeviceKind: o.DeviceKind,
	})
	if err != nil {
		return pairResult{}, fmt.Errorf("pair: %w", err)
	}
	return pairResult{Code: code, DeviceName: o.DeviceName, Device: o.DeviceKind, Zone: zone}, nil
}

// writePairSummary prints the code and where to enter it.
func writePairSummary(w io.Writer, res pairResult) {
	fmt.Fprintf(w, "  Pairing code %s   for %q (%s), expires %s\n", res.Code.Code, res.DeviceName, res.Device, expiryIn(res.Code, res.Zone))
	fmt.Fprintf(w, "  To use it, %s and enter the code.\n", openHint(res.Device))
}
