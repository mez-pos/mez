package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/tunnel"
	"github.com/byjoellobo/mez/web"
)

// TestShutdownRunsInReverse is the whole contract. The logger is opened first
// and has to close last, and a database handle opened after it must be gone
// before it.
func TestShutdownRunsInReverse(t *testing.T) {
	var order []string

	var s stack
	for _, name := range []string{"logging", "database", "print queue"} {
		s.push(name, func(context.Context) error {
			order = append(order, name)
			return nil
		})
	}

	if err := s.shutdown(context.Background(), quietLogger()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	want := []string{"print queue", "database", "logging"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("closed %v, want %v", order, want)
	}
}

// TestShutdownContinuesPastFailure covers the case that actually costs
// something: one subsystem refusing to close must not strand the ones below it,
// because the next thing that happens on that machine is the operator starting
// the binary again and finding the port or the database still held.
func TestShutdownContinuesPastFailure(t *testing.T) {
	broken := errors.New("printer is wedged")
	closed := map[string]bool{}

	var s stack
	s.push("logging", func(context.Context) error {
		closed["logging"] = true
		return nil
	})
	s.push("print queue", func(context.Context) error { return broken })

	err := s.shutdown(context.Background(), quietLogger())

	if !errors.Is(err, broken) {
		t.Errorf("error = %v, want it to wrap %v", err, broken)
	}
	if !strings.Contains(err.Error(), "closing print queue") {
		t.Errorf("error = %q, want it to name the subsystem", err)
	}
	if !closed["logging"] {
		t.Error("logging was not closed: one failure stranded the subsystem below it")
	}
}

// TestShutdownJoinsEveryFailure keeps a second broken subsystem from hiding
// behind the first in the log a support call will be reading.
func TestShutdownJoinsEveryFailure(t *testing.T) {
	first, second := errors.New("db busy"), errors.New("printer wedged")

	var s stack
	s.push("database", func(context.Context) error { return first })
	s.push("print queue", func(context.Context) error { return second })

	err := s.shutdown(context.Background(), quietLogger())

	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Errorf("error = %v, want it to wrap both %v and %v", err, first, second)
	}
}

// TestShutdownOfNothingSucceeds: a startup that fails before anything starts
// still unwinds through here.
func TestShutdownOfNothingSucceeds(t *testing.T) {
	var s stack
	if err := s.shutdown(context.Background(), quietLogger()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

// TestShutdownPassesItsDeadline proves the deadline reaches the subsystems,
// which is what lets a close that talks to hardware give up rather than hang
// the restart.
func TestShutdownPassesItsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var seen error
	var s stack
	s.push("print queue", func(ctx context.Context) error {
		seen = ctx.Err()
		return nil
	})

	if err := s.shutdown(ctx, quietLogger()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !errors.Is(seen, context.Canceled) {
		t.Errorf("subsystem saw ctx.Err() = %v, want context.Canceled", seen)
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestTunnelStatus: the tunnel client's states in the API's words. A venue
// with no relay has no client and is off, which is not a fault.
func TestTunnelStatus(t *testing.T) {
	if got := tunnelStatus(nil); got.State != api.TunnelOff || !got.Since.IsZero() {
		t.Errorf("no relay configured: %+v, want off", got)
	}

	idle, err := tunnel.New(tunnel.Options{
		RelayURL: "wss://relay.example",
		Key:      "mzr_cafe.0123456789ab.sig",
		Handler:  http.NotFoundHandler(),
		Mount:    web.MountGuest,
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := tunnelStatus(idle); got.State != api.TunnelStopped || got.Since.IsZero() || got.BackOffice {
		t.Errorf("a client not started: %+v, want stopped, with when", got)
	}

	// The setting is carried through, whatever the line is doing (ADR-306).
	open, err := tunnel.New(tunnel.Options{
		RelayURL:   "wss://relay.example",
		Key:        "mzr_cafe.0123456789ab.remote.sig",
		Handler:    http.NotFoundHandler(),
		Mount:      web.MountGuest,
		BackOffice: true,
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := tunnelStatus(open); !got.BackOffice {
		t.Errorf("remote_backoffice on: %+v, want BackOffice", got)
	}

	tests := []struct {
		in   tunnel.State
		want api.TunnelState
	}{
		{tunnel.StateConnecting, api.TunnelConnecting},
		{tunnel.StateConnected, api.TunnelConnected},
		{tunnel.StateWaiting, api.TunnelWaiting},
		{tunnel.StateRefused, api.TunnelRefused},
		{tunnel.StateReplaced, api.TunnelReplaced},
		{tunnel.StateStopped, api.TunnelStopped},
		{tunnel.State("a state added later and not mapped"), api.TunnelStopped},
	}
	for _, tt := range tests {
		if got := translateTunnelState(tt.in); got != tt.want {
			t.Errorf("translateTunnelState(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestRemoteNeedsRelay: asking for the back office from outside with no
// relay to carry it stops the start, since the setting would do nothing and
// the owner would believe it did (ADR-019, ADR-306).
func TestRemoteNeedsRelay(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{"neither", config.Config{}, false},
		{"a relay and no remote", config.Config{RelayURL: "wss://relay.example"}, false},
		{"both", config.Config{RelayURL: "wss://relay.example", RemoteBackoffice: true}, false},
		{"remote and no relay", config.Config{RemoteBackoffice: true}, true},
	}
	for _, tt := range tests {
		err := remoteNeedsRelay(tt.cfg)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: remoteNeedsRelay = %v", tt.name, err)
		}
		if err != nil && (!strings.Contains(err.Error(), "remote_backoffice") || !strings.Contains(err.Error(), "relay_url")) {
			t.Errorf("%s: the error does not name both settings: %v", tt.name, err)
		}
	}
}

// TestTheRelayMarkIsOneName: the tunnel writes the mark and the API reads
// it, and neither package may import the other, so each has the name. If
// they ever differed, every relayed request would be taken for one from the
// venue's own network and held to no rule at all (ADR-307, ADR-308).
func TestTheRelayMarkIsOneName(t *testing.T) {
	if api.HeaderVia != tunnel.HeaderVia || api.ViaRelay != tunnel.ViaRelay {
		t.Fatalf("the API reads %q: %q, the tunnel writes %q: %q",
			api.HeaderVia, api.ViaRelay, tunnel.HeaderVia, tunnel.ViaRelay)
	}
}
