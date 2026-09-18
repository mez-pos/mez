package db

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

// TestHealthIsAnsweredBeforeAnybodyAsks matters because the health handler is
// forbidden from touching the database: the endpoint is usually being called
// because something has already hung, and a check that does IO is one more
// thing that can hang. Open probes once, so the first request after startup
// gets a real answer from cache rather than "unknown".
func TestHealthIsAnsweredBeforeAnybodyAsks(t *testing.T) {
	d := testDB(t)

	health := d.Health()
	if health.Status != StatusOK {
		t.Errorf("Health().Status = %q, want %q (error: %s)", health.Status, StatusOK, health.Error)
	}
	if health.Error != "" {
		t.Errorf("Health().Error = %q, want empty", health.Error)
	}
	if health.CheckedAt.IsZero() {
		t.Error("Health().CheckedAt is zero, so nothing has actually been checked")
	}
}

// TestHealthAnswersWithoutTouchingTheDatabase is the property the HTTP handler
// depends on. Both pools are closed, so any query would fail or block; Health
// must still answer, because it is reading a cached value and not the disk.
func TestHealthAnswersWithoutTouchingTheDatabase(t *testing.T) {
	d, err := Open(context.Background(), Options{Path: filepath.Join(t.TempDir(), FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := d.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A query against the closed pool must fail, or this test proves nothing.
	var n int
	if err := d.read.QueryRowContext(context.Background(), "SELECT 1").Scan(&n); err == nil {
		t.Fatal("the closed read pool still answers queries, so this test cannot tell the difference")
	}

	answered := make(chan Health, 1)
	go func() { answered <- d.Health() }()

	select {
	case health := <-answered:
		if health.Status != StatusOK {
			t.Errorf("Health().Status = %q after close, want the last cached answer %q", health.Status, StatusOK)
		}
	case <-time.After(time.Second):
		t.Fatal("Health did not answer with the database closed, so it is doing IO")
	}
}

func TestHealthReportsAFailure(t *testing.T) {
	d := testDB(t)

	// Close the read pool out from under the prober, which is what a deleted
	// file or a disk that has gone away looks like from here.
	if err := d.read.Close(); err != nil {
		t.Fatalf("closing the read pool: %v", err)
	}

	d.probe(context.Background())

	health := d.Health()
	if health.Status != StatusFailing {
		t.Errorf("Health().Status = %q, want %q", health.Status, StatusFailing)
	}
	if health.Error == "" {
		t.Error("Health().Error is empty, so the operator is told it is broken and not why")
	}
}

// TestHealthRecovers checks that the status is the current answer rather than
// a latch. A database that was briefly unreachable and is now fine must stop
// reporting failing, or the next real failure is ignored.
func TestHealthRecovers(t *testing.T) {
	d := testDB(t)

	d.setHealth(StatusFailing, io.ErrUnexpectedEOF)
	if d.Health().Status != StatusFailing {
		t.Fatal("the test could not put the database into a failing state")
	}

	d.probe(context.Background())

	if got := d.Health().Status; got != StatusOK {
		t.Errorf("Health().Status = %q after a good probe, want %q", got, StatusOK)
	}
	if got := d.Health().Error; got != "" {
		t.Errorf("Health().Error = %q after a good probe, want empty", got)
	}
}

// TestTheProberRunsOnItsOwn checks the loop, not the probe: with a short
// interval the checked-at timestamp must move without anybody asking.
func TestTheProberRunsOnItsOwn(t *testing.T) {
	d, err := Open(context.Background(), Options{
		Path:          filepath.Join(t.TempDir(), FileName),
		Log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		ProbeInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close(context.Background())

	first := d.Health().CheckedAt

	deadline := time.After(2 * time.Second)
	for {
		if d.Health().CheckedAt.After(first) {
			return
		}
		select {
		case <-deadline:
			t.Fatal("the prober never ran a second time")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestCloseStopsTheProber is the goroutine hygiene rule from
// docs/03-conventions.md: a long running loop is owned by a struct with a
// Close, and it stops when that Close is called.
func TestCloseStopsTheProber(t *testing.T) {
	d, err := Open(context.Background(), Options{
		Path:          filepath.Join(t.TempDir(), FileName),
		Log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		ProbeInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := d.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Close waits for the loop to return, so nothing can move after it.
	settled := d.Health().CheckedAt
	time.Sleep(50 * time.Millisecond)

	if got := d.Health().CheckedAt; !got.Equal(settled) {
		t.Errorf("the prober ran after Close: checked at %s, then %s", settled, got)
	}
}
