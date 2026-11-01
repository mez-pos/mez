package backup

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/benbjohnson/litestream"
)

// TestTheProbeTakesForLostOnlyWhatWasThereBeforeItLooked: a destination
// has lost what it had when a copy known to be there before the probe
// looked is not found. A first copy that lands while the probe is looking
// was not there to lose: "back up now" sends it, sees it and notes it
// sent, and a listing made a moment before it landed finds nothing. Taken
// for a loss, the backup started over and sent the whole database again
// (seen on CI under load, 2026-11-01).
func TestTheProbeTakesForLostOnlyWhatWasThereBeforeItLooked(t *testing.T) {
	tests := []struct {
		name string
		// sentBefore: the copy was known to be there before the look.
		sentBefore bool
		// landsDuringLook: "back up now" notes the copy sent while the
		// probe is still listing.
		landsDuringLook bool
		found           bool
		wantLost        bool
	}{
		{name: "sent, and not there: lost", sentBefore: true, found: false, wantLost: true},
		{name: "sent, and there", sentBefore: true, found: true},
		{name: "the first copy lands while it looks", landsDuringLook: true, found: false},
		{name: "nothing sent, nothing there", found: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := probedEngine(t)
			e.sent.Store(tt.sentBefore)
			e.dest = destination{reach: func(context.Context) (bool, error) {
				if tt.landsDuringLook {
					e.sent.Store(true)
				}
				return tt.found, nil
			}}
			marker := e.ldb.MetaPath() + markerSuffix
			if err := os.WriteFile(marker, []byte("mark\ngeneration\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			e.probe(context.Background())
			e.tidying.Wait()

			if got := e.errors.lost.Load(); got != tt.wantLost {
				t.Errorf("lost = %v, want %v", got, tt.wantLost)
			}
			_, err := os.Stat(marker)
			if kept := err == nil; kept == tt.wantLost {
				t.Errorf("the marker that carries the generation on: kept = %v, want %v", kept, !tt.wantLost)
			}
		})
	}
}

// probedEngine is an engine with Litestream already started, as far as
// probe can tell, and nothing behind it.
func probedEngine(t *testing.T) *Engine {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Engine{
		log:    quiet,
		clock:  systemClock{},
		ldb:    litestream.NewDB(filepath.Join(t.TempDir(), "mez.db")),
		errors: &errorKeeper{Handler: quiet.Handler(), last: new(lastError), lost: new(atomic.Bool)},
		opened: true,
	}
}
