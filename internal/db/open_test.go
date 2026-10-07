package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testDB opens a database in a temp directory. Every test in this package runs
// against a real file, because the thing being tested is the connection and
// the pragmas on it, and a mock of SQLite would only prove that the mock
// agrees with the test.
func testDB(t *testing.T) *DB {
	t.Helper()

	d, err := Open(context.Background(), Options{
		Path: filepath.Join(t.TempDir(), FileName),
		Log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	return d
}

func TestOpenCreatesTheDatabaseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	d, err := Open(context.Background(), Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close(context.Background())

	if _, err := os.Stat(path); err != nil {
		t.Errorf("database file: %v", err)
	}
	if d.Path() != path {
		t.Errorf("Path() = %q, want %q", d.Path(), path)
	}
}

func TestOpenCreatesTheDirectoryItNeeds(t *testing.T) {
	// A venue machine on its first boot has an empty data directory, and on a
	// restore it may not have one at all.
	path := filepath.Join(t.TempDir(), "nested", "deeper", FileName)

	d, err := Open(context.Background(), Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close(context.Background())

	if _, err := os.Stat(path); err != nil {
		t.Errorf("database file: %v", err)
	}
}

func TestOpenRejectsAnUnusablePath(t *testing.T) {
	tests := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			name: "empty",
			path: func(*testing.T) string { return "" },
		},
		{
			name: "a directory",
			path: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "a query string would be read as driver options",
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "a?b.db") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Open(context.Background(), Options{Path: tt.path(t)})
			if err == nil {
				d.Close(context.Background())
				t.Fatal("Open succeeded, want an error")
			}
		})
	}
}

// TestPragmasHoldOnEveryConnection is the test this package exists for.
//
// journal_mode is stored in the file, but foreign_keys, busy_timeout and
// synchronous are per connection. database/sql opens connections whenever the
// pool feels like it, so a pragma applied once after Open protects exactly one
// connection and leaves every later one unguarded. Forcing several concurrent
// connections is the only way to see the difference.
func TestPragmasHoldOnEveryConnection(t *testing.T) {
	// A read pool of this test's own choosing, as large as the number of
	// connections it takes at once. The default is sized from the machine,
	// down to two, and on a machine with two processors (a CI runner for a
	// private repository) the third connection was waited for until the
	// test timed out.
	const readers = 4
	d, err := Open(context.Background(), Options{
		Path:         filepath.Join(t.TempDir(), FileName),
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReadPoolSize: readers,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	tests := []struct {
		pragma string
		want   string
		why    string
	}{
		{pragma: "journal_mode", want: "wal", why: "readers must not block the writer during service"},
		{pragma: "foreign_keys", want: "1", why: "an orphaned order line is a wrong bill"},
		{pragma: "busy_timeout", want: "5000", why: "a locked database should wait, not fail a sale"},
		{pragma: "synchronous", want: "1", why: "NORMAL is safe under WAL and survives a power cut"},
	}

	for _, pool := range []struct {
		name string
		open func() *sql.DB
	}{
		{name: "writer", open: d.Writer},
		{name: "reader", open: d.Reader},
	} {
		for _, tt := range tests {
			t.Run(pool.name+"/"+tt.pragma, func(t *testing.T) {
				ctx := context.Background()

				// Take every connection before asking any of them anything.
				// Holding them all at once is what forces the pool to open
				// more than one, which is the whole point: a pragma set once
				// after Open would answer correctly here on the first
				// connection and wrongly on the rest.
				//
				// The writer pool is capped at one connection on purpose, so
				// it is asked once.
				connections := readers
				if pool.name == "writer" {
					connections = 1
				}

				conns := make([]*sql.Conn, 0, connections)
				for i := range connections {
					conn, err := pool.open().Conn(ctx)
					if err != nil {
						t.Fatalf("taking connection %d: %v", i, err)
					}
					defer conn.Close()
					conns = append(conns, conn)
				}

				for i, conn := range conns {
					var got string
					if err := conn.QueryRowContext(ctx, "PRAGMA "+tt.pragma).Scan(&got); err != nil {
						t.Fatalf("connection %d: PRAGMA %s: %v", i, tt.pragma, err)
					}
					if !strings.EqualFold(got, tt.want) {
						t.Errorf("connection %d: %s = %s, want %s (%s)", i, tt.pragma, got, tt.want, tt.why)
					}
				}
			})
		}
	}
}

// TestForeignKeysAreEnforced proves the pragma does something, rather than
// proving SQLite reports it as set. Those are different claims and only one of
// them keeps an order line attached to an order.
func TestForeignKeysAreEnforced(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	err := d.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			CREATE TABLE orders (id INTEGER PRIMARY KEY);
			CREATE TABLE order_lines (
				id       INTEGER PRIMARY KEY,
				order_id INTEGER NOT NULL REFERENCES orders(id)
			);
		`)
		return err
	})
	if err != nil {
		t.Fatalf("creating tables: %v", err)
	}

	err = d.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO order_lines (order_id) VALUES (999)")
		return err
	})
	if err == nil {
		t.Fatal("inserting a line against a missing order succeeded, so foreign keys are not enforced")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Errorf("error = %v, want one naming the foreign key constraint", err)
	}
}

// TestTheReadPoolCannotWrite checks the guard that makes the split worth
// having. The pool is opened read only at the driver, so a write that reaches
// it fails in SQLite rather than depending on everyone remembering which
// handle is which.
func TestTheReadPoolCannotWrite(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	if err := d.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "CREATE TABLE venues (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
		return err
	}); err != nil {
		t.Fatalf("creating the table: %v", err)
	}

	if _, err := d.Reader().ExecContext(ctx, "INSERT INTO venues (name) VALUES ('The Reader')"); err == nil {
		t.Fatal("the read pool accepted a write")
	}

	// And it can still read what the writer wrote.
	if _, err := d.Writer().ExecContext(ctx, "INSERT INTO venues (name) VALUES ('The Writer')"); err != nil {
		t.Fatalf("writing: %v", err)
	}
	var name string
	if err := d.Reader().QueryRowContext(ctx, "SELECT name FROM venues").Scan(&name); err != nil {
		t.Fatalf("reading: %v", err)
	}
	if name != "The Writer" {
		t.Errorf("read %q, want %q", name, "The Writer")
	}
}

// TestCloseCheckpointsTheWAL covers what happens on the way down. A venue
// machine is switched off at the wall most nights, and a database left with a
// write-ahead log full of committed data is one a backup or a support engineer
// cannot copy on its own.
func TestCloseCheckpointsTheWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	ctx := context.Background()

	d, err := Open(ctx, Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	err = d.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "CREATE TABLE venues (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
			return err
		}
		for i := range 200 {
			if _, err := tx.ExecContext(ctx, "INSERT INTO venues (name) VALUES (?)", fmt.Sprint("venue ", i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("writing: %v", err)
	}

	wal := path + "-wal"
	before, err := os.Stat(wal)
	if err != nil {
		t.Fatalf("the write-ahead log: %v", err)
	}
	if before.Size() == 0 {
		t.Fatal("the write-ahead log is empty before close, so this test cannot tell whether it was checkpointed")
	}

	if err := d.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if after, err := os.Stat(wal); err == nil && after.Size() != 0 {
		t.Errorf("the write-ahead log is %d bytes after close, want 0: it was not checkpointed", after.Size())
	}

	// And the data is still there, which is the part that actually matters.
	reopened, err := Open(ctx, Options{Path: path})
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer reopened.Close(ctx)

	var venues int
	if err := reopened.Reader().QueryRowContext(ctx, "SELECT count(*) FROM venues").Scan(&venues); err != nil {
		t.Fatalf("counting after reopen: %v", err)
	}
	if venues != 200 {
		t.Errorf("%d venues after reopening, want 200", venues)
	}
}

func TestCloseIsSafeTwice(t *testing.T) {
	d, err := Open(context.Background(), Options{Path: filepath.Join(t.TempDir(), FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := d.Close(context.Background()); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := d.Close(context.Background()); err != nil {
		t.Errorf("second Close: %v, want nil: shutdown must not depend on being called exactly once", err)
	}
}

func TestUseAfterCloseFails(t *testing.T) {
	d, err := Open(context.Background(), Options{Path: filepath.Join(t.TempDir(), FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := d.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err = d.Tx(context.Background(), func(*sql.Tx) error { return nil })
	if err == nil {
		t.Error("Tx on a closed database succeeded")
	}
	if !errors.Is(err, sql.ErrConnDone) && !errors.Is(err, ErrClosed) {
		t.Logf("Tx after close returned %v", err)
	}
}
