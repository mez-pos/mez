package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/fsperm"
	"github.com/byjoellobo/mez/internal/mirror"
	"github.com/byjoellobo/mez/internal/service/audit"
	"github.com/byjoellobo/mez/internal/service/firstrun"
	"github.com/byjoellobo/mez/internal/service/floor"
	"github.com/byjoellobo/mez/internal/service/menus"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	"github.com/byjoellobo/mez/internal/service/people"
	"github.com/byjoellobo/mez/internal/service/printers"
	"github.com/byjoellobo/mez/internal/service/reports"
	"github.com/byjoellobo/mez/internal/service/settings"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
	svcstock "github.com/byjoellobo/mez/internal/service/stock"
	"github.com/byjoellobo/mez/web"
)

// mirrorListenAddr is where a mirror listens unless told: this computer
// only, and not the port a venue's own server takes. A mirror keeps no lock
// on a PIN that outlasts it (ADR-326), so it is not put on a network by
// default.
const mirrorListenAddr = "127.0.0.1:8090"

// mirrorDirPrefix begins the name of every directory a mirror keeps its
// copy in, under the system's temporary directory.
const mirrorDirPrefix = "mez-mirror-"

// leftCopies is the mirror directories already under dir: those of mirrors
// running now, and of any that was killed before it could remove its own.
func leftCopies(dir string) []string {
	found, err := filepath.Glob(filepath.Join(dir, mirrorDirPrefix+"*"))
	if err != nil {
		return nil
	}
	return found
}

// mirrorOptions is everything `mez mirror` takes: where the backup is, as
// `mez restore` takes it, and where to listen.
type mirrorOptions struct {
	restoreOptions
	ListenAddr string
	// StateDir is where the mirror keeps what it remembers between runs.
	StateDir string
}

// stateDir is where this mirror remembers the browsers paired to it and
// the locks wrong PINs set: as told, or mez/mirror under the user's
// configuration directory.
func (o mirrorOptions) stateDir() (string, error) {
	if o.StateDir != "" {
		return filepath.Abs(o.StateDir)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("mirror: this account has no configuration directory to remember paired browsers in; give one with -state-dir: %w", err)
	}
	return filepath.Join(base, "mez", "mirror"), nil
}

// mirrorUp is a mirror that is serving: where, what a browser pairs to it
// with, and how old its copy is.
type mirrorUp struct {
	Addr string
	Code string
	AsOf time.Time
	Dir  string
}

// runMirror is `mez mirror`: the back office over a copy of a venue's
// backup, to read (phase 14 Task 6a, ADR-312, ADR-326). It serves until it
// is interrupted, and keeps nothing afterwards.
func runMirror(args []string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveMirror(ctx, args, stdout, nil)
}

// serveMirror restores, serves until ctx ends, and removes what it restored.
// ready, when given, is told once it is serving.
func serveMirror(ctx context.Context, args []string, stdout io.Writer, ready func(mirrorUp)) (err error) {
	opts, err := parseMirrorFlags(args, stdout)
	if err != nil {
		return err
	}
	from, err := opts.destination(os.Getenv)
	if err != nil {
		return fmt.Errorf("mirror: %w", err)
	}
	// Litestream says what it is doing through Go's default logger, in its
	// own words, in the middle of what this prints.
	slog.SetDefault(quietLog())
	log := slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	generation, err := chooseGeneration(ctx, from, opts.Generation)
	if err != nil {
		return err
	}

	// Bound before anything is fetched: a port in use is the likeliest
	// failure, and a bucket is not read to find it out.
	listener, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("mirror: listening on %s: %w", opts.ListenAddr, err)
	}
	defer func() { _ = listener.Close() }()

	// A directory of the mirror's own, gone when it stops. What is in it
	// is a venue's sales, so it is kept from every other account as a data
	// directory is (ADR-259).
	// A mirror that was killed, not stopped, could not remove its copy.
	// It is not removed here: another mirror may be running on it now.
	for _, left := range leftCopies(os.TempDir()) {
		fmt.Fprintf(stdout, "A mirror that was not stopped cleanly left a copy of a venue at %s\n  Remove it once no mirror is running.\n", left)
	}
	dir, err := os.MkdirTemp("", mirrorDirPrefix)
	if err != nil {
		return fmt.Errorf("mirror: making a directory for the copy: %w", err)
	}
	var running stack
	defer func() {
		closing, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, running.shutdown(closing, log))
		// Said whatever else went wrong: it is the one thing left behind.
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("mirror: the copy in %s could not be removed, and is a copy of the venue's sales: remove it by hand: %w", dir, rmErr))
		}
	}()
	if err := fsperm.Restrict(dir); err != nil {
		fmt.Fprintf(stdout, "  Could not restrict %s to its owner: %v\n", dir, err)
	}

	fmt.Fprintf(stdout, "Reading the backup at %s\n", from.Destination())
	dbPath := filepath.Join(dir, db.FileName)
	if err := backup.RestoreGeneration(ctx, from, generation.Name, dbPath); err != nil {
		return fmt.Errorf("mirror: %w", err)
	}
	photosDir := filepath.Join(dir, "photos")
	if _, err := backup.RestorePhotos(ctx, from, photosDir); err != nil {
		// The numbers are what a mirror is for. A menu without its pictures
		// is still a menu.
		fmt.Fprintf(stdout, "  Not all the menu photos could be read: %v\n", err)
	}

	database, err := db.Open(ctx, db.Options{Path: dbPath, Log: log})
	if err != nil {
		return fmt.Errorf("mirror: opening the copy: %w", err)
	}
	running.push("database", database.Close)
	// The copy may be from an older mez than this one. It is brought up to
	// this build's schema, as a restore is at its first start. It is the
	// last thing written to the copy: who is let in is kept beside it
	// (ADR-327).
	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("mirror: the copy could not be read by this version of mez: %w", err)
	}
	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		return fmt.Errorf("mirror: reading the copy: %w", err)
	}
	if len(venues) != 1 {
		return fmt.Errorf("mirror: the backup holds %d venues, and a mirror shows one", len(venues))
	}
	venue := venues[0]

	// What the mirror remembers of its own, kept where a restart finds it:
	// the browsers paired to it and the locks wrong PINs have set.
	stateDir, err := opts.stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("mirror: making %s: %w", stateDir, err)
	}
	if err := fsperm.Restrict(stateDir); err != nil {
		fmt.Fprintf(stdout, "  Could not restrict %s to its owner: %v\n", stateDir, err)
	}
	keeper, err := mirror.OpenKeeper(mirror.KeeperOptions{Path: filepath.Join(stateDir, venue.PublicID+".json")})
	if err != nil {
		return err
	}

	srv, err := mirrorServer(database, keeper, venue.ID, photosDir, opts.ListenAddr, log, func() api.MirrorInfo {
		return api.MirrorInfo{AsOf: generation.Last}
	})
	if err != nil {
		return err
	}
	// One more browser, paired with a code as any back office is. Those
	// paired before are remembered and need none.
	code, codeEnds, err := keeper.NewCode()
	if err != nil {
		return err
	}
	known := len(keeper.Browsers())

	addr := listener.Addr().String()
	fmt.Fprintf(stdout, `
A mirror of %s, as it was at %s.

  Open          http://%s%s
  Pairing code  %s   (for a browser not paired to this mirror before; good
                until %s, once. Start the mirror again for another)
  Remembered    %d browsers paired before, in %s

Then sign in with a PIN of someone who manages the venue. It is a copy to
read: nothing changed here reaches the venue, and nothing is printed.
Ctrl-C stops it and removes the copy.

`, venue.Name, localTime(generation.Last), addr, web.MountBackoffice, code, codeEnds.Local().Format("15:04"), known, stateDir)
	if ready != nil {
		ready(mirrorUp{Addr: addr, Code: code, AsOf: generation.Last, Dir: dir})
	}
	return serve(ctx, log, listener, srv.Handler(), shutdownTimeout)
}

// mirrorServer is the API over a copy, as a mirror serves it: what the back
// office reads, and nothing that reaches out. The print queue is built and
// never started, so a ticket that was waiting when the backup was cut is
// not printed from here. There is no stream, tunnel, backup or gateway.
//
// Who is let in is the mirror's own affair: door reads people and their
// PINs from the copy and remembers browsers, sessions and wrong PINs in the
// keeper, so the copy is not written (ADR-327).
func mirrorServer(database *db.DB, keeper *mirror.Keeper, venueID int64, photosDir, listenAddr string, log *slog.Logger, info func() api.MirrorInfo) (*api.Server, error) {
	// These two only read here: the door asks them who works at the venue
	// and what its devices were.
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	door, err := mirror.NewAuth(mirror.Options{Keeper: keeper, Staff: staff, Devices: devices, VenueID: venueID, Log: log})
	if err != nil {
		return nil, err
	}
	prints, err := printq.New(printq.Options{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	orders, err := svcorder.New(svcorder.Options{DB: database, Log: log, Prints: prints})
	if err != nil {
		return nil, err
	}
	printerSettings, err := printers.New(database, prints, nil, buildInfo().Version)
	if err != nil {
		return nil, err
	}
	venueSettings, err := settings.New(settings.Options{DB: database})
	if err != nil {
		return nil, err
	}
	photos, err := menus.NewPhotoStore(photosDir)
	if err != nil {
		return nil, err
	}
	menuEditor, err := menus.New(menus.Options{DB: database, Photos: photos})
	if err != nil {
		return nil, err
	}
	stockBook, err := svcstock.New(database, nil)
	if err != nil {
		return nil, err
	}
	staffBook, err := people.New(database, nil)
	if err != nil {
		return nil, err
	}
	trail, err := audit.New(database, orders)
	if err != nil {
		return nil, err
	}
	rangeReports, err := reports.New(database)
	if err != nil {
		return nil, err
	}
	floorPlan, err := floor.New(floor.Options{DB: database, Orders: orders})
	if err != nil {
		return nil, err
	}
	shifts, err := svcshift.New(svcshift.Options{DB: database, Prints: prints})
	if err != nil {
		return nil, err
	}
	bundles, err := web.Bundles()
	if err != nil {
		return nil, err
	}
	// Only so that the back office's first question, whether this machine
	// still needs setting up, is answered no. The wizard's own steps are
	// writes, and refused with the rest.
	firstRun, err := firstrun.New(database, devices, nil)
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		ListenAddr: listenAddr, DataDir: filepath.Dir(photosDir),
		LogLevel: slog.LevelWarn, LogFormat: config.LogFormatText, Timezone: time.Local,
	}
	srv := api.New(cfg, log, buildInfo(),
		api.WithMirror(info),
		api.WithStatic(bundles),
		api.WithDBCheck(dbCheck(database)),
		api.WithDevices(door),
		api.WithFirstRun(firstRun),
		api.WithStaff(door),
		api.WithOrders(orders),
		api.WithMenus(database),
		api.WithPrints(prints),
		api.WithPrinters(printerSettings),
		api.WithSettings(venueSettings),
		api.WithMenuEditor(menuEditor),
		api.WithStock(stockBook),
		api.WithPeople(staffBook),
		api.WithAudit(trail),
		api.WithReports(rangeReports),
		api.WithFloor(floorPlan),
		api.WithShifts(shifts),
		// No WithGuests and no WithTableCodes: guests are not served from a
		// copy, and the codes they scan lead to the venue, not here.
	)
	return srv, nil
}

// parseMirrorFlags reads the command line for mirror.
func parseMirrorFlags(args []string, out io.Writer) (mirrorOptions, error) {
	var o mirrorOptions
	set := flag.NewFlagSet("mez mirror", flag.ContinueOnError)
	set.SetOutput(out)
	set.Usage = func() {
		fmt.Fprint(out, strings.TrimLeft(`
mez mirror opens the back office over a copy of a venue's backup, to read:
the reports, the menu, the stock, as they were when the backup was last
written. Nothing can be changed in it, and the venue does not need to be on.

  mez mirror -from D:\mez-backup
  mez mirror -bucket corner-cafe -endpoint https://ACCOUNT.r2.cloudflarestorage.com -access-key-id KEY
      (with `+restoreSecretEnv+` set to the secret)

`, "\n"))
		set.PrintDefaults()
	}
	set.StringVar(&o.From, "from", "", "the folder the backup is in")
	set.StringVar(&o.ConfigFile, "config", "", "a saved backup.json naming the destination, in place of -from or -bucket")
	set.StringVar(&o.Bucket.Bucket, "bucket", "", "the bucket the backup is in")
	set.StringVar(&o.Bucket.Path, "path", "", "the folder in the bucket (default mez)")
	set.StringVar(&o.Bucket.Endpoint, "endpoint", "", "the service's address, for anything but Amazon S3")
	set.StringVar(&o.Bucket.Region, "region", "", "the bucket's region, for Amazon S3")
	set.StringVar(&o.Bucket.AccessKeyID, "access-key-id", "", "the bucket's access key id; the secret is read from "+restoreSecretEnv)
	set.StringVar(&o.Generation, "generation", "", "show this backup and not the newest, by its name from `mez restore -list`")
	set.StringVar(&o.ListenAddr, "listen-addr", mirrorListenAddr, "address to listen on, host:port")
	set.StringVar(&o.StateDir, "state-dir", "", "where to remember the browsers paired to this mirror and the locks wrong PINs set (default: mez/mirror in your configuration directory)")
	if err := set.Parse(args); err != nil {
		return mirrorOptions{}, err
	}
	if set.NArg() > 0 {
		return mirrorOptions{}, fmt.Errorf("mirror: unexpected argument %q", set.Arg(0))
	}
	if _, _, err := net.SplitHostPort(o.ListenAddr); err != nil {
		return mirrorOptions{}, fmt.Errorf("mirror: -listen-addr %q is not host:port", o.ListenAddr)
	}
	return o, nil
}
