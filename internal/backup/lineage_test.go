package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
)

// session runs the venue for a moment: start, sell n rows from from, stop.
func session(t *testing.T, v venue, from, n int) {
	t.Helper()
	database, engine, stop := v.open(t)
	half := n / 2
	sell(t, database, from, half)
	if err := engine.BackUpNow(context.Background()); err != nil {
		// A destination found to have lost what it had says so here, and
		// is sent everything by the session after.
		t.Logf("BackUpNow: %v", err)
	}
	// Long enough, hurried, for the small files to be merged and deleted.
	time.Sleep(700 * time.Millisecond)
	sell(t, database, from+half, n-half)
	time.Sleep(300 * time.Millisecond)
	stop()
	if got := localSmallFiles(t, v); got > 12 {
		t.Logf("%d small files are still on this machine", got)
	}
}

func localSmallFiles(t *testing.T, v venue) int {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(v.dataDir, ".mez.db-litestream", "ltx", "0"))
	return len(entries)
}

// TestMovingTheBackupElsewhere: an owner changes the folder, and later
// changes it back. Each destination must restore to everything sold up to
// the last time it was the destination.
func TestMovingTheBackupElsewhere(t *testing.T) {
	backup.HurryLitestream(t)
	v := newVenue(t)
	a := v.cfg
	b := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "elsewhere")}

	session(t, v, 1, 30)

	v.cfg = b
	session(t, v, 31, 30)
	if rows, _ := restored(t, b); rows != 60 {
		t.Errorf("the new destination restores %d rows, want all 60", rows)
	}

	v.cfg = a
	session(t, v, 61, 30)
	if rows, _ := restored(t, a); rows != 90 {
		t.Errorf("the first destination, gone back to, restores %d rows, want all 90", rows)
	}
	if rows, _ := restored(t, b); rows != 60 {
		t.Errorf("the destination left behind restores %d rows, want the 60 it had", rows)
	}
}

// TestANewDriveInTheOldOnesPlace: the same folder, emptied. The backup
// starts again on it and does not fail for ever looking for what it had.
func TestANewDriveInTheOldOnesPlace(t *testing.T) {
	backup.HurryLitestream(t)
	v := newVenue(t)
	session(t, v, 1, 30)

	if err := os.RemoveAll(v.cfg.Folder); err != nil {
		t.Fatal(err)
	}
	session(t, v, 31, 30)
	session(t, v, 61, 5)
	if rows, _ := restored(t, v.cfg); rows != 65 {
		t.Errorf("the emptied destination restores %d rows, want all 65", rows)
	}
}

// TestLitestreamsWorkingFilesLost: the data directory was copied to a new
// machine without the hidden folder, and the destination still holds the
// old backup.
func TestLitestreamsWorkingFilesLost(t *testing.T) {
	backup.HurryLitestream(t)
	v := newVenue(t)
	session(t, v, 1, 30)

	if err := os.RemoveAll(filepath.Join(v.dataDir, ".mez.db-litestream")); err != nil {
		t.Fatal(err)
	}
	session(t, v, 31, 30)
	session(t, v, 61, 5)
	if rows, _ := restored(t, v.cfg); rows != 65 {
		t.Errorf("restores %d rows, want all 65", rows)
	}
}
