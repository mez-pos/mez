package backup

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
)

// TestTheKeeperHearsWhatTheLogDoesNotPrint: the server logging at warn
// must not make the backup deaf to its own recovery, and Litestream's
// line a second must not reach a venue's log.
func TestTheKeeperHearsWhatTheLogDoesNotPrint(t *testing.T) {
	tests := []struct {
		name        string
		level       slog.Level
		wantChatter bool
	}{
		{name: "the server at warn", level: slog.LevelWarn},
		{name: "the server at info", level: slog.LevelInfo},
		{name: "the server at debug prints it all", level: slog.LevelDebug, wantChatter: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			keeper := &errorKeeper{
				Handler: slog.NewTextHandler(&out, &slog.HandlerOptions{Level: tt.level}),
				last:    new(lastError),
				lost:    new(atomic.Bool),
			}
			// Through With and a group, as Litestream's loggers are made.
			log := slog.New(keeper).With("subsystem", "backup").WithGroup("replica")

			log.Info("replica sync")
			if got := strings.Contains(out.String(), "replica sync"); got != tt.wantChatter {
				t.Errorf("the every-second line printed = %v, want %v", got, tt.wantChatter)
			}

			log.Error("monitor error", "error", errors.New("the drive is gone"), "backoff", "1s")
			if msg, at := keeper.last.get(); msg != "the drive is gone" || at.IsZero() {
				t.Errorf("after an error the keeper holds %q at %v, want the error's own words and a time", msg, at)
			}
			if !strings.Contains(out.String(), "the drive is gone") {
				t.Error("the error did not reach the log")
			}

			log.Error("auto-recovery failed")
			if msg, _ := keeper.last.get(); msg != "auto-recovery failed" {
				t.Errorf("an error with no error attribute is kept as %q, want its message", msg)
			}

			log.Info("replica sync recovered", "previous_errors", 3)
			if msg, _ := keeper.last.get(); msg != "" {
				t.Errorf("after recovering the keeper still holds %q", msg)
			}
		})
	}
}
