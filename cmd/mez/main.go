// Command mez is the mez server. One binary on the venue's own
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
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/device/discover"
	"github.com/byjoellobo/mez/internal/device/escpos"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/fsperm"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/gateway/razorpay"
	"github.com/byjoellobo/mez/internal/gateway/stripe"
	"github.com/byjoellobo/mez/internal/hub"
	"github.com/byjoellobo/mez/internal/logging"
	"github.com/byjoellobo/mez/internal/service/audit"
	"github.com/byjoellobo/mez/internal/service/charges"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
	"github.com/byjoellobo/mez/internal/service/cloudsignin"
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
	"github.com/byjoellobo/mez/internal/service/tablecodes"
	"github.com/byjoellobo/mez/internal/tunnel"
	"github.com/byjoellobo/mez/web"
)

// shutdownTimeout bounds the whole shutdown, not one subsystem of it. Ten
// seconds is long enough to finish a card payment that is already talking to a
// terminal, and short enough that a Windows Service restart does not look hung.
const shutdownTimeout = 10 * time.Second

// readHeaderTimeout keeps a half-open connection from holding a goroutine
// forever. The register is on a venue's own LAN, but the guest ordering page is
// reachable through the relay, so the server is not among friends.
const readHeaderTimeout = 10 * time.Second

// idleTimeout closes a keep-alive connection that has sent nothing for two
// minutes. Browsers hold a pool of them open and a relay holds more, and
// without this each one is a goroutine and a file descriptor for as long as
// the far end feels like keeping it. It is the only whole-connection timeout
// the server sets: a read or write deadline would cut off the WebSocket
// stream and the long downloads, which are meant to go on for hours.
const idleTimeout = 120 * time.Second

func main() {
	// The subcommands are read before the flags: each takes its own, and
	// the server's parser refuses a positional argument.
	if len(os.Args) > 1 {
		commands := map[string]func([]string, io.Writer) error{"setup": runSetup, "setup-link": runSetupLink, "pair": runPair, "table-codes": runTableCodes, "unlock": runUnlock, "reset-pin": runResetPIN, "restore": runRestore, "mirror": runMirror, "service": runService}
		if command, ok := commands[os.Args[1]]; ok {
			if err := command(os.Args[2:], os.Stdout); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return
				}
				logging.Bootstrap().Error(os.Args[1]+" failed", "error", err)
				os.Exit(1)
			}
			return
		}
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], runOptions{stdout: os.Stdout})
	stop()
	if err != nil {
		// -h is a question, not a failure. The flag package's own output is
		// captured so that a bad flag comes back as an error rather than being
		// printed from inside a library, which means the usage text has to be
		// asked for and printed here. This is the one place the server writes
		// to stdout without going through slog: it is an answer to the
		// operator, not a log line.
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stdout, "mez %s\n\nUsage:\n%s\nSubcommands:\n  setup\tcreate the venue, its owner and the first pairing code\n"+
				"  setup-link\tprint the address that opens the setup wizard on this machine\n"+
				"  pair\tmake another pairing code at this machine (-device-kind, -device-name)\n"+
				"  table-codes\tlist each table's code for guest ordering, making any that are missing\n"+
				"  restore\tbring a venue back from its backup, into a data directory with no venue in it (-from, -list)\n"+
				"  mirror\topen the back office over a copy of a venue's backup, to read (-from, -listen-addr)\n"+
				"  service\trun as a Windows service: install, uninstall, start, stop, status\n"+
				"  --test-print ADDRESS\tprint a test page on a receipt printer (-paper 58, -drawer, -save FILE)\n",
				buildInfo().Version, config.Usage())
			return
		}
		logging.Bootstrap().Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// runOptions is what differs between the server a person starts in a
// terminal and the one the Service Control Manager starts.
type runOptions struct {
	// stdout takes the log beside the file. A service has no console and
	// passes io.Discard.
	stdout io.Writer
	// system is the operating system's log, the Windows event log under a
	// service, or nil.
	system logging.SystemLog
}

// run is main with errors instead of exits, so every path below unwinds through
// the same shutdown. It serves until ctx is cancelled: by Ctrl-C in a
// terminal, by a stop from the Service Control Manager under a service.
func run(ctx context.Context, args []string, opts runOptions) (err error) {
	cfg, err := config.LoadArgs(args)
	if err != nil {
		return err
	}

	// The log directory is created by logging.Setup; this is the parent that
	// the database and the backups also want, and failing here names the
	// path rather than letting the database open fail less clearly.
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("creating data directory %s: %w", cfg.DataDir, err)
	}
	// The mode above means nothing on Windows, where the directory inherits
	// an ACL that lets any user read it (ADR-259). Restricted before the
	// logs are opened, so they inherit the restriction; a failure is said
	// once there is a log to say it in, and the server starts regardless.
	restricted := fsperm.Restrict(cfg.DataDir)

	log, logFile, err := logging.Setup(cfg, opts.stdout, logging.WithSystemLog(opts.system))
	if err != nil {
		return err
	}
	if restricted != nil {
		log.Warn("could not restrict the data directory to its owner", "path", cfg.DataDir, "error", restricted)
	}
	restrictConfig(log, runtime.GOOS, binaryDir())

	var running stack

	// Shutdown runs on the way out of every return below, including the
	// failures, and its errors join whatever sent us here. The log file is
	// closed last of all, after shutdown has written its final lines: on
	// the stack it was closed first, and "subsystem closed" went to a
	// closed file.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, running.shutdown(ctx, log))
		if cerr := logFile.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("closing the log: %w", cerr))
		}
	}()

	build := buildInfo()

	// The venue's pairing to mez Cloud, if it has one (ADR-318). It is
	// where the relay is and the key to it, so it is read before anything
	// asks whether there is a relay (ADR-319).
	pairing, err := loadPairing(cfg.DataDir)
	if err != nil {
		return err
	}
	// What the settings file itself says, before the pairing is laid over
	// it: whether it names a relay decides whether the venue may be paired
	// while it runs, and its public_url stands over any pairing's.
	relayInSettings, settingsPublicURL := cfg.RelayEnabled(), cfg.PublicURL
	cfg, paired := withPairing(cfg, pairing)
	link := newVenueLink(settingsPublicURL, pairing, paired)
	switch paired {
	case pairingInUse:
		log.Info("this venue is paired to mez Cloud", "pairing", *pairing, "public_url", cfg.PublicURL)
	case pairingSetAside:
		log.Warn("this venue is paired to mez Cloud, and relay_url is set as well: the settings are what is used, and the pairing is not. Remove relay_url and relay_key to use the pairing",
			"pairing", *pairing)
	}

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
	// Every return between here and serve is a startup failure, and the port
	// must not stay bound through it. After a clean shutdown the server has
	// already closed it, and closing a closed listener is nothing.
	defer func() { _ = listener.Close() }()

	// Where the backup goes, read before the database is opened: a file
	// that names a destination and cannot be read as one stops the start,
	// as a relay that cannot be used does (ADR-019). The owner who wrote
	// it believes the venue is being backed up.
	dbPath := filepath.Join(cfg.DataDir, db.FileName)
	photosDir := filepath.Join(cfg.DataDir, "photos")
	backups, err := backup.NewManager(backup.ManagerOptions{
		DataDir: cfg.DataDir, DBPath: dbPath, PhotosDir: photosDir, Log: log, SyncInterval: cfg.BackupSyncInterval,
		SyncFloor: link.syncFloor,
	})
	if err != nil {
		return err
	}
	// On the stack under the database, so that it closes after it:
	// Litestream's last act is to copy what the database wrote last
	// (ADR-273). It starts further down, once there is a schema to copy.
	running.push("backup", backups.Close)

	// The database opens before anything is served. A file that cannot be
	// opened is a startup failure with a human watching, which is a far better
	// place to find out than the first sale of the day.
	database, err := db.Open(ctx, db.Options{
		Path: dbPath,
		Log:  log,
		// The log is the backup's to fold in while one runs.
		LogKeptBy: backups.Running,
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

	// A destination that cannot be reached does not stop the start, or a
	// sale: the backup says it is failing and keeps trying.
	if err := backups.Start(ctx); err != nil {
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

	// The hub is a subsystem with goroutines of its own, so it shuts down
	// with everything else rather than being left to the process exiting.
	events, err := hub.New(hub.Options{Source: database, Log: log, Origins: cfg.StreamOrigins()})
	if err != nil {
		return err
	}
	running.push("hub", events.Close)

	// A revoked device's stream is cut as it is revoked: a socket opened
	// while it was paired would otherwise carry the venue's every event to
	// it for as long as it stayed open.
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: database, Log: log, OnRevoke: events.Disconnect})
	if err != nil {
		return err
	}

	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: database, Log: log})
	if err != nil {
		return err
	}
	// A venue paired before the cloud could sign anyone in has nobody for
	// it to be (ADR-323).
	ensureCloudOwner(ctx, database, staff, link.inUse(), log)

	// The wizard's way to what `mez setup` does, from a browser on this
	// machine and nowhere else (ADR-266).
	firstRun, err := firstrun.New(database, devices, nil)
	if err != nil {
		return err
	}
	// And what a browser must hold to use it (ADR-270): a token in the data
	// directory, which is not every user's to read.
	setupNeeded, err := firstRun.Needed(ctx)
	if err != nil {
		return err
	}
	setupToken, err := firstrun.OpenToken(cfg.DataDir, setupNeeded)
	if err != nil {
		return err
	}
	if setupNeeded {
		log.Info("this machine has no venue yet: the setup wizard is open to whoever holds the setup token", "hint", "run `mez setup-link` for the address")
		announceSetupLink(opts.stdout, cfg, setupToken)
	}

	// The print queue sends whatever is waiting, including what a crash left
	// half printed, from the moment the server is up. It stops before the
	// database closes: the stack unwinds in reverse.
	// A job that moves is announced on the stream, as an order event is.
	prints, err := printq.New(printq.Options{DB: database, Log: log, OnChange: events.Notify})
	if err != nil {
		return err
	}
	if err := prints.Start(ctx); err != nil {
		return err
	}
	running.push("print queue", prints.Close)

	orders, err := svcorder.New(svcorder.Options{
		DB: database, Log: log, Publisher: publisher(events), Prints: prints, OnChange: events.Notify,
		// Where the owner says guests pay first, they do only if there is a
		// gateway for them to pay through (ADR-225).
		CanCharge: cfg.GatewayEnabled(),
	})
	if err != nil {
		return err
	}

	// It can look for printers: its own LAN and this computer (ADR-268).
	printerSettings, err := printers.New(database, prints, nil, build.Version, printers.WithFinder(&discover.Finder{}))
	if err != nil {
		return err
	}

	venueSettings, err := settings.New(settings.Options{DB: database, OnChange: events.Notify, Backups: backups})
	if err != nil {
		return err
	}

	photos, err := menus.NewPhotoStore(photosDir)
	if err != nil {
		return err
	}
	menuEditor, err := menus.New(menus.Options{DB: database, OnChange: events.Notify, Photos: photos})
	if err != nil {
		return err
	}

	stockBook, err := svcstock.New(database, nil)
	if err != nil {
		return err
	}

	staffBook, err := people.New(database, nil)
	if err != nil {
		return err
	}

	trail, err := audit.New(database, orders)
	if err != nil {
		return err
	}

	rangeReports, err := reports.New(database)
	if err != nil {
		return err
	}

	floorPlan, err := floor.New(floor.Options{DB: database, Orders: orders, OnChange: events.Notify})
	if err != nil {
		return err
	}

	shifts, err := svcshift.New(svcshift.Options{DB: database, Prints: prints, OnChange: events.Notify})
	if err != nil {
		return err
	}

	// The tunnel serves guests by calling the server's handler, and the
	// server reports the tunnel's health, so each needs the other. The
	// server is built first and reads the tunnel through link, which holds
	// whichever tunnel the venue has now: the one it started with, or the
	// one a pairing made while it ran gave it (ADR-320).
	var srv *api.Server
	dial := func(relayURL, key string, backOffice bool) (*tunnel.Client, error) {
		return tunnel.New(tunnel.Options{
			RelayURL: relayURL,
			Key:      key,
			Handler:  srv.Handler(),
			Mount:    web.MountGuest,
			// Off unless the venue wrote remote_backoffice = true or is
			// paired, and refused on a key the relay did not issue for it
			// (ADR-306).
			BackOffice: backOffice,
			Log:        log,
		})
	}

	// Pairing to a mez Cloud account from the back office. Nothing is sent
	// to the cloud until someone types a code.
	exchange, err := cloud.NewClient(cfg.CloudURL)
	if err != nil {
		return err
	}
	pairer, err := cloudpair.New(cloudpair.Options{
		DB: database, DataDir: cfg.DataDir, Version: build.Version, Cloud: exchange, Backups: backups,
		RelayInSettings: relayInSettings, Log: log,
		Owner: func(ctx context.Context, venueID int64, name string) error {
			_, err := staff.PairCloudOwner(ctx, venueID, name)
			return err
		},
		Apply: func(ctx context.Context, p cloud.Pairing) (bool, error) {
			return link.apply(ctx, p, func(p cloud.Pairing) (*tunnel.Client, error) { return dial(p.Relay.URL, p.Relay.Key, true) })
		},
	})
	if err != nil {
		return err
	}

	// The owner, signed in on mez Cloud's word (ADR-323, ADR-324). The key
	// is the pairing's as it stands when it is asked for.
	cloudIn, err := cloudsignin.New(cloudsignin.Options{DB: database, Staff: staff, Key: link.cloudKey, Log: log})
	if err != nil {
		return err
	}

	guestCodes, err := tablecodes.New(database, nil, nil)
	if err != nil {
		return err
	}

	// Guests paying online (ADR-220, ADR-221). With no gateway set, which is
	// the normal state, nothing here exists and nothing calls the internet.
	var payOnline api.Charges // nil with no gateway: no route, and no bill offers it
	if cfg.GatewayEnabled() {
		// A gateway that is set but cannot be used stops the start, as a
		// relay does (ADR-019): the owner believes guests can pay.
		provider, err := newGateway(cfg)
		if err != nil {
			return err
		}
		// With the webhook's secret set, the gateway's calls are taken as
		// hints to ask it now (ADR-224). Without it, the sweep asks.
		var notices gateway.Notified
		if n, ok := provider.(gateway.Notified); ok && cfg.GatewayWebhookSecret != "" {
			notices = n
		}
		online, err := charges.New(charges.Options{
			DB: database, Orders: orders, Log: log, OnChange: events.Notify, Notices: notices,
			Gateways: func(context.Context, int64) (gateway.Gateway, error) { return provider, nil },
		})
		if err != nil {
			return err
		}
		payOnline = online
		running.push("charges", online.StartSweeper(charges.DefaultSweepEvery).Close)
		log.Info("guests can pay online", "gateway", provider.Name())
	}

	srv = api.New(cfg, log, build,
		api.WithStatic(bundles),
		api.WithDBCheck(dbCheck(database)),
		api.WithTunnel(link.status),
		api.WithPairing(link.pairedTo),
		api.WithPublicURL(link.publicURL),
		api.WithCloudPairing(pairer),
		api.WithCloudSignIn(cloudIn),
		api.WithDevices(devices),
		api.WithFirstRun(firstRun),
		api.WithSetupToken(setupToken),
		// Where a second device opens this server, for whoever is pairing one.
		api.WithAddresses(func() ([]discover.Address, error) { return (&discover.Addresser{}).Addresses(listener.Addr().String()) }),
		api.WithStaff(staff),
		api.WithOrders(orders),
		api.WithMenus(database),
		api.WithStream(events),
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
		api.WithGuests(guestCodes, web.MountGuest),
		api.WithTableCodes(guestCodes, cfg.PublicURL),
		api.WithGuestOrders(orders),
		api.WithGuestRequests(orders),
		api.WithCharges(payOnline, cfg.PublicURL),
	)

	if err := remoteNeedsRelay(cfg); err != nil {
		return err
	}
	if cfg.RelayEnabled() {
		// A relay that is set but cannot be used is a setting the owner
		// believes took, so it stops the start (ADR-019). A relay that is
		// merely unreachable stops nothing: Start returns at once and the
		// counter opens whether or not the internet is there.
		guests, err := dial(cfg.RelayURL, cfg.RelayKey, cfg.RemoteBackoffice)
		if err != nil {
			return err
		}
		if guests.BackOffice() {
			log.Info("the back office is reachable through the relay", "public_url", cfg.PublicURL)
		}
		link.connect(guests)
	}
	// Whether or not there is a tunnel now: one may be started later, and
	// is stopped here all the same.
	running.push("tunnel", link.Close)

	// The tunnel is handed the handler as it is. What arrives on the socket
	// is told apart from it here.
	return serve(ctx, log, listener, fromTheNetwork(srv.Handler()), shutdownTimeout)
}

// remoteNeedsRelay stops a venue that asked for the back office from
// outside and set no relay to carry it. Without one the setting does
// nothing, and an owner who wrote it believes it does (ADR-019).
func remoteNeedsRelay(cfg config.Config) error {
	if cfg.RemoteBackoffice && !cfg.RelayEnabled() {
		return errors.New("remote_backoffice is true but relay_url is not set: the back office is reached from outside through a relay, so set relay_url and relay_key, or set remote_backoffice to false")
	}
	return nil
}

// tunnelStatus puts the tunnel client's status into the API's words.
//
// The two packages name the same states, and this is the one place allowed
// to know both, so the mapping is written out and not left to the strings
// happening to match. internal/api must not import internal/tunnel. A venue
// with no relay has no client, and is off.
func tunnelStatus(guests *tunnel.Client) api.TunnelStatus {
	if guests == nil {
		return api.TunnelStatus{State: api.TunnelOff}
	}
	st := guests.Status()
	out := api.TunnelStatus{
		State:      translateTunnelState(st.State),
		Since:      st.Since,
		RetryAt:    st.RetryAt,
		Detail:     st.LastError,
		BackOffice: guests.BackOffice(),
	}
	if s := st.Standing; s != nil {
		out.Cloud = &api.TunnelCloud{State: s.State, PaidUntil: s.PaidUntil, Until: s.Until, BackOffice: s.BackOffice, Reason: s.Reason}
	}
	return out
}

func translateTunnelState(state tunnel.State) api.TunnelState {
	switch state {
	case tunnel.StateConnecting:
		return api.TunnelConnecting
	case tunnel.StateConnected:
		return api.TunnelConnected
	case tunnel.StateWaiting:
		return api.TunnelWaiting
	case tunnel.StateRefused:
		return api.TunnelRefused
	case tunnel.StateNotEntitled:
		return api.TunnelNotEntitled
	case tunnel.StateReplaced:
		return api.TunnelReplaced
	default:
		return api.TunnelStopped
	}
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
		IdleTimeout:       idleTimeout,
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

// newGateway builds the payment gateway the configuration names. It calls
// nobody: keys that are wrong are found out by the first charge. Nothing
// it returns or says carries the secret.
func newGateway(cfg config.Config) (gateway.Gateway, error) {
	switch cfg.Gateway {
	case config.GatewayRazorpay:
		if cfg.GatewayKeyID == "" || cfg.GatewayKeySecret == "" {
			return nil, errors.New("gateway is razorpay, and gateway_key_id and gateway_key_secret are not both set")
		}
		return razorpay.New(razorpay.Options{KeyID: cfg.GatewayKeyID, KeySecret: cfg.GatewayKeySecret, WebhookSecret: cfg.GatewayWebhookSecret})
	case config.GatewayStripe:
		if cfg.GatewayKeySecret == "" {
			return nil, errors.New("gateway is stripe, and gateway_key_secret, its secret key, is not set")
		}
		// Stripe makes no payment page with nowhere to send the guest back
		// to, so without this every charge would be refused.
		if cfg.PublicURL == "" {
			return nil, errors.New("gateway is stripe, and public_url is not set: stripe needs the address to send a guest back to")
		}
		return stripe.New(stripe.Options{SecretKey: cfg.GatewayKeySecret, WebhookSecret: cfg.GatewayWebhookSecret})
	default:
		return nil, fmt.Errorf("gateway %q: this build has no adapter for it", cfg.Gateway)
	}
}
