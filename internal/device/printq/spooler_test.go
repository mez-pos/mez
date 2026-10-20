package printq

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/printjob"
)

// TestSpoolerNoSuchPrinter: a printer name this computer does not have is
// a permanent failure on Windows, and every spooler job is one elsewhere.
// Either way the job goes dead with the reason instead of spending its
// retries. On Windows this calls the real spooler.
func TestSpoolerNoSuchPrinter(t *testing.T) {
	err := sendSpooler(context.Background(), "mez test: no printer has this name", []byte("x"))
	if !errors.Is(err, printjob.ErrPermanent) {
		t.Errorf("err = %v, want ErrPermanent", err)
	}
}

func TestSpoolerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sendSpooler(ctx, "any", []byte("x")); err == nil {
		t.Error("a cancelled send succeeded")
	}
}

func TestDefaultTransports(t *testing.T) {
	ts := DefaultTransports()
	if ts[TCP] == nil || ts[Windows] == nil || len(ts) != 2 {
		t.Fatalf("transports = %v", ts)
	}
	// The TCP transport is escpos.Send: an address nothing listens on fails,
	// and retryably.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ts[TCP].Send(ctx, "127.0.0.1:1", []byte("x")); err == nil || errors.Is(err, printjob.ErrPermanent) {
		t.Errorf("err = %v, want a retryable failure", err)
	}
}
