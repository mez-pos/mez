package printq

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/printjob"
)

// TestReason: what the queue screen is told about a failed send. A dial
// error says which addresses answer and how, so it never leaves the server;
// a reason the queue or the spooler wrote itself is shown as it is
// (ADR-259).
func TestReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"a refused connection", errors.New("escpos: connecting to the printer at 10.0.0.7:9100: dial tcp 10.0.0.7:9100: connect: connection refused"), Unreachable},
		{"a timeout", fmt.Errorf("escpos: connecting to the printer at 192.168.1.9:9100: %w", context.DeadlineExceeded), Unreachable},
		{"no route", errors.New("dial tcp 10.9.9.9:9100: connect: no route to host"), Unreachable},
		{"a permanent transport error", fmt.Errorf("spooler: loading winspool.drv: oops: %w", printjob.ErrPermanent), Unreachable},
		{"a reason of the queue's own", shown(fmt.Errorf("printer %q has been retired: %w", "Kitchen", printjob.ErrPermanent)), `printer "Kitchen" has been retired: will not succeed on a retry`},
		{"a shown reason wrapped again", fmt.Errorf("sending: %w", shown(errors.New("no printer named \"EPSON\" on this computer"))), `no printer named "EPSON" on this computer`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reason(tt.err); got != tt.want {
				t.Errorf("Reason = %q, want %q", got, tt.want)
			}
		})
	}
	if !errors.Is(shown(fmt.Errorf("x: %w", printjob.ErrPermanent)), printjob.ErrPermanent) {
		t.Error("a shown error lost what it wraps")
	}
}

// TestDialErrorStaysInTheLog: the job keeps the fixed reason; the log keeps
// what the network said, at Warn while the job will be tried again.
func TestDialErrorStaysInTheLog(t *testing.T) {
	f := newFixture(t)
	var logged bytes.Buffer
	f.q.log = slog.New(slog.NewTextHandler(&logged, nil))
	raw := errors.New("escpos: connecting to the printer at 10.0.0.7:9100: dial tcp 10.0.0.7:9100: connect: connection refused")
	f.fake.fail("bar", raw)
	id := f.add(t, 1, "Receipt 1")

	f.pass(t)
	j := f.job(t, id)
	if j.State != "failed" || j.LastError != Unreachable {
		t.Fatalf("job = %+v, want failed with %q", j, Unreachable)
	}
	out := logged.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "connection refused") || !strings.Contains(out, "10.0.0.7:9100") {
		t.Errorf("the log lost the detail: %s", out)
	}
}
