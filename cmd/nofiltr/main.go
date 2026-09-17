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
	"syscall"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/api"
	"github.com/byjoellobo/nofiltr-pos/internal/config"
	"github.com/byjoellobo/nofiltr-pos/internal/logging"
	"github.com/byjoellobo/nofiltr-pos/web"
)

// version is stamped at build time with -ldflags "-X main.version=...".
// See the Makefile. Phase 01 Task 5 adds the commit and the build date.
var version = "dev"

// shutdownTimeout bounds the whole shutdown, not one subsystem of it. Ten
// seconds is long enough to finish a card payment that is already talking to a
// terminal, and short enough that a Windows Service restart does not look hung.
const shutdownTimeout = 10 * time.Second

// readHeaderTimeout keeps a half-open connection from holding a goroutine
// forever. The register is on a venue's own LAN, but the guest ordering page is
// reachable through the relay, so the server is not among friends.
const readHeaderTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		// -h is a question, not a failure. flag has already printed the usage.
		if errors.Is(err, flag.ErrHelp) {
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

	log.Info("starting",
		"version", version,
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

	srv := api.New(cfg, log, api.BuildInfo{Version: version}, api.WithStatic(bundles))

	return serve(ctx, log, listener, srv.Handler(), shutdownTimeout)
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
