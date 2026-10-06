// Command relay is the optional cloud relay that lets a guest's phone reach a
// venue server which has no public address.
//
// It stores nothing. A venue dials wss://relay/agent with a venue key, the
// relay maps venue_id to that open socket, and a guest request is forwarded
// down it and streamed back.
//
// Optional is the important word. A self-hoster can run their own relay, point
// cloudflared at the machine, or skip guest ordering entirely, and the POS is
// fully functional either way. See ADR-008 and ADR-201.
//
//	relay                       serve, configured by the environment
//	relay issue-key VENUE_ID    print a new venue key and exit
//	relay version               print the version and exit
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/byjoellobo/mez/internal/relay"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

// The environment is the whole configuration. A relay runs in a container or
// under a process manager, where the environment is the one thing every one
// of them can set, and a file would be the only thing it kept on disk.
const (
	envListen     = "RELAY_LISTEN_ADDR"  // default ":8080"
	envSecret     = "RELAY_SECRET"       // signs venue keys; required
	envRevoked    = "RELAY_REVOKED"      // comma-separated venue ids and key ids to refuse
	envTrustProxy = "RELAY_TRUST_PROXY"  // "true" when the operator's own proxy sits in front
	envDomain     = "RELAY_VENUE_DOMAIN" // gives each venue the hostname <venue>.<domain>
	envVenueRPM   = "RELAY_VENUE_RPM"    // requests a minute each venue may receive
	envVenueBurst = "RELAY_VENUE_BURST"  // requests each venue may receive at once
	envLogLevel   = "RELAY_LOG_LEVEL"    // debug, info, warn or error

	defaultListen = ":8080"
)

const usage = `relay: the optional public address for mez guest ordering

  relay                       serve
  relay issue-key VENUE_ID    print a new venue key
  relay version               print the version

Environment:
  RELAY_SECRET        signs venue keys; at least 32 characters; required
  RELAY_LISTEN_ADDR   address to listen on (default :8080)
  RELAY_REVOKED       comma-separated venue ids and key ids to refuse
  RELAY_TRUST_PROXY   true when your own proxy sets X-Forwarded-For
  RELAY_VENUE_DOMAIN  serve each venue at <venue>.<this domain>, not /v/<venue>/
  RELAY_VENUE_RPM     requests a minute per venue (default 1200)
  RELAY_VENUE_BURST   requests at once per venue (default 300)
  RELAY_LOG_LEVEL     debug, info, warn or error (default info)

It serves plain HTTP. Put it behind something that terminates TLS.
`

const (
	// readHeaderTimeout keeps a connection that never sends a request from
	// holding a goroutine. Everyone here is a stranger.
	readHeaderTimeout = 10 * time.Second

	// maxHeaderBytes keeps a guest's headers small enough to fit one frame
	// once they are encoded.
	maxHeaderBytes = 32 << 10

	// shutdownTimeout is how long requests in flight get to finish.
	shutdownTimeout = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, rand.Reader); err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("relay failed", "error", err)
		os.Exit(1)
	}
}

// run is main with its inputs passed in and errors instead of exits.
func run(ctx context.Context, args []string, env func(string) string, stdout io.Writer, random io.Reader) error {
	command := ""
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "version":
		_, err := fmt.Fprintln(stdout, version)
		return err
	case "help", "-h", "-help", "--help":
		_, err := io.WriteString(stdout, usage)
		return err
	case "issue-key":
		if len(args) != 2 {
			return errors.New("usage: relay issue-key VENUE_ID")
		}
		keys, err := loadKeys(env)
		if err != nil {
			return err
		}
		key, err := keys.Issue(random, args[1])
		if err != nil {
			return err
		}
		// The one thing this binary writes that is not a log line: it is
		// the answer, and it is piped into a config file.
		_, err = fmt.Fprintln(stdout, key)
		return err
	case "":
		return serve(ctx, env, stdout, nil)
	default:
		return fmt.Errorf("unknown command %q; try relay help", command)
	}
}

func loadKeys(env func(string) string) (*relay.Keys, error) {
	secret := env(envSecret)
	if secret == "" {
		return nil, fmt.Errorf("%s is not set", envSecret)
	}
	return relay.NewKeys(secret, strings.Split(env(envRevoked), ","))
}

// serve runs the relay until ctx ends. listening, when not nil, is told the
// address once the socket is open, which is how a test finds a port it let
// the system choose.
func serve(ctx context.Context, env func(string) string, stdout io.Writer, listening chan<- string) error {
	level, err := logLevel(env(envLogLevel))
	if err != nil {
		return err
	}
	// JSON, always: a relay's log is read by a collector, not at a terminal.
	log := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: utcTime}))

	keys, err := loadKeys(env)
	if err != nil {
		return err
	}
	limit, err := venueLimit(env)
	if err != nil {
		return err
	}
	trustProxy, err := boolean(envTrustProxy, env(envTrustProxy))
	if err != nil {
		return err
	}

	srv, err := relay.New(relay.Options{
		Keys:        keys,
		Limit:       limit,
		TrustProxy:  trustProxy,
		VenueDomain: env(envDomain),
		Version:     version,
		Log:         log,
	})
	if err != nil {
		return err
	}
	addr := env(envListen)
	if addr == "" {
		addr = defaultListen
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// No read or write timeout for the whole request: a guest watching
		// their order holds one open for as long as the kitchen takes. The
		// handler sets a rolling deadline on each read and write instead.
	}

	log.Info("relay: listening",
		"addr", ln.Addr().String(),
		"version", version,
		"trust_proxy", trustProxy,
		"venue_domain", env(envDomain),
		"venue_rpm", limit.PerMinute,
		"venue_burst", limit.Burst)
	if listening != nil {
		listening <- ln.Addr().String()
	}

	served := make(chan error, 1)
	go func() { served <- httpSrv.Serve(ln) }()

	select {
	case err := <-served:
		srv.Close()
		return fmt.Errorf("serving: %w", err)
	case <-ctx.Done():
	}

	log.Info("relay: shutting down")
	// Venues first: their sockets are hijacked, so Shutdown would neither
	// close them nor wait for them, and closing them ends the guest
	// requests riding on them.
	srv.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving: %w", err)
	}
	log.Info("relay: stopped")
	return nil
}

// utcTime writes timestamps in UTC, as the venue server does, so a guest
// request reads the same in both logs wherever the relay happens to run.
func utcTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		a.Value = slog.TimeValue(a.Value.Time().UTC())
	}
	return a
}

func venueLimit(env func(string) string) (relay.Limit, error) {
	limit := relay.DefaultLimit
	for _, setting := range []struct {
		name string
		into *int
	}{{envVenueRPM, &limit.PerMinute}, {envVenueBurst, &limit.Burst}} {
		raw := strings.TrimSpace(env(setting.name))
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return relay.Limit{}, fmt.Errorf("%s %q: must be a whole number above zero", setting.name, raw)
		}
		*setting.into = n
	}
	return limit, nil
}

func boolean(name, raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s %q: must be true or false", name, raw)
	}
	return v, nil
}

func logLevel(raw string) (slog.Level, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return slog.LevelInfo, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("%s %q: must be debug, info, warn or error", envLogLevel, raw)
	}
	return level, nil
}
