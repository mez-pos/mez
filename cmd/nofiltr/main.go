// Command nofiltr is the Nofiltr POS server. One binary on the venue's own
// computer: HTTP API, the three SPA bundles, the WebSocket hub, the print queue
// and the scheduler.
//
// It must boot on a bare machine with no external services and keep selling
// with no internet connection.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/api"
	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/config"
	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/device/escpos"
	"github.com/byjoellobo/nofiltr-pos/internal/device/printq"
	"github.com/byjoellobo/nofiltr-pos/internal/hub"
	"github.com/byjoellobo/nofiltr-pos/internal/logging"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
	"github.com/byjoellobo/nofiltr-pos/web"
)

// shutdownTimeout bounds the whole shutdown, not one subsystem of it. Ten
// seconds is long enough to finish a card payment that is already talking to a
// terminal, and short enough that a Windows Service restart does not look hung.
const shutdownTimeout = 10 * time.Second

// readHeaderTimeout keeps a half-open connection from holding a goroutine
// forever. The register is on a venue's own LAN, but the guest ordering page is
// reachable through the relay, so the server is not among friends.
const readHeaderTimeout = 10 * time.Second

func main() {
	// One subcommand, read before the flags: setup takes its own, and the
	// server's parser refuses a positional argument.
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		if err := runSetup(os.Args[2:], os.Stdout); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return
			}
			logging.Bootstrap().Error("setup failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// A printer test, before the server's flags: it needs no venue, no
	// database and no server, so an owner can run it on day one.
	if isTestPrint(os.Args[1:]) {
		if err := runTestPrint(os.Args[1:], os.Stdout, time.Now(), escpos.Send); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return
			}
			logging.Bootstrap().Error("test print failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		// -h is a question, not a failure. The flag package's own output is
		// captured so that a bad flag comes back as an error rather than being
		// printed from inside a library, which means the usage text has to be
		// asked for and printed here. This is the one place the server writes
		// to stdout without going through slog: it is an answer to the
		// operator, not a log line.
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stdout, "nofiltr %s\n\nUsage:\n%s\nSubcommands:\n  setup\tcreate the venue, its owner and the first pairing code\n"+
				"  --test-print ADDRESS\tprint a test page on a receipt printer (-paper 58, -drawer, -save FILE)\n",
				buildInfo().Version, config.Usage())
			return
		}
		logging.Bootstrap().Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// run is main with errors instead of exits, so every path below unwinds through
// the same shutdown.
func run() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// The log directory is created by logging.Setup; this is the parent that
	// the database and the backups will also want, and failing here names the
	// path rather than waiting for Phase 02 to fail less clearly.
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("creating data directory %s: %w", cfg.DataDir, err)
	}

	log, logFile, err := logging.Setup(cfg, os.Stdout)
	if err != nil {
		return err
	}

	var running stack
	running.push("logging", func(context.Context) error { return logFile.Close() })

	// Shutdown runs on the way out of every return below, including the
	// failures, and its errors join whatever sent us here.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, running.shutdown(ctx, log))
	}()

	build := buildInfo()

	log.Info("starting",
		"version", build.Version,
		"commit", build.Commit,
		"build_date", build.BuildDate,
		"listen_addr", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
		"log_level", cfg.LogLevel.String(),
		"timezone", cfg.Timezone.String(),
		"relay_enabled", cfg.RelayEnabled(),
	)

	// Bind before announcing anything. A port already in use is the single most
	// likely startup failure on a machine that has the old version still
	// running, and it belongs here, at the top, not inside a goroutine.
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.ListenAddr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The database opens before anything is served. A file that cannot be
	// opened is a startup failure with a human watching, which is a far better
	// place to find out than the first sale of the day.
	database, err := db.Open(ctx, db.Options{
		Path: filepath.Join(cfg.DataDir, db.FileName),
		Log:  log,
	})
	if err != nil {
		return err
	}
	running.push("database", database.Close)

	// Migrate before serving, never alongside it. A half-migrated database
	// serving requests writes rows the next version cannot read, and this is
	// the one moment where failing to start is the correct outcome.
	if err := database.Migrate(ctx); err != nil {
		return err
	}

	bundles, err := web.Bundles()
	if err != nil {
		return err
	}
	if web.Placeholder() {
		// Worth saying out loud. A venue machine showing a paragraph of text
		// instead of a register is a binary built without the frontends, and
		// this line is the difference between a five-minute diagnosis and an
		// afternoon.
		log.Warn("serving placeholder frontends",
			"hint", "rebuild with -tags frontend once web/*/dist exists")
	}

	devices, err := auth.NewDevices(auth.DevicesOptions{DB: database, Log: log})
	if err != nil {
		return err
	}

	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: database, Log: log})
	if err != nil {
		return err
	}

	// The hub is a subsystem with goroutines of its own, so it shuts down
	// with everything else rather than being left to the process exiting.
	events, err := hub.New(hub.Options{Source: database, Log: log, Origins: cfg.StreamOrigins()})
	if err != nil {
		return err
	}
	running.push("hub", events.Close)

	// The print queue sends whatever is waiting, including what a crash left
	// half printed, from the moment the server is up. It stops before the
	// database closes: the stack unwinds in reverse.
	prints, err := printq.New(printq.Options{DB: database, Log: log})
	if err != nil {
		return err
	}
	if err := prints.Start(ctx); err != nil {
		return err
	}
	running.push("print queue", prints.Close)

	orders, err := svcorder.New(svcorder.Options{DB: database, Log: log, Publisher: publisher(events)})
	if err != nil {
		return err
	}

	srv := api.New(cfg, log, build,
		api.WithStatic(bundles),
		api.WithDBCheck(dbCheck(database)),
		api.WithDevices(devices),
		api.WithStaff(staff),
		api.WithOrders(orders),
		api.WithMenus(database),
		api.WithStream(events),
	)

	return serve(ctx, log, listener, srv.Handler(), shutdownTimeout)
}

// publisher adapts the order service's publisher to the hub.
//
// The change itself is not passed on. The hub reads what it sends from the
// database, so all it needs is to be told that a venue has moved: that way
// a client that reconnects and a client that is already connected go down
// the same path, and neither can be sent an event that is not on disk.
func publisher(events *hub.Hub) svcorder.Publisher {
	return svcorder.PublisherFunc(func(_ context.Context, c svcorder.Change) {
		events.Notify(c.VenueID)
	})
}

// dbCheck adapts the database's health to the vocabulary /api/v1/health
// publishes.
//
// The two packages use the same three words, and this is the one place allowed
// to know both, so the mapping is written out rather than left to the strings
// happening to match. internal/db must not import internal/api, and
// internal/api must not import internal/db.
func dbCheck(database *db.DB) func() string {
	return func() string { return translateDBStatus(database.Health().Status) }
}

// translateDBStatus is the mapping itself, split out so it can be tested
// against every status without a database to produce them.
func translateDBStatus(status string) string {
	switch status {
	case db.StatusOK:
		return api.CheckOK
	case db.StatusFailing:
		return api.CheckFailing
	default:
		return api.CheckUnknown
	}
}

// serve runs the HTTP server until ctx is cancelled, then stops accepting and
// lets the requests already in flight finish within timeout.
//
// The drain is the point. A cashier who hits Charge as the machine is being
// restarted must see that payment complete or fail, never vanish.
func serve(ctx context.Context, log *slog.Logger, listener net.Listener, h http.Handler, timeout time.Duration) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		// net/http logs its own accept and TLS errors through a *log.Logger.
		// Routing it into slog keeps one stream instead of two.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serving := make(chan error, 1)
	go func() { serving <- srv.Serve(listener) }()

	log.Info("listening", "addr", listener.Addr().String())

	select {
	case err := <-serving:
		// Serve stopped on its own: the listener died, which is a real failure.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serving: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutdown requested, draining requests", "timeout", timeout)

	// Not ctx: it is already cancelled, and the drain needs its own clock.
	drain, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(drain); err != nil {
		return fmt.Errorf("draining requests after %s: %w", timeout, err)
	}

	// Serve has returned by now; take it so the goroutine does not outlive us.
	<-serving

	log.Info("stopped accepting requests")
	return nil
}
