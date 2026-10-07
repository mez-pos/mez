package backup_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
)

// venue is a data directory with a migrated database in it, the way the
// server leaves one, and somewhere else to back it up to.
type venue struct {
	dataDir string
	cfg     backup.Config
}

func newVenue(t *testing.T) venue {
	t.Helper()
	return venue{
		dataDir: t.TempDir(),
		cfg:     backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "backups")},
	}
}

func (v venue) dbPath() string { return filepath.Join(v.dataDir, db.FileName) }

// dbOptions opens the venue's database as the server does when a backup
// may be running: the write-ahead log is the backup's to fold in.
func (v venue) dbOptions() db.Options {
	return db.Options{Path: v.dbPath(), LogKeptBy: func() bool { return true }}
}

// open starts the database and the backup over it in the order the server
// does, and returns a stop that closes them in the order the server does:
// the database first, the backup after it.
func (v venue) open(t *testing.T) (*db.DB, *backup.Engine, func()) {
	t.Helper()
	ctx := context.Background()

	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, Log: testLog(t)})
	if err != nil {
		t.Fatalf("backup.Start: %v", err)
	}

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		if err := database.Close(ctx); err != nil {
			t.Errorf("closing the database: %v", err)
		}
		if err := engine.Close(ctx); err != nil {
			t.Errorf("closing the backup: %v", err)
		}
	}
	t.Cleanup(stop)
	return database, engine, stop
}

// testLog is the backup's log for a test: nowhere, unless MEZ_TEST_LOG is
// set, when its warnings and errors go to the test's own output. For
// finding out why a backup did what it did.
func testLog(t *testing.T) *slog.Logger {
	if os.Getenv("MEZ_TEST_LOG") == "" {
		return nil
	}
	return slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

// sell writes n rows the way a service does: through the writer, each in
// its own transaction. The table is the test's own, in a database that
// otherwise holds the real schema.
func sell(t *testing.T, database *db.DB, from, n int) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.Writer().ExecContext(ctx, "CREATE TABLE IF NOT EXISTS backup_probe (id INTEGER PRIMARY KEY, note TEXT NOT NULL)"); err != nil {
		t.Fatalf("making the probe table: %v", err)
	}
	for i := from; i < from+n; i++ {
		err := database.Tx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO backup_probe (id, note) VALUES (?, ?)", i, fmt.Sprint("sale ", i))
			return err
		})
		if err != nil {
			t.Fatalf("writing row %d: %v", i, err)
		}
	}
}

// restored restores the venue's backup to a new file and reads back what
// is in it: how many probe rows, and how many tables of the real schema.
func restored(t *testing.T, cfg backup.Config) (rows, tables int) {
	t.Helper()
	ctx := context.Background()
	out := filepath.Join(t.TempDir(), db.FileName)
	if err := backup.Restore(ctx, cfg, out); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Opened the way the server would, so a file it could not start on
	// fails here.
	database, err := db.Open(ctx, db.Options{Path: out})
	if err != nil {
		t.Fatalf("opening the restored database: %v", err)
	}
	defer database.Close(ctx)

	if err := database.Reader().QueryRowContext(ctx, "SELECT count(*), coalesce(max(id), 0) FROM backup_probe").Scan(&rows, new(int)); err != nil {
		t.Fatalf("counting the restored rows: %v", err)
	}
	if err := database.Reader().QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&tables); err != nil {
		t.Fatalf("counting the restored tables: %v", err)
	}
	var check string
	if err := database.Reader().QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
		t.Fatalf("integrity_check on the restored database: %q, %v", check, err)
	}
	return rows, tables
}

func tableCount(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Reader().QueryRowContext(context.Background(), "SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&n); err != nil {
		t.Fatalf("counting tables: %v", err)
	}
	return n
}

// TestARestoreHoldsWhatWasWritten is the reason the package exists: a
// database that was being written to is restored from its backup alone,
// and everything is there.
func TestARestoreHoldsWhatWasWritten(t *testing.T) {
	v := newVenue(t)
	database, engine, stop := v.open(t)
	ctx := context.Background()

	sell(t, database, 1, 40)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	wantTables := tableCount(t, database)

	// Restored while the venue is still open and selling: everything up
	// to "back up now" is at the destination already.
	if rows, tables := restored(t, v.cfg); rows != 40 || tables != wantTables {
		t.Errorf("restored after BackUpNow: %d rows and %d tables, want 40 and %d", rows, tables, wantTables)
	}

	// And what is written after it goes on the way down, with nobody
	// asking.
	sell(t, database, 41, 25)
	stop()
	if rows, tables := restored(t, v.cfg); rows != 65 || tables != wantTables {
		t.Errorf("restored after closing: %d rows and %d tables, want 65 and %d", rows, tables, wantTables)
	}
}

// TestItCarriesOnAfterARestart covers the venue's every morning: the
// backup made yesterday is added to, not started again or abandoned.
func TestItCarriesOnAfterARestart(t *testing.T) {
	v := newVenue(t)

	database, _, stop := v.open(t)
	sell(t, database, 1, 30)
	stop()

	database, _, stop = v.open(t)
	sell(t, database, 31, 30)
	stop()

	// A third start that writes nothing must not lose anything either.
	_, _, stop = v.open(t)
	stop()

	if rows, _ := restored(t, v.cfg); rows != 60 {
		t.Errorf("restored %d rows after two days, want 60", rows)
	}
}

// TestClosingTheDatabaseDoesNotWaitOnTheBackup covers the shutdown order.
// Litestream holds the write-ahead log open, and the database's closing
// checkpoint must not sit out its busy timeout against it: the whole
// shutdown has ten seconds.
func TestClosingTheDatabaseDoesNotWaitOnTheBackup(t *testing.T) {
	v := newVenue(t)
	database, engine, _ := v.open(t)
	ctx := context.Background()

	sell(t, database, 1, 20)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	sell(t, database, 21, 20)

	start := time.Now()
	if err := database.Close(ctx); err != nil {
		t.Fatalf("closing the database: %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("closing the database took %s under a running backup, want it not to wait", took)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("closing the backup: %v", err)
	}

	// Once both are closed the database file stands on its own: everything
	// is in it, whatever is left of the log beside it.
	if err := os.Remove(v.dbPath() + "-wal"); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removing the write-ahead log: %v", err)
	}
	os.Remove(v.dbPath() + "-shm")
	reopened, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatalf("reopening without the log: %v", err)
	}
	defer reopened.Close(ctx)
	var rows int
	if err := reopened.Reader().QueryRowContext(ctx, "SELECT count(*) FROM backup_probe").Scan(&rows); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if rows != 40 {
		t.Errorf("the database file alone holds %d rows, want 40", rows)
	}
}

type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time { return c.at }

func TestStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("off", func(t *testing.T) {
		engine, err := backup.Start(ctx, backup.Options{DBPath: filepath.Join(t.TempDir(), db.FileName)})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if got := engine.Status(); got.State != backup.StateOff {
			t.Errorf("Status = %+v, want off", got)
		}
		if err := engine.BackUpNow(ctx); !errors.Is(err, backup.ErrOff) {
			t.Errorf("BackUpNow = %v, want ErrOff", err)
		}
		if err := engine.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	t.Run("a nil engine is off too", func(t *testing.T) {
		var engine *backup.Engine
		if got := engine.Status(); got.State != backup.StateOff {
			t.Errorf("Status = %+v, want off", got)
		}
		if err := engine.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	t.Run("ok once it has reached the destination, failing once that is stale", func(t *testing.T) {
		v := newVenue(t)
		database, err := db.Open(ctx, v.dbOptions())
		if err != nil {
			t.Fatal(err)
		}
		clock := &fixedClock{at: time.Now()}
		engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, Clock: clock})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { database.Close(ctx); engine.Close(ctx) })

		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		clock.at = time.Now()
		got := engine.Status()
		if got.State != backup.StateOK || got.LastSyncedAt.IsZero() || got.Error != "" {
			t.Errorf("Status = %+v, want ok with a time and no error", got)
		}
		if got.LastSyncedAt.Location() != time.UTC {
			t.Errorf("LastSyncedAt is in %s, want UTC", got.LastSyncedAt.Location())
		}

		clock.at = time.Now().Add(10 * time.Minute)
		if got := engine.Status(); got.State != backup.StateFailing || got.LastSyncedAt.IsZero() {
			t.Errorf("Status ten minutes on = %+v, want failing and still saying when it last worked", got)
		}
	})

	t.Run("a destination that cannot be written to is failing, with why, and sales carry on", func(t *testing.T) {
		v := newVenue(t)
		// A file where the folder should be: nothing can be made inside it.
		if err := os.WriteFile(v.cfg.Folder, []byte("not a folder"), 0o600); err != nil {
			t.Fatal(err)
		}
		database, engine, _ := v.open(t)

		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err == nil {
			t.Fatal("BackUpNow succeeded into a file, want an error")
		}
		got := engine.Status()
		if got.State != backup.StateFailing || got.Error == "" {
			t.Errorf("Status = %+v, want failing with a reason", got)
		}
		sell(t, database, 6, 5) // the till still works
	})

	t.Run("a destination that goes away while nothing is being sold is failing", func(t *testing.T) {
		v := newVenue(t)
		database, err := db.Open(ctx, v.dbOptions())
		if err != nil {
			t.Fatal(err)
		}
		engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, ProbeEvery: 20 * time.Millisecond})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { database.Close(ctx); engine.Close(ctx) })

		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		if got := engine.Status(); got.State != backup.StateOK {
			t.Fatalf("Status = %+v, want ok before the drive goes", got)
		}

		// The drive is pulled: the folder is gone and nothing can be made
		// where it was. Nobody sells anything, so Litestream has no reason
		// to touch it.
		rename(t, v.cfg.Folder, v.cfg.Folder+"-pulled")
		if err := os.WriteFile(v.cfg.Folder, []byte("not a folder"), 0o600); err != nil {
			t.Fatal(err)
		}
		waitForState(t, engine, backup.StateFailing)
		if got := engine.Status(); got.Error == "" || got.LastSyncedAt.IsZero() {
			t.Errorf("Status = %+v, want a reason and when it last worked", got)
		}

		// And put back.
		if err := os.Remove(v.cfg.Folder); err != nil {
			t.Fatal(err)
		}
		rename(t, v.cfg.Folder+"-pulled", v.cfg.Folder)
		waitForState(t, engine, backup.StateOK)
	})

	t.Run("an error is forgotten once a backup gets through", func(t *testing.T) {
		v := newVenue(t)
		if err := os.WriteFile(v.cfg.Folder, []byte("not a folder"), 0o600); err != nil {
			t.Fatal(err)
		}
		database, engine, _ := v.open(t)
		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err == nil {
			t.Fatal("BackUpNow succeeded into a file")
		}
		if err := os.Remove(v.cfg.Folder); err != nil {
			t.Fatal(err)
		}
		// Litestream is backing off by now; asking is what an owner who
		// has just plugged the drive back in would do.
		deadline := time.Now().Add(10 * time.Second)
		for engine.BackUpNow(ctx) != nil {
			if time.Now().After(deadline) {
				t.Fatalf("BackUpNow never got through: %v", engine.BackUpNow(ctx))
			}
			time.Sleep(50 * time.Millisecond)
		}
		waitForState(t, engine, backup.StateOK)
		if got := engine.Status(); got.Error != "" {
			t.Errorf("Status = %+v, want the old error gone", got)
		}
	})

	t.Run("starting, before it has had a chance", func(t *testing.T) {
		v := newVenue(t)
		clock := &fixedClock{at: time.Now()}
		// No database file yet: there is nothing to have synced.
		engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, Clock: clock})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { engine.Close(ctx) })
		if got := engine.Status(); got.State != backup.StateStarting {
			t.Errorf("Status = %+v, want starting", got)
		}
		clock.at = clock.at.Add(5 * time.Minute)
		if got := engine.Status(); got.State != backup.StateFailing {
			t.Errorf("Status five minutes on with nothing synced = %+v, want failing", got)
		}
	})
}

// rename moves a folder, asking until it can: Windows will not move one
// while a file in it is open, and Litestream opens one now and then.
func rename(t *testing.T, from, to string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := os.Rename(from, to)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("moving %s: %v", from, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitForState waits for the probe to notice.
func waitForState(t *testing.T, engine *backup.Engine, want backup.State) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for engine.Status().State != want {
		if time.Now().After(deadline) {
			t.Fatalf("Status = %+v, want %s", engine.Status(), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCloseDoesNotOutstayItsBudget: whatever the destination is doing,
// the shutdown gets its answer when its time is up.
func TestCloseDoesNotOutstayItsBudget(t *testing.T) {
	v := newVenue(t)
	if err := os.WriteFile(v.cfg.Folder, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	database, engine, _ := v.open(t)
	sell(t, database, 1, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := database.Close(ctx); err != nil {
		t.Fatalf("closing the database: %v", err)
	}
	start := time.Now()
	if err := engine.Close(ctx); err != nil {
		t.Errorf("Close = %v, want a backup that cannot reach its destination to stop without failing the shutdown", err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("Close took %s on a 2s budget", took)
	}
}

func TestStartRefusesADestinationThatIsWrong(t *testing.T) {
	dataDir := t.TempDir()
	_, err := backup.Start(context.Background(), backup.Options{
		DBPath: filepath.Join(dataDir, db.FileName),
		Config: backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(dataDir, "backups")},
	})
	if !errors.Is(err, backup.ErrInvalidConfig) {
		t.Fatalf("Start = %v, want ErrInvalidConfig", err)
	}
}

func TestRestoreRefuses(t *testing.T) {
	ctx := context.Background()

	t.Run("with no destination", func(t *testing.T) {
		err := backup.Restore(ctx, backup.Config{}, filepath.Join(t.TempDir(), db.FileName))
		if !errors.Is(err, backup.ErrOff) {
			t.Errorf("Restore = %v, want ErrOff", err)
		}
	})

	t.Run("from a destination that is not there", func(t *testing.T) {
		cfg := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "unplugged")}
		err := backup.Restore(ctx, cfg, filepath.Join(t.TempDir(), db.FileName))
		if !errors.Is(err, backup.ErrDestinationMissing) || errors.Is(err, backup.ErrNothingToRestore) {
			t.Errorf("Restore = %v, want ErrDestinationMissing and not ErrNothingToRestore", err)
		}
	})

	t.Run("from a destination with nothing in it", func(t *testing.T) {
		cfg := backup.Config{Kind: backup.KindFolder, Folder: t.TempDir()}
		out := filepath.Join(t.TempDir(), db.FileName)
		if err := backup.Restore(ctx, cfg, out); !errors.Is(err, backup.ErrNothingToRestore) {
			t.Errorf("Restore = %v, want ErrNothingToRestore", err)
		}
		if _, err := os.Stat(out); err == nil {
			t.Error("a restore of nothing left a file behind")
		}
	})

	t.Run("to write over a database", func(t *testing.T) {
		v := newVenue(t)
		database, engine, stop := v.open(t)
		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		stop()

		before, err := os.ReadFile(v.dbPath())
		if err != nil {
			t.Fatal(err)
		}
		if err := backup.Restore(ctx, v.cfg, v.dbPath()); !errors.Is(err, backup.ErrWouldOverwrite) {
			t.Fatalf("Restore over the live database = %v, want ErrWouldOverwrite", err)
		}
		after, err := os.ReadFile(v.dbPath())
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Error("the refused restore changed the database it refused to write over")
		}
	})
}
