package backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
)

// managed is a venue whose backup is run by a Manager, the way the server
// runs it: the database opened and migrated, then the Manager started.
type managed struct {
	venue
	database *db.DB
	manager  *backup.Manager
}

func newManaged(t *testing.T, v venue, live bool) managed {
	t.Helper()
	ctx := context.Background()
	manager, err := backup.NewManager(backup.ManagerOptions{
		DataDir: v.dataDir, DBPath: v.dbPath(), PhotosDir: v.photosDir(), Log: testLog(t),
		// Looked at often, so that a test of what the looking finds is
		// not a test of waiting fifteen seconds.
		ProbeEvery: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	manager.SwapsLive(live)
	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { database.Close(ctx); manager.Close(ctx) })
	return managed{venue: v, database: database, manager: manager}
}

func folderAt(t *testing.T, name string) backup.Config {
	t.Helper()
	return backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), name)}
}

// TestSavingADestinationStartsTheBackup is the owner's first visit to the
// Backups section: nothing is being backed up, they name a folder, and
// from then on it is.
func TestSavingADestinationStartsTheBackup(t *testing.T) {
	ctx := context.Background()
	// Not live, as on Linux: a backup that is off has nothing to stop,
	// so it starts at once all the same.
	m := newManaged(t, venue{dataDir: t.TempDir()}, false)
	if got := m.manager.Status(); got.State != backup.StateOff {
		t.Fatalf("Status = %+v before anything is saved, want off", got)
	}
	sell(t, m.database, 1, 20)

	to := folderAt(t, "backups")
	restart, err := m.manager.Set(ctx, to)
	if err != nil || restart {
		t.Fatalf("Set = restart %v, %v; want it to take effect at once", restart, err)
	}
	if m.manager.RestartNeeded() {
		t.Error("RestartNeeded after a backup was turned on, want none")
	}
	if got := m.manager.Config(); got != to {
		t.Errorf("Config = %+v, want what was saved", got)
	}
	sell(t, m.database, 21, 20)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if rows, _ := restored(t, to); rows != 40 {
		t.Errorf("restored %d rows, want the 20 sold before it was turned on and the 20 after", rows)
	}

	// Kept for the next start, and a note left for whoever finds the folder.
	kept, err := backup.LoadConfig(m.dataDir)
	if err != nil || kept != to {
		t.Errorf("backup.json holds %+v, %v; want what was saved", kept, err)
	}
	note, err := os.ReadFile(filepath.Join(to.Folder, "mez-backup.txt"))
	if err != nil || !strings.Contains(string(note), "mez point of sale") {
		t.Errorf("the note at the destination: %q, %v", note, err)
	}
}

func TestADestinationThatCannotBeUsedIsNotSaved(t *testing.T) {
	ctx := context.Background()
	good := folderAt(t, "backups")
	v := venue{dataDir: t.TempDir(), cfg: good}
	if err := backup.SaveConfig(v.dataDir, good); err != nil {
		t.Fatal(err)
	}
	m := newManaged(t, v, true)

	// A file where the folder would be: written correctly, and unusable.
	blocked := folderAt(t, "blocked")
	if err := os.WriteFile(blocked.Folder, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.manager.Set(ctx, blocked); !errors.Is(err, backup.ErrUnusable) {
		t.Fatalf("Set to a folder that cannot be made = %v, want ErrUnusable", err)
	}

	// Written wrongly: refused, and which part said.
	var invalid *backup.InvalidConfig
	_, err := m.manager.Set(ctx, backup.Config{Kind: backup.KindFolder, Folder: "relative"})
	if !errors.As(err, &invalid) || invalid.Field != "folder" {
		t.Fatalf("Set to a relative folder = %v, want an InvalidConfig about the folder", err)
	}
	_, err = m.manager.Set(ctx, backup.Config{Kind: backup.KindS3, S3: backup.S3{Bucket: "corner-cafe", Region: "ap-south-1"}})
	if !errors.As(err, &invalid) || invalid.Field != "secret_access_key" {
		t.Fatalf("Set to a bucket with no key = %v, want an InvalidConfig about the key", err)
	}

	// Nothing moved: the same destination, saved and running.
	if got := m.manager.Config(); got != good {
		t.Errorf("Config = %+v after three refusals, want the one it had", got)
	}
	if kept, _ := backup.LoadConfig(v.dataDir); kept != good {
		t.Errorf("backup.json = %+v after three refusals, want the one it had", kept)
	}
	sell(t, m.database, 1, 10)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if rows, _ := restored(t, good); rows != 10 {
		t.Errorf("restored %d rows from the destination it kept, want 10", rows)
	}
}

// TestChangingTheDestinationWhileSelling is the swap, where it is done
// live: nothing sold is missing from either side of it.
func TestChangingTheDestinationWhileSelling(t *testing.T) {
	backup.HurryLitestream(t)
	ctx := context.Background()
	a, b := folderAt(t, "first"), folderAt(t, "second")
	v := venue{dataDir: t.TempDir(), cfg: a}
	if err := backup.SaveConfig(v.dataDir, a); err != nil {
		t.Fatal(err)
	}
	m := newManaged(t, v, true)
	addPhoto(t, v.photosDir())

	sell(t, m.database, 1, 30)
	restart, err := m.manager.Set(ctx, b)
	if err != nil || restart {
		t.Fatalf("Set = restart %v, %v; want a live swap", restart, err)
	}
	sell(t, m.database, 31, 30)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}

	if rows, _ := restored(t, a); rows != 30 {
		t.Errorf("the destination left behind restores %d rows, want the 30 sold while it was the destination", rows)
	}
	if rows, _ := restored(t, b); rows != 60 {
		t.Errorf("the new destination restores %d rows, want all 60", rows)
	}
	if got := photosIn(t, filepath.Join(b.Folder, "photos")); len(got) != 1 {
		t.Errorf("the new destination holds %d photos, want the one on the menu", len(got))
	}

	// Saving the same again changes nothing and stops nothing.
	if restart, err := m.manager.Set(ctx, b); err != nil || restart {
		t.Errorf("Set to what it already is = restart %v, %v", restart, err)
	}

	// And off: no file, nothing running, and no error for asking.
	if restart, err := m.manager.Set(ctx, backup.Config{}); err != nil || restart {
		t.Fatalf("Set to off = restart %v, %v", restart, err)
	}
	if got := m.manager.Status(); got.State != backup.StateOff {
		t.Errorf("Status = %+v after turning it off, want off", got)
	}
	if _, err := os.Stat(filepath.Join(v.dataDir, backup.ConfigFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup.json after turning it off: %v, want it gone", err)
	}
	if err := m.manager.BackUpNow(ctx); !errors.Is(err, backup.ErrOff) {
		t.Errorf("BackUpNow with it off = %v, want ErrOff", err)
	}
	sell(t, m.database, 61, 5) // the till still works
}

// TestWhereARunningBackupCannotBeStopped is every system but Windows
// (ADR-276): the new destination is saved, the old one goes on being
// copied to, and the next start uses the new.
func TestWhereARunningBackupCannotBeStopped(t *testing.T) {
	ctx := context.Background()
	a, b := folderAt(t, "first"), folderAt(t, "second")
	v := venue{dataDir: t.TempDir(), cfg: a}
	if err := backup.SaveConfig(v.dataDir, a); err != nil {
		t.Fatal(err)
	}
	m := newManaged(t, v, false)
	sell(t, m.database, 1, 20)

	restart, err := m.manager.Set(ctx, b)
	if err != nil || !restart {
		t.Fatalf("Set = restart %v, %v; want it saved and a restart asked for", restart, err)
	}
	if !m.manager.RestartNeeded() {
		t.Error("RestartNeeded = false with a destination waiting")
	}
	if got := m.manager.Config(); got != b {
		t.Errorf("Config = %+v, want the one saved", got)
	}

	// Still the old one, until then.
	sell(t, m.database, 21, 20)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if rows, _ := restored(t, a); rows != 40 {
		t.Errorf("the old destination restores %d rows, want all 40: it is still the one in use", rows)
	}

	// Saving the old one back owes nothing.
	if restart, err := m.manager.Set(ctx, a); err != nil || restart || m.manager.RestartNeeded() {
		t.Errorf("Set back to what is running = restart %v, %v", restart, err)
	}
	if restart, err := m.manager.Set(ctx, b); err != nil || !restart {
		t.Fatalf("Set = restart %v, %v", restart, err)
	}

	// The restart.
	if err := m.database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.manager.Close(ctx); err != nil {
		t.Fatal(err)
	}
	again := newManaged(t, venue{dataDir: v.dataDir, cfg: b}, false)
	if again.manager.RestartNeeded() {
		t.Error("RestartNeeded after the restart")
	}
	sell(t, again.database, 41, 10)
	if err := again.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if rows, _ := restored(t, b); rows != 50 {
		t.Errorf("the new destination restores %d rows after the restart, want all 50", rows)
	}
}

// TestANewDriveIsFilledWithoutBeingAsked: the owner swaps the USB drive
// for an empty one and tells nobody. Where a running backup can be
// stopped, it is, and a new one sends the whole database to the new drive.
func TestANewDriveIsFilledWithoutBeingAsked(t *testing.T) {
	backup.HurryLitestream(t)
	ctx := context.Background()
	to := folderAt(t, "usb")
	v := venue{dataDir: t.TempDir(), cfg: to}
	if err := backup.SaveConfig(v.dataDir, to); err != nil {
		t.Fatal(err)
	}
	m := newManaged(t, v, true)

	sell(t, m.database, 1, 30)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	// Long enough, hurried, for Litestream to have tidied away the small
	// files that could have rebuilt the backup from nothing.
	time.Sleep(700 * time.Millisecond)

	// The new drive: the same folder, with nothing in it.
	deadline := time.Now().Add(10 * time.Second)
	for os.RemoveAll(to.Folder) != nil {
		if time.Now().After(deadline) {
			t.Fatal("could not empty the destination")
		}
		time.Sleep(20 * time.Millisecond)
	}
	sell(t, m.database, 31, 30)

	// Nobody presses anything. It is noticed, started again, and whole.
	deadline = time.Now().Add(30 * time.Second)
	for {
		if m.manager.Status().State == backup.StateOK {
			if rows := tryRestore(t, to); rows == 60 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the new drive was never filled: status %+v, restores %d rows of 60", m.manager.Status(), tryRestore(t, to))
		}
		time.Sleep(100 * time.Millisecond)
	}

	// And the till went on selling throughout, and still backs up.
	sell(t, m.database, 61, 10)
	if err := m.manager.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow after the new drive: %v", err)
	}
	if rows, _ := restored(t, to); rows != 70 {
		t.Errorf("restored %d rows, want all 70", rows)
	}
}

// tryRestore is restored for a backup that may not be there yet: the
// probe rows it holds, or -1.
func tryRestore(t *testing.T, cfg backup.Config) int {
	t.Helper()
	ctx := context.Background()
	out := filepath.Join(t.TempDir(), db.FileName)
	if err := backup.Restore(ctx, cfg, out); err != nil {
		return -1
	}
	database, err := db.Open(ctx, db.Options{Path: out})
	if err != nil {
		return -1
	}
	defer database.Close(ctx)
	var rows int
	if err := database.Reader().QueryRowContext(ctx, "SELECT count(*) FROM backup_probe").Scan(&rows); err != nil {
		return -1
	}
	return rows
}

// TestAStoppedManagerDoesNothingMore: a request that outlives the
// shutdown must not start a backup nobody will stop.
func TestAStoppedManagerDoesNothingMore(t *testing.T) {
	ctx := context.Background()
	to := folderAt(t, "backups")
	v := venue{dataDir: t.TempDir(), cfg: to}
	if err := backup.SaveConfig(v.dataDir, to); err != nil {
		t.Fatal(err)
	}
	m := newManaged(t, v, true)
	sell(t, m.database, 1, 5)
	if err := m.database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.manager.Close(ctx); err != nil {
		t.Fatal(err)
	}

	if err := m.manager.BackUpNow(ctx); !errors.Is(err, backup.ErrStopping) {
		t.Errorf("BackUpNow after Close = %v, want ErrStopping", err)
	}
	if _, err := m.manager.Set(ctx, folderAt(t, "elsewhere")); !errors.Is(err, backup.ErrStopping) {
		t.Errorf("Set after Close = %v, want ErrStopping", err)
	}
	if err := m.manager.Close(ctx); err != nil {
		t.Errorf("Close twice: %v", err)
	}
	if rows, _ := restored(t, to); rows != 5 {
		t.Errorf("restored %d rows, want the 5 sold before it stopped", rows)
	}
}

func TestAManagerRefusesAFileItCannotRead(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, backup.ConfigFile), []byte(`{"kind":"folder"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := backup.NewManager(backup.ManagerOptions{DataDir: dataDir, DBPath: filepath.Join(dataDir, db.FileName)})
	if !errors.Is(err, backup.ErrInvalidConfig) {
		t.Fatalf("NewManager = %v, want ErrInvalidConfig", err)
	}
}

// TestTheSavedFile covers backup.json as written: read back the same,
// its owner's alone where modes mean something, and left as it was by a
// save that is refused.
func TestTheSavedFile(t *testing.T) {
	dataDir := t.TempDir()
	cfg := backup.Config{Kind: backup.KindS3, S3: backup.S3{Bucket: "corner-cafe", Region: "ap-south-1", AccessKeyID: "AKID", SecretAccessKey: "shh"}}
	if err := backup.SaveConfig(dataDir, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := backup.LoadConfig(dataDir)
	if err != nil || got != cfg {
		t.Fatalf("LoadConfig = %+v, %v; want what was saved", got, err)
	}
	info, err := os.Stat(filepath.Join(dataDir, backup.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); os.PathSeparator == '/' && mode&0o077 != 0 {
		t.Errorf("backup.json is %v, want it its owner's alone", mode)
	}
	if err := backup.SaveConfig(dataDir, backup.Config{Kind: backup.KindFolder}); !errors.Is(err, backup.ErrInvalidConfig) {
		t.Errorf("SaveConfig of a folder with no name = %v, want ErrInvalidConfig", err)
	}
	if got, _ := backup.LoadConfig(dataDir); got != cfg {
		t.Error("a refused save changed the file")
	}
}
