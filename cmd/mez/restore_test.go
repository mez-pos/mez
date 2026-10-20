package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
)

// backedUpVenue makes a venue the way an owner does (`mez setup`), runs the
// server over it with a backup to a folder until it is serving, and stops
// it. It returns the venue's data directory, where its backup is, and the
// name of a photo on its menu.
func backedUpVenue(t *testing.T) (dataDir string, to backup.Config, photo string) {
	t.Helper()
	return backedUpVenueAfter(t, nil)
}

// backedUpVenueAfter is backedUpVenue with something done to the venue's
// data directory once it is set up and before it first runs and backs up.
func backedUpVenueAfter(t *testing.T, prepare func(dataDir string)) (dataDir string, to backup.Config, photo string) {
	t.Helper()
	dataDir = t.TempDir()
	to = backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "backups")}

	var said bytes.Buffer
	setup := []string{"-data-dir", dataDir, "-name", "The Corner Cafe", "-owner", "Priya", "-owner-pin", "246813", "-timezone", "Asia/Kolkata", "-demo-menu"}
	if err := runSetup(setup, &said); err != nil {
		t.Fatalf("setup: %v\n%s", err, said.String())
	}

	picture := []byte("a picture of a filter coffee")
	sum := sha256.Sum256(picture)
	photo = hex.EncodeToString(sum[:]) + ".jpg"
	if err := os.MkdirAll(filepath.Join(dataDir, "photos"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "photos", photo), picture, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(to)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, backup.ConfigFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if prepare != nil {
		prepare(dataDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, dataDir), runOptions{stdout: io.Discard}) }()
	waitForLog(t, dataDir, "listening", done)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("the server did not stop")
	}
	return dataDir, to, photo
}

// TestRestoreBringsAVenueBack is `mez restore` on the day it is needed:
// the venue's computer is gone, there is a new one with nothing on it, and
// the backup folder.
func TestRestoreBringsAVenueBack(t *testing.T) {
	_, to, photo := backedUpVenue(t)
	fresh := filepath.Join(t.TempDir(), "new-machine")

	var said bytes.Buffer
	if err := runRestore([]string{"-data-dir", fresh, "-from", to.Folder}, &said); err != nil {
		t.Fatalf("restore: %v\n%s", err, said.String())
	}
	out := said.String()
	for _, want := range []string{"The database is restored and checked", "1 menu photos restored", "The Corner Cafe: no invoices yet", "Backups are off on this copy"} {
		if !strings.Contains(out, want) {
			t.Errorf("restore did not say %q; it said:\n%s", want, out)
		}
	}
	if back, err := os.ReadFile(filepath.Join(fresh, "photos", photo)); err != nil || string(back) != "a picture of a filter coffee" {
		t.Errorf("the photo did not come back: %v", err)
	}
	// Not carried over: the restored copy does not start writing into the
	// backup it came from without being told to.
	if _, err := os.Stat(filepath.Join(fresh, backup.ConfigFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup.json in the restored directory: %v, want none", err)
	}

	// The server starts on it, as the venue it was: the staff are there
	// to sign in, and nothing asks for setup again.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, fresh), runOptions{stdout: io.Discard}) }()
	waitForLog(t, fresh, "listening", done)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the server on the restored venue: %v", err)
	}
	if log := readLog(t, fresh); strings.Contains(log, "has no venue yet") {
		t.Error("the restored venue was taken for a machine with no venue")
	}
	restored, err := db.Open(context.Background(), db.Options{Path: filepath.Join(fresh, db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(context.Background())
	var staff, items int
	if err := restored.Reader().QueryRow("SELECT (SELECT count(*) FROM staff), (SELECT count(*) FROM items)").Scan(&staff, &items); err != nil {
		t.Fatal(err)
	}
	if staff < 5 || items == 0 {
		t.Errorf("the restored venue has %d staff and %d items, want the demo cafe's", staff, items)
	}

	t.Run("it does not write over a venue", func(t *testing.T) {
		before, err := os.ReadFile(filepath.Join(fresh, db.FileName))
		if err != nil {
			t.Fatal(err)
		}
		var said bytes.Buffer
		err = runRestore([]string{"-data-dir", fresh, "-from", to.Folder}, &said)
		if !errors.Is(err, ErrVenueThere) {
			t.Fatalf("restore over a venue = %v, want ErrVenueThere", err)
		}
		after, _ := os.ReadFile(filepath.Join(fresh, db.FileName))
		if !bytes.Equal(before, after) {
			t.Error("the refused restore changed the venue it refused to write over")
		}
	})
}

// TestRestoreOntoANewInstall: the new computer has had mez installed and
// started, as the installer does, so there is a database, with no venue
// in it. That is not a venue to protect.
func TestRestoreOntoANewInstall(t *testing.T) {
	_, to, _ := backedUpVenue(t)

	// A new install: the server started once and stopped, nothing set up.
	fresh := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, fresh), runOptions{stdout: io.Discard}) }()
	waitForLog(t, fresh, "listening", done)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the new install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fresh, db.FileName)); err != nil {
		t.Fatalf("the new install made no database: %v", err)
	}

	var said bytes.Buffer
	if err := runRestore([]string{"-data-dir", fresh, "-from", to.Folder}, &said); err != nil {
		t.Fatalf("restore onto a new install: %v\n%s", err, said.String())
	}
	out := said.String()
	if !strings.Contains(out, "A new install with no venue was here") || !strings.Contains(out, "The Corner Cafe") {
		t.Errorf("restore said:\n%s", out)
	}
	// Moved aside, not deleted.
	aside, err := filepath.Glob(filepath.Join(fresh, db.FileName+".empty-*"))
	if err != nil || len(aside) != 1 {
		t.Errorf("the empty database moved aside: %v, %v; want one", aside, err)
	}

	// And the server comes up on it as the venue, with no wizard.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	// The new install's log is cleared, so that "listening" is this start's.
	if err := os.RemoveAll(filepath.Join(fresh, "logs")); err != nil {
		t.Fatal(err)
	}
	go func() { done <- run(ctx, serverArgs(t, fresh), runOptions{stdout: io.Discard}) }()
	waitForLog(t, fresh, "listening", done)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the server on the restored venue: %v", err)
	}
	restored, err := db.Open(context.Background(), db.Options{Path: filepath.Join(fresh, db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(context.Background())
	var name string
	if err := restored.Reader().QueryRow("SELECT name FROM venues").Scan(&name); err != nil || name != "The Corner Cafe" {
		t.Errorf("the venue in the restored database: %q, %v", name, err)
	}
}

// TestRestoreLeavesAVenueAsItFoundIt: looking at the database in the way
// must not change it. It may be a real venue's, made by an older mez.
func TestRestoreLeavesAVenueAsItFoundIt(t *testing.T) {
	dataDir, to, _ := backedUpVenue(t)

	// Made to look like an older mez's: a table this binary would add.
	path := filepath.Join(dataDir, db.FileName)
	old, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec("DELETE FROM goose_db_version WHERE version_id = (SELECT max(version_id) FROM goose_db_version)"); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := old.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	old.Close()

	var said bytes.Buffer
	err = runRestore([]string{"-data-dir", dataDir, "-from", to.Folder}, &said)
	if !errors.Is(err, ErrVenueThere) || !strings.Contains(err.Error(), "The Corner Cafe") {
		t.Fatalf("restore over a venue = %v, want ErrVenueThere naming the venue", err)
	}

	after, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	var now int
	if err := after.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&now); err != nil {
		t.Fatal(err)
	}
	if now != version {
		t.Errorf("the refused restore migrated the venue in the way from version %d to %d", version, now)
	}
}

// TestRestoreRefusesWhileMezIsRunning: a database moved out from under a
// running server is a server selling into a file nobody will find.
func TestRestoreRefusesWhileMezIsRunning(t *testing.T) {
	_, to, _ := backedUpVenue(t)

	fresh := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, fresh), runOptions{stdout: io.Discard}) }()
	waitForLog(t, fresh, "listening", done)

	var said bytes.Buffer
	err := runRestore([]string{"-data-dir", fresh, "-from", to.Folder}, &said)
	if !errors.Is(err, ErrStillRunning) {
		t.Errorf("restore under a running server = %v, want ErrStillRunning\n%s", err, said.String())
	}
	if aside, _ := filepath.Glob(filepath.Join(fresh, db.FileName+".empty-*")); len(aside) != 0 {
		t.Errorf("the running server's database was moved aside: %v", aside)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the server: %v", err)
	}
	// Stopped, it goes through.
	said.Reset()
	if err := runRestore([]string{"-data-dir", fresh, "-from", to.Folder}, &said); err != nil {
		t.Fatalf("restore once it is stopped: %v\n%s", err, said.String())
	}
}

func TestRestoreListsWhatIsThere(t *testing.T) {
	_, to, _ := backedUpVenue(t)

	var said bytes.Buffer
	if err := runRestore([]string{"-from", to.Folder, "-list"}, &said); err != nil {
		t.Fatalf("restore -list: %v", err)
	}
	found, err := backup.Generations(context.Background(), to)
	if err != nil || len(found) != 1 {
		t.Fatalf("Generations = %v, %v", found, err)
	}
	out := said.String()
	if !strings.Contains(out, found[0].Name) || !strings.Contains(out, "the one a restore takes") {
		t.Errorf("the list does not name the backup and say it is the one taken:\n%s", out)
	}

	// By name, into another directory.
	other := filepath.Join(t.TempDir(), "by-name")
	said.Reset()
	if err := runRestore([]string{"-data-dir", other, "-from", to.Folder, "-generation", found[0].Name}, &said); err != nil {
		t.Fatalf("restore -generation: %v\n%s", err, said.String())
	}

	t.Run("an empty folder", func(t *testing.T) {
		var said bytes.Buffer
		if err := runRestore([]string{"-from", t.TempDir(), "-list"}, &said); err != nil {
			t.Fatalf("restore -list: %v", err)
		}
		if !strings.Contains(said.String(), "There is no backup here") {
			t.Errorf("an empty folder listed as:\n%s", said.String())
		}
	})
}

func TestRestoreRefuses(t *testing.T) {
	_, to, _ := backedUpVenue(t)
	into := func() string { return filepath.Join(t.TempDir(), "into") }

	tests := []struct {
		name string
		args []string
		is   error
		says string
	}{
		{name: "nowhere to restore from", args: []string{"-data-dir", into()}, says: "say where the backup is"},
		{name: "two places to restore from", args: []string{"-data-dir", into(), "-from", to.Folder, "-bucket", "corner-cafe"}, says: "one way"},
		{name: "a folder that is not there", args: []string{"-data-dir", into(), "-from", filepath.Join(t.TempDir(), "unplugged")}, is: backup.ErrDestinationMissing},
		{name: "a folder with no backup in it", args: []string{"-data-dir", into(), "-from", t.TempDir()}, is: backup.ErrNothingToRestore},
		{name: "a backup that is not there", args: []string{"-data-dir", into(), "-from", to.Folder, "-generation", "20200101T000000Z-abcdef"}, is: backup.ErrNoSuchGeneration},
		{name: "a bucket with no secret in the environment", args: []string{"-data-dir", into(), "-bucket", "corner-cafe", "-region", "ap-south-1", "-access-key-id", "AKID"}, says: restoreSecretEnv},
		{name: "into the backup's own folder", args: []string{"-data-dir", to.Folder, "-from", to.Folder}, is: backup.ErrInvalidConfig},
		{name: "a stray argument", args: []string{"-from", to.Folder, "now"}, says: "unexpected argument"},
		{name: "a config file by another name", args: []string{"-data-dir", into(), "-config", filepath.Join(t.TempDir(), "saved.json")}, says: backup.ConfigFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(restoreSecretEnv, "")
			var said bytes.Buffer
			err := runRestore(tt.args, &said)
			if err == nil {
				t.Fatalf("restore succeeded, want it refused:\n%s", said.String())
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Errorf("restore = %v, want %v", err, tt.is)
			}
			if tt.says != "" && !strings.Contains(err.Error(), tt.says) {
				t.Errorf("restore = %v, want it to say %q", err, tt.says)
			}
			// A refusal leaves no half-made venue behind.
			for i, a := range tt.args {
				if a == "-data-dir" {
					if _, statErr := os.Stat(filepath.Join(tt.args[i+1], db.FileName)); statErr == nil {
						t.Error("a refused restore left a database behind")
					}
				}
			}
		})
	}

	t.Run("-h is a question", func(t *testing.T) {
		var said bytes.Buffer
		if err := runRestore([]string{"-h"}, &said); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("restore -h = %v, want flag.ErrHelp", err)
		}
		if !strings.Contains(said.String(), "mez restore -from") || strings.Contains(said.String(), "secret-access-key") {
			t.Errorf("the help:\n%s", said.String())
		}
	})
}

// TestRestoreFromASavedBackupFile: the destination read from a backup.json
// kept from the old machine, the way an owner with a bucket would have it.
func TestRestoreFromASavedBackupFile(t *testing.T) {
	_, to, _ := backedUpVenue(t)
	kept := t.TempDir()
	raw, err := json.Marshal(to)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, backup.ConfigFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	var said bytes.Buffer
	fresh := filepath.Join(t.TempDir(), "new-machine")
	if err := runRestore([]string{"-data-dir", fresh, "-config", filepath.Join(kept, backup.ConfigFile)}, &said); err != nil {
		t.Fatalf("restore -config: %v\n%s", err, said.String())
	}
	if _, err := os.Stat(filepath.Join(fresh, db.FileName)); err != nil {
		t.Errorf("no database restored: %v", err)
	}
}
