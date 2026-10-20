package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/restore"
)

const stagedCode = "ABCD2345"

// venueIDIn reads the public id of the venue in a data directory that is
// not being served.
func venueIDIn(t *testing.T, dataDir string) string {
	t.Helper()
	d, err := db.Open(context.Background(), db.Options{Path: filepath.Join(dataDir, db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(context.Background())
	var id string
	if err := d.Reader().QueryRow("SELECT public_id FROM venues").Scan(&id); err != nil {
		t.Fatalf("reading the venue's id: %v", err)
	}
	return id
}

// venueNameIn is the name of the venue in a data directory, or "" for none.
func venueNameIn(t *testing.T, dataDir string) string {
	t.Helper()
	d, err := db.Open(context.Background(), db.Options{Path: filepath.Join(dataDir, db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(context.Background())
	var name string
	if err := d.Reader().QueryRow("SELECT name FROM venues").Scan(&name); err != nil {
		return ""
	}
	return name
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// serving starts the server on a data directory and returns what stops it.
func serving(t *testing.T, dataDir string) (stop func()) {
	t.Helper()
	// So that "listening" is this start's.
	if err := os.RemoveAll(filepath.Join(dataDir, "logs")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, dataDir), runOptions{stdout: io.Discard}) }()
	waitForLog(t, dataDir, "listening", done)
	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		case <-time.After(shutdownTimeout + 5*time.Second):
			t.Fatal("the server did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
}

// TestARunningServerTakesARestoredVenueAtItsNextStart is "I had mez before"
// underneath its screen: the new computer's server is up on a database with
// no venue, the venue is staged beside it while it runs, and the next start
// is the venue's (ADR-333).
func TestARunningServerTakesARestoredVenueAtItsNextStart(t *testing.T) {
	old, from, photo := backedUpVenue(t)
	venueID := venueIDIn(t, old)

	fresh := t.TempDir()
	stop := serving(t, fresh)
	dbPath := filepath.Join(fresh, db.FileName)
	pending, staged := filepath.Join(fresh, restore.PendingFile), dbPath+restore.StagedSuffix

	at := time.Date(2026, 10, 12, 16, 30, 0, 0, time.UTC)
	got, err := restore.Stage(context.Background(), restore.StageOptions{
		DataDir: fresh, From: from, VenueID: venueID, PairingCode: stagedCode, Now: func() time.Time { return at },
	})
	if err != nil {
		t.Fatalf("Stage under a running server: %v", err)
	}
	if got.VenueID != venueID || got.VenueName != "The Corner Cafe" || !got.StagedAt.Equal(at) || got.Photos != 1 || got.PhotosErr != nil {
		t.Errorf("staged %+v", got)
	}
	if !exists(t, staged) || !exists(t, pending) {
		t.Fatal("staging left no database beside the server's, or no word of it")
	}
	if !exists(t, filepath.Join(fresh, "photos", photo)) {
		t.Error("the menu's photo was not brought back")
	}

	// The server it was staged under goes on as it was: no venue, and a
	// wizard still to show.
	stop()
	if name := venueNameIn(t, fresh); name != "" {
		t.Fatalf("staging changed the running server's database: it holds %q", name)
	}

	stop = serving(t, fresh)
	log := readLog(t, fresh)
	if !strings.Contains(log, "a restored venue was put in place") || !strings.Contains(log, venueID) {
		t.Errorf("the start did not say it adopted the venue:\n%s", log)
	}
	if strings.Contains(log, stagedCode) {
		t.Errorf("the pairing code is in the log:\n%s", log)
	}
	if strings.Contains(log, "has no venue yet") {
		t.Errorf("the start still offers the setup wizard:\n%s", log)
	}
	stop()

	if name := venueNameIn(t, fresh); name != "The Corner Cafe" {
		t.Errorf("the venue in the database after the start: %q", name)
	}
	if exists(t, staged) || exists(t, pending) {
		t.Error("the staged database or the word of it is still there after it was adopted")
	}
	if aside, _ := filepath.Glob(dbPath + ".empty-*"); len(aside) != 1 {
		t.Errorf("the empty database moved aside: %v; want one", aside)
	}

	// And the start after that is an ordinary one.
	stop = serving(t, fresh)
	if log := readLog(t, fresh); strings.Contains(log, "restored venue") {
		t.Errorf("a later start speaks of a restore:\n%s", log)
	}
	stop()
}

// TestStageRefuses: what is not staged, and that nothing is left behind.
func TestStageRefuses(t *testing.T) {
	old, from, _ := backedUpVenue(t)
	venueID := venueIDIn(t, old)
	ctx := context.Background()

	left := func(t *testing.T, dataDir string) {
		t.Helper()
		if exists(t, filepath.Join(dataDir, restore.PendingFile)) || exists(t, filepath.Join(dataDir, db.FileName+restore.StagedSuffix)) {
			t.Error("a refused staging left a database or word of one")
		}
	}
	t.Run("a backup of another venue", func(t *testing.T) {
		fresh := t.TempDir()
		_, err := restore.Stage(ctx, restore.StageOptions{DataDir: fresh, From: from, VenueID: "ven_ZZZZ9999", PairingCode: stagedCode})
		if !errors.Is(err, restore.ErrWrongVenue) {
			t.Errorf("err = %v, want ErrWrongVenue", err)
		}
		if err != nil && (strings.Contains(err.Error(), "Corner Cafe") || strings.Contains(err.Error(), venueID)) {
			t.Errorf("the refusal says whose backup it was: %v", err)
		}
		left(t, fresh)
	})
	t.Run("over a venue", func(t *testing.T) {
		_, err := restore.Stage(ctx, restore.StageOptions{DataDir: old, From: from, VenueID: venueID})
		if !errors.Is(err, restore.ErrVenueThere) {
			t.Errorf("err = %v, want ErrVenueThere", err)
		}
		left(t, old)
		if name := venueNameIn(t, old); name != "The Corner Cafe" {
			t.Errorf("the venue that was there is now %q", name)
		}
	})
	t.Run("a folder with no backup in it", func(t *testing.T) {
		fresh := t.TempDir()
		_, err := restore.Stage(ctx, restore.StageOptions{DataDir: fresh, From: backup.Config{Kind: backup.KindFolder, Folder: t.TempDir()}, VenueID: venueID})
		if !errors.Is(err, backup.ErrNothingToRestore) {
			t.Errorf("err = %v, want ErrNothingToRestore", err)
		}
		left(t, fresh)
	})
	t.Run("a folder that is not there", func(t *testing.T) {
		fresh := t.TempDir()
		_, err := restore.Stage(ctx, restore.StageOptions{DataDir: fresh, From: backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "gone")}, VenueID: venueID})
		if !errors.Is(err, backup.ErrDestinationMissing) {
			t.Errorf("err = %v, want ErrDestinationMissing", err)
		}
		left(t, fresh)
	})
	t.Run("nothing said of what to stage", func(t *testing.T) {
		for _, o := range []restore.StageOptions{{From: from, VenueID: venueID}, {DataDir: t.TempDir(), VenueID: venueID}, {DataDir: t.TempDir(), From: from}} {
			if _, err := restore.Stage(ctx, o); err == nil {
				t.Errorf("Stage(%+v) was accepted", o)
			}
		}
	})
	t.Run("a staging left unfinished is cleared by the next", func(t *testing.T) {
		fresh := t.TempDir()
		staged := filepath.Join(fresh, db.FileName+restore.StagedSuffix)
		if err := os.WriteFile(staged, []byte("half a database"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := restore.Stage(ctx, restore.StageOptions{DataDir: fresh, From: from, VenueID: venueID}); err != nil {
			t.Fatalf("Stage over an unfinished one: %v", err)
		}
		adopted, err := restore.Adopt(ctx, fresh, time.Now())
		if err != nil || adopted == nil || adopted.PairingCode != "" {
			t.Fatalf("Adopt = %v, %v", adopted, err)
		}
		if name := venueNameIn(t, fresh); name != "The Corner Cafe" {
			t.Errorf("the venue adopted: %q", name)
		}
	})
}

// TestAdopt: at a start, with nothing serving.
func TestAdopt(t *testing.T) {
	old, from, _ := backedUpVenue(t)
	venueID := venueIDIn(t, old)
	ctx := context.Background()
	stage := func(t *testing.T, dataDir string) {
		t.Helper()
		if _, err := restore.Stage(ctx, restore.StageOptions{DataDir: dataDir, From: from, VenueID: venueID, PairingCode: stagedCode}); err != nil {
			t.Fatalf("Stage: %v", err)
		}
	}

	t.Run("nothing waiting is every start but one", func(t *testing.T) {
		if got, err := restore.Adopt(ctx, t.TempDir(), time.Now()); got != nil || err != nil {
			t.Errorf("Adopt = %v, %v", got, err)
		}
		if got, err := restore.Adopt(ctx, old, time.Now()); got != nil || err != nil {
			t.Errorf("Adopt on a venue = %v, %v", got, err)
		}
	})
	t.Run("into a directory with no database at all", func(t *testing.T) {
		fresh := t.TempDir()
		stage(t, fresh)
		// The log of a database that was here once and was deleted by
		// hand: beside the restored one it would be taken for its own.
		// So too what the backup remembered of it, and what a staging cut
		// short as it left word leaves.
		working := filepath.Join(fresh, "."+db.FileName+"-litestream")
		if err := os.MkdirAll(working, 0o750); err != nil {
			t.Fatal(err)
		}
		stale := []string{filepath.Join(fresh, db.FileName+"-wal"), filepath.Join(fresh, db.FileName+"-shm"), working + "-for", filepath.Join(fresh, restore.PendingFile+".tmp")}
		for _, path := range stale {
			if err := os.WriteFile(path, []byte("another database's, with "+stagedCode), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		got, err := restore.Adopt(ctx, fresh, time.Now())
		if err != nil || got == nil || got.VenueID != venueID || got.VenueName != "The Corner Cafe" || got.PairingCode != stagedCode {
			t.Fatalf("Adopt = %+v, %v", got, err)
		}
		for _, path := range append(stale, working) {
			if exists(t, path) {
				t.Errorf("%s is still there beside the restored database", filepath.Base(path))
			}
		}
		if name := venueNameIn(t, fresh); name != "The Corner Cafe" {
			t.Errorf("the venue adopted beside a stale log: %q", name)
		}
		if aside, _ := filepath.Glob(filepath.Join(fresh, db.FileName+".empty-*")); len(aside) != 0 {
			t.Errorf("something was moved aside where there was nothing: %v", aside)
		}
		if exists(t, filepath.Join(fresh, restore.PendingFile)) {
			t.Error("the word, and the code in it, is still on disk")
		}
		// Once.
		if got, err := restore.Adopt(ctx, fresh, time.Now()); got != nil || err != nil {
			t.Errorf("a second Adopt = %v, %v", got, err)
		}
	})
	t.Run("a venue made in between stands", func(t *testing.T) {
		fresh := t.TempDir()
		stage(t, fresh)
		var said bytes.Buffer
		if err := runSetup([]string{"-data-dir", fresh, "-name", "Somebody Else's", "-owner", "Kabir", "-owner-pin", "135790", "-timezone", "Asia/Kolkata"}, &said); err != nil {
			t.Fatalf("setup: %v\n%s", err, said.String())
		}
		got, err := restore.Adopt(ctx, fresh, time.Now())
		if got != nil || !errors.Is(err, restore.ErrVenueThere) {
			t.Fatalf("Adopt over a venue = %v, %v; want ErrVenueThere", got, err)
		}
		if name := venueNameIn(t, fresh); name != "Somebody Else's" {
			t.Errorf("the venue that was there is now %q", name)
		}
		if exists(t, filepath.Join(fresh, restore.PendingFile)) {
			t.Error("the word, and the code in it, was left on disk")
		}
		if !exists(t, filepath.Join(fresh, db.FileName+restore.StagedSuffix)) {
			t.Error("the staged database was removed; it is left for a person to look at")
		}
		// Dropped, and not asked about again.
		if got, err := restore.Adopt(ctx, fresh, time.Now()); got != nil || err != nil {
			t.Errorf("the next start = %v, %v", got, err)
		}
	})
	t.Run("another mez has the empty database open", func(t *testing.T) {
		fresh := t.TempDir()
		// A new install's database, held as a server mid-write holds it.
		held, err := db.Open(ctx, db.Options{Path: filepath.Join(fresh, db.FileName)})
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close(ctx)
		if err := held.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
		stage(t, fresh)
		tx, err := held.Writer().BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "CREATE TABLE held (x)"); err != nil {
			t.Fatal(err)
		}
		got, err := restore.Adopt(ctx, fresh, time.Now())
		if got != nil || !errors.Is(err, restore.ErrStillRunning) {
			t.Fatalf("Adopt under another mez = %v, %v; want ErrStillRunning", got, err)
		}
		if strings.Contains(err.Error(), stagedCode) {
			t.Errorf("the refusal repeats the code: %v", err)
		}
		if aside, _ := filepath.Glob(filepath.Join(fresh, db.FileName+".empty-*")); len(aside) != 0 {
			t.Errorf("the database was moved from under it: %v", aside)
		}
		if !exists(t, filepath.Join(fresh, restore.PendingFile)) || !exists(t, filepath.Join(fresh, db.FileName+restore.StagedSuffix)) {
			t.Fatal("the restore was dropped; it is to be tried at the next start")
		}

		// It lets go, and the next start adopts.
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if got, err := restore.Adopt(ctx, fresh, time.Now()); err != nil || got == nil || got.PairingCode != stagedCode {
			t.Fatalf("the next start: Adopt = %v, %v", got, err)
		}
		if name := venueNameIn(t, fresh); name != "The Corner Cafe" {
			t.Errorf("the venue adopted: %q", name)
		}
	})
	t.Run("a database in the way that cannot be read", func(t *testing.T) {
		fresh := t.TempDir()
		stage(t, fresh)
		junk := []byte("this is not a database, and it may be somebody's")
		if err := os.WriteFile(filepath.Join(fresh, db.FileName), junk, 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := restore.Adopt(ctx, fresh, time.Now())
		if got != nil || err == nil {
			t.Fatalf("Adopt = %v, %v; want an error", got, err)
		}
		if strings.Contains(err.Error(), stagedCode) {
			t.Errorf("the refusal repeats the code: %v", err)
		}
		if raw, _ := os.ReadFile(filepath.Join(fresh, db.FileName)); !bytes.Equal(raw, junk) {
			t.Error("a file that was not known to be empty was moved or changed")
		}
	})
	t.Run("word with no database beside it", func(t *testing.T) {
		fresh := t.TempDir()
		stage(t, fresh)
		if err := os.Remove(filepath.Join(fresh, db.FileName+restore.StagedSuffix)); err != nil {
			t.Fatal(err)
		}
		got, err := restore.Adopt(ctx, fresh, time.Now())
		if got != nil || !errors.Is(err, restore.ErrPending) {
			t.Fatalf("Adopt = %v, %v; want ErrPending", got, err)
		}
		if strings.Contains(err.Error(), stagedCode) {
			t.Errorf("the refusal repeats the code: %v", err)
		}
		if exists(t, filepath.Join(fresh, restore.PendingFile)) {
			t.Error("the word was left to be tripped over at every start")
		}
	})
	t.Run("word that is not what was left", func(t *testing.T) {
		for _, body := range []string{"not json", `{}`, `{"venue_id":"ven_A1B2C3D4","extra":1}`} {
			fresh := t.TempDir()
			stage(t, fresh)
			if err := os.WriteFile(filepath.Join(fresh, restore.PendingFile), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := restore.Adopt(ctx, fresh, time.Now()); got != nil || !errors.Is(err, restore.ErrPending) {
				t.Errorf("%s: Adopt = %v, %v; want ErrPending", body, got, err)
			}
			if exists(t, filepath.Join(fresh, db.FileName)) {
				t.Errorf("%s: a database was put in place on word nobody could read", body)
			}
		}
	})
}
