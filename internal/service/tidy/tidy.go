// Package tidy is the venue's housekeeping: what a database that runs for
// years must shed now and then, done once a day on a goroutine of its own
// (ADR-372). Nothing here changes a sale, an invoice or a shift; it drops
// what was kept only for a retry minutes later.
package tidy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

// KeepIdempotencyKeys is how long an idempotency key is kept. A retry of a
// lost answer comes within minutes; a day covers a till left off overnight
// mid-request; thirty days covers anyone's imagination.
const KeepIdempotencyKeys = 30 * 24 * time.Hour

// Every is how often the housekeeping runs. Once a day, and once at start.
const Every = 24 * time.Hour

// Clock is where the tidier gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options is what a Tidier is made from.
type Options struct {
	DB    *db.DB
	Log   *slog.Logger
	Clock Clock // nil is the system clock
	// Every overrides the interval, for tests. Zero is Every.
	Every time.Duration
}

// Tidier runs the housekeeping until closed.
type Tidier struct {
	db    *db.DB
	log   *slog.Logger
	clock Clock
	every time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New returns a Tidier. It starts nothing.
func New(o Options) (*Tidier, error) {
	if o.DB == nil {
		return nil, errors.New("tidy: the database is needed")
	}
	t := &Tidier{db: o.DB, log: o.Log, clock: o.Clock, every: o.Every}
	if t.log == nil {
		t.log = slog.Default()
	}
	if t.clock == nil {
		t.clock = systemClock{}
	}
	if t.every <= 0 {
		t.every = Every
	}
	return t, nil
}

// Start runs the housekeeping now and then every interval, until Close.
func (t *Tidier) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done != nil {
		return errors.New("tidy: already started")
	}
	// Not ctx: the loop ends by Close, with the rest of the subsystems,
	// and not by whatever cancels the caller's context first.
	loopCtx, cancel := context.WithCancel(context.Background())
	t.cancel, t.done = cancel, make(chan struct{})
	go t.loop(loopCtx)
	return nil
}

// Close stops the loop and waits for a run in progress, or for ctx.
func (t *Tidier) Close(ctx context.Context) error {
	t.mu.Lock()
	cancel, done := t.cancel, t.done
	t.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tidy: closing: %w", ctx.Err())
	}
}

func (t *Tidier) loop(ctx context.Context) {
	defer close(t.done)
	ticker := time.NewTicker(t.every)
	defer ticker.Stop()
	for {
		t.Run(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Run does one round of housekeeping now. A failure is said and the next
// round tries again; nothing here is worth stopping the server for.
func (t *Tidier) Run(ctx context.Context) {
	pruned, err := t.PruneIdempotencyKeys(ctx, t.clock.Now().Add(-KeepIdempotencyKeys))
	if err != nil {
		if ctx.Err() == nil {
			t.log.Warn("tidy: pruning idempotency keys failed", "error", err)
		}
		return
	}
	if pruned > 0 {
		t.log.Info("tidy: pruned idempotency keys", "keys", pruned, "older_than", KeepIdempotencyKeys.String())
	}
}

// PruneIdempotencyKeys drops the keys made before the cut-off and answers
// how many went.
func (t *Tidier) PruneIdempotencyKeys(ctx context.Context, before time.Time) (int64, error) {
	var pruned int64
	err := t.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.PruneIdempotencyKeys(ctx, before.UTC())
		pruned = n
		return err
	})
	return pruned, err
}
