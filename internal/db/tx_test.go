package db

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
)

// seedCounter makes the one table the transaction tests count with.
func seedCounter(t *testing.T, d *DB) {
	t.Helper()

	err := d.Tx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), `
			CREATE TABLE counter (id INTEGER PRIMARY KEY, n INTEGER NOT NULL);
			INSERT INTO counter (id, n) VALUES (1, 0);
		`)
		return err
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
}

func counterValue(t *testing.T, d *DB) int {
	t.Helper()

	var n int
	if err := d.Reader().QueryRowContext(context.Background(), "SELECT n FROM counter WHERE id = 1").Scan(&n); err != nil {
		t.Fatalf("reading the counter: %v", err)
	}
	return n
}

func TestTxCommits(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	err := d.Tx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = n + 7 WHERE id = 1")
		return err
	})
	if err != nil {
		t.Fatalf("Tx: %v", err)
	}

	if got := counterValue(t, d); got != 7 {
		t.Errorf("counter = %d, want 7", got)
	}
}

// TestTxRollsBackOnError is the invariant every money path leans on: a
// transaction that fails half way leaves nothing behind. An invoice number
// allocated for an order that did not commit is a gap in the series.
func TestTxRollsBackOnError(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	sentinel := errors.New("the second step failed")

	err := d.Tx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = 99 WHERE id = 1"); err != nil {
			return err
		}
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("Tx error = %v, want it to wrap %v", err, sentinel)
	}
	if got := counterValue(t, d); got != 0 {
		t.Errorf("counter = %d, want 0: the failed transaction was not rolled back", got)
	}
}

// TestTxRollsBackOnPanic covers the case the deferred rollback exists for. A
// panic must not leave a write transaction open, because the single writer
// connection would then be held forever and the next sale would block until
// the busy timeout.
func TestTxRollsBackOnPanic(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("Tx swallowed the panic, which would hide a bug rather than report it")
			}
		}()

		_ = d.Tx(context.Background(), func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = 99 WHERE id = 1"); err != nil {
				return err
			}
			panic("a handler exploded mid transaction")
		})
	}()

	if got := counterValue(t, d); got != 0 {
		t.Errorf("counter = %d, want 0: the panicking transaction was not rolled back", got)
	}

	// The writer must still work, which is what proves the connection came back.
	err := d.Tx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = 1 WHERE id = 1")
		return err
	})
	if err != nil {
		t.Fatalf("writing after a panic: %v: the writer connection was never released", err)
	}
	if got := counterValue(t, d); got != 1 {
		t.Errorf("counter = %d, want 1", got)
	}
}

// TestTxSerialisesWriters is the reason the writer is one connection. Twenty
// concurrent read-modify-write transactions must land on twenty, not on
// whatever survives the last writer to finish.
func TestTxSerialisesWriters(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	const writers = 20

	var wg sync.WaitGroup
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = d.Tx(context.Background(), func(tx *sql.Tx) error {
				var n int
				if err := tx.QueryRowContext(context.Background(), "SELECT n FROM counter WHERE id = 1").Scan(&n); err != nil {
					return err
				}
				_, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = ? WHERE id = 1", n+1)
				return err
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}

	if got := counterValue(t, d); got != writers {
		t.Errorf("counter = %d, want %d: a read-modify-write was lost", got, writers)
	}
}

func TestTxRespectsAContextThatIsAlreadyCancelled(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := d.Tx(ctx, func(*sql.Tx) error {
		called = true
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Tx error = %v, want it to wrap context.Canceled", err)
	}
	if called {
		t.Error("Tx ran the function on a cancelled context")
	}
	if got := counterValue(t, d); got != 0 {
		t.Errorf("counter = %d, want 0", got)
	}
}

// TestTxReadsItsOwnWrites is worth pinning because it is what lets a caller
// allocate an invoice number and use it in the same transaction.
func TestTxReadsItsOwnWrites(t *testing.T) {
	d := testDB(t)
	seedCounter(t, d)

	err := d.Tx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(context.Background(), "UPDATE counter SET n = 42 WHERE id = 1"); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(context.Background(), "SELECT n FROM counter WHERE id = 1").Scan(&n); err != nil {
			return err
		}
		if n != 42 {
			t.Errorf("inside the transaction the counter reads %d, want 42", n)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tx: %v", err)
	}
}
