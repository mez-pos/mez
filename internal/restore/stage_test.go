package restore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
)

// What stages a real backup and adopts it under a server is tested where
// there is a server to make one: cmd/mez/restorestage_test.go.

// TestAPendingRestoreKeepsItsCodeOutOfALog: however it is printed. In its
// own file it is written whole, which is what the file is for.
func TestAPendingRestoreKeepsItsCodeOutOfALog(t *testing.T) {
	p := Pending{VenueID: "ven_A1B2C3D4", VenueName: "Corner Cafe", StagedAt: time.Date(2026, 10, 12, 16, 30, 0, 0, time.UTC), PairingCode: "ABCD2345"}
	var logged strings.Builder
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	log.Info("adopted", "restore", p)
	log.Info("adopted", "restore", &p)
	printed := fmt.Sprintf("%v %+v %#v %s %v", p, p, p, p, &p) + logged.String() + fmt.Sprint(fmt.Errorf("with %v", p))
	if strings.Contains(printed, "ABCD2345") {
		t.Errorf("the pairing code was printed: %s", printed)
	}
	if !strings.Contains(printed, "ven_A1B2C3D4") || !strings.Contains(printed, "Corner Cafe") {
		t.Errorf("a printed restore does not say whose it is: %s", printed)
	}
}

// emptyDatabase is a database as a server leaves it that was started and
// never set up.
func emptyDatabase(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, db.FileName)
	d, err := db.Open(context.Background(), db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEmptyInstall(t *testing.T) {
	ctx := context.Background()
	if empty, err := EmptyInstall(ctx, filepath.Join(t.TempDir(), db.FileName)); empty || err != nil {
		t.Errorf("no database at all = %v, %v; want not in the way", empty, err)
	}
	path := emptyDatabase(t, t.TempDir())
	if empty, err := EmptyInstall(ctx, path); !empty || err != nil {
		t.Errorf("a database with no venue = %v, %v; want an empty install", empty, err)
	}

	junk := filepath.Join(t.TempDir(), db.FileName)
	if err := os.WriteFile(junk, []byte("this is not a database, and it may be somebody's"), 0o600); err != nil {
		t.Fatal(err)
	}
	if empty, err := EmptyInstall(ctx, junk); empty || err == nil || errors.Is(err, ErrVenueThere) {
		t.Errorf("a file that cannot be read = %v, %v; want an error, and not treated as empty", empty, err)
	}
}

// TestMoveAsideClearsWhatWasTheOldDatabases: the logs and what the backup
// remembered of it, so that neither is read as the restored one's.
func TestMoveAsideClearsWhatWasTheOldDatabases(t *testing.T) {
	dir := t.TempDir()
	path := emptyDatabase(t, dir)
	working := filepath.Join(dir, "."+db.FileName+"-litestream")
	if err := os.MkdirAll(filepath.Join(working, "generations"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, left := range []string{path + "-wal", path + "-shm", working + "-for", working + "-made"} {
		if err := os.WriteFile(left, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	aside, err := MoveAside(path, time.Date(2026, 10, 12, 16, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MoveAside: %v", err)
	}
	if want := path + ".empty-20261012T163000Z"; aside != want {
		t.Errorf("moved to %s, want %s", aside, want)
	}
	if _, err := os.Stat(aside); err != nil {
		t.Errorf("the empty database is not where it was moved to: %v", err)
	}
	for _, gone := range []string{path, path + "-wal", path + "-shm", working, working + "-for"} {
		if _, err := os.Stat(gone); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is still there: %v", gone, err)
		}
	}
	// The list of runs this directory began is kept, to be tidied away.
	if _, err := os.Stat(working + "-made"); err != nil {
		t.Errorf("the list of backups begun here was removed: %v", err)
	}

	if _, err := MoveAside(path, time.Now()); err == nil {
		t.Error("moving aside a database that is not there was accepted")
	}
}

func TestNotRunning(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := emptyDatabase(t, dir)
	if err := NotRunning(ctx, path); err != nil {
		t.Errorf("with nothing running: %v", err)
	}

	d, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(ctx)
	// A server mid-write: the lock a restore must not take the file under.
	tx, err := d.Writer().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "CREATE TABLE held (x)"); err != nil {
		t.Fatal(err)
	}
	if err := NotRunning(ctx, path); !errors.Is(err, ErrStillRunning) {
		t.Errorf("under a server that is writing: %v, want ErrStillRunning", err)
	}
}
