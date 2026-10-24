package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

// TestBindWaitsForAnAddressThisComputerLacks: an address that is not this
// computer's is tried again until the patience runs out, and then refused
// with the system's own error; a port that is taken is refused at once.
func TestBindWaitsForAnAddressThisComputerLacks(t *testing.T) {
	log := slog.New(slog.NewTextHandler(discardWriter{}, nil))

	// 198.51.100.1 is documentation space (RFC 5737): no computer has it.
	start := time.Now()
	_, err := bindPatiently(context.Background(), log, "198.51.100.1:0", 120*time.Millisecond, 20*time.Millisecond)
	if err == nil || !addrNotAvailable(err) {
		t.Fatalf("err = %v, want EADDRNOTAVAIL", err)
	}
	if waited := time.Since(start); waited < 100*time.Millisecond {
		t.Errorf("gave up after %s, before the patience ran out", waited)
	}

	// A taken port is not waited on.
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close() //nolint:errcheck // a test's
	start = time.Now()
	_, err = bindPatiently(context.Background(), log, taken.Addr().String(), time.Second, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "listening on "+taken.Addr().String()) {
		t.Fatalf("err = %v", err)
	}
	if waited := time.Since(start); waited > 500*time.Millisecond {
		t.Errorf("waited %s on a port that is taken", waited)
	}

	// A stop while waiting is a stop.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bindPatiently(ctx, log, "198.51.100.1:0", time.Minute, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the context's", err)
	}

	// And an address this computer has is bound at once.
	l, err := bindPatiently(context.Background(), log, "127.0.0.1:0", time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
