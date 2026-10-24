package tidy

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var now = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func openDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(ctx) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (id, public_id, name, legal_name, currency, timezone, tax_profile, tax_registration_id, financial_year_start_month, cash_rounding_step_minor, created_at, updated_at)
		 VALUES (1, 'ven_T', 'Test', 'Test', 'INR', 'Asia/Kolkata', 'none', '', 4, 100, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	// A key names the order its command was for.
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO orders (id, public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
		 VALUES (1, 'ord_T', 1, 'counter', 'open', 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return d
}

func keyAt(t *testing.T, d *db.DB, key string, at time.Time) {
	t.Helper()
	err := d.WriteTx(context.Background(), func(q *gen.Queries) error {
		return q.SaveIdempotencyKey(context.Background(), gen.SaveIdempotencyKeyParams{
			VenueID: 1, Key: key, Fingerprint: "fp", OrderID: 1, FirstSeq: 1, LastSeq: 1, CreatedAt: at,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func countKeys(t *testing.T, d *db.DB) int {
	t.Helper()
	var n int
	if err := d.Reader().QueryRowContext(context.Background(), `SELECT count(*) FROM idempotency_keys`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestOldKeysGoAndRecentOnesStay: a run prunes the keys older than the
// cut-off and nothing younger, and says how many went.
func TestOldKeysGoAndRecentOnesStay(t *testing.T) {
	d := openDB(t)
	keyAt(t, d, "ancient", now.Add(-31*24*time.Hour))
	keyAt(t, d, "yesterday", now.Add(-24*time.Hour))
	keyAt(t, d, "just-now", now)

	tidier, err := New(Options{DB: d, Clock: fixedClock{now}})
	if err != nil {
		t.Fatal(err)
	}
	pruned, err := tidier.PruneIdempotencyKeys(context.Background(), now.Add(-KeepIdempotencyKeys))
	if err != nil || pruned != 1 {
		t.Fatalf("pruned %d, %v; want 1", pruned, err)
	}
	if n := countKeys(t, d); n != 2 {
		t.Fatalf("%d keys left, want 2", n)
	}
	// Run is the same with the clock's cut-off, and is quiet with nothing to do.
	tidier.Run(context.Background())
	if n := countKeys(t, d); n != 2 {
		t.Fatalf("%d keys after a run with nothing old, want 2", n)
	}
}

// TestTheLoopRunsAtStartAndStops: Start runs once at once, and Close ends
// the loop.
func TestTheLoopRunsAtStartAndStops(t *testing.T) {
	d := openDB(t)
	keyAt(t, d, "ancient", now.Add(-60*24*time.Hour))
	tidier, err := New(Options{DB: d, Clock: fixedClock{now}, Every: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := tidier.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for countKeys(t, d) != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := countKeys(t, d); n != 0 {
		t.Fatalf("%d keys after the start's run, want 0", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tidier.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tidier.Close(ctx); err != nil {
		t.Fatalf("closing twice: %v", err)
	}
}
