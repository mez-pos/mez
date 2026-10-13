package backup_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
)

func (v venue) photosDir() string { return filepath.Join(v.dataDir, "photos") }

// addPhoto puts a photo in the venue's folder the way the menu does: its
// bytes under the name of their hash. The bytes are not a picture; nothing
// here looks inside one.
func addPhoto(t *testing.T, dir string) string {
	t.Helper()
	body := make([]byte, 4096)
	if _, err := rand.Read(body); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	name := hex.EncodeToString(sum[:]) + ".jpg"
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

// photosIn names the photos in a folder, sorted. No folder is none.
func photosIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func sameNames(a, b []string) bool {
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, ",") == strings.Join(b, ",")
}

// startWithPhotos starts a venue's backup with its photos folder and a
// probe quick enough to watch.
func startWithPhotos(t *testing.T, v venue, probe time.Duration) (*db.DB, *backup.Engine) {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, PhotosDir: v.photosDir(), ProbeEvery: probe})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { database.Close(ctx); engine.Close(ctx) })
	return database, engine
}

// TestPhotosGoToAFolderAndComeBack is the photos' reason: a venue restored
// from its backup has its menu's pictures, not grey boxes.
func TestPhotosGoToAFolderAndComeBack(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	first, second := addPhoto(t, v.photosDir()), addPhoto(t, v.photosDir())

	// Things in the folder that are not photos, and one that says it is.
	if err := os.WriteFile(filepath.Join(v.photosDir(), ".tmp-123"), []byte("half written"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.photosDir(), "notes.txt"), []byte("not a photo"), 0o600); err != nil {
		t.Fatal(err)
	}
	damaged := strings.Repeat("ab", 32) + ".jpg"
	if err := os.WriteFile(filepath.Join(v.photosDir(), damaged), []byte("not what the name says"), 0o600); err != nil {
		t.Fatal(err)
	}

	// An hour between looks: only "back up now" can have sent them.
	_, engine := startWithPhotos(t, v, time.Hour)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	at := filepath.Join(v.cfg.Folder, "photos")
	if got := photosIn(t, at); !sameNames(got, []string{first, second}) {
		t.Errorf("the destination holds %v, want the two photos and nothing else", got)
	}

	// A third, and only it goes: the first two are not written again.
	before, err := os.Stat(filepath.Join(at, first))
	if err != nil {
		t.Fatal(err)
	}
	third := addPhoto(t, v.photosDir())
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	after, err := os.Stat(filepath.Join(at, first))
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("a photo already at the destination was written again")
	}

	// Back, into a venue being put together again that already has one.
	restoreTo := filepath.Join(t.TempDir(), "photos")
	if err := os.MkdirAll(restoreTo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restoreTo, first), []byte("already here"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := backup.RestorePhotos(ctx, v.cfg, restoreTo)
	if err != nil {
		t.Fatalf("RestorePhotos: %v", err)
	}
	if n != 2 {
		t.Errorf("restored %d photos, want the 2 that were missing", n)
	}
	if got := photosIn(t, restoreTo); !sameNames(got, []string{first, second, third}) {
		t.Errorf("restored %v, want all three", got)
	}
	for _, name := range []string{second, third} {
		want, _ := os.ReadFile(filepath.Join(v.photosDir(), name))
		got, _ := os.ReadFile(filepath.Join(restoreTo, name))
		if string(got) != string(want) || len(got) == 0 {
			t.Errorf("photo %s did not come back as it went", name)
		}
	}
	if left, _ := os.ReadFile(filepath.Join(restoreTo, first)); string(left) != "already here" {
		t.Error("a photo already in the folder was written over")
	}
}

// TestAPhotoAddedDuringServiceGoesByItself: nobody presses a button when
// they change the menu.
func TestAPhotoAddedDuringServiceGoesByItself(t *testing.T) {
	v := newVenue(t)
	_, _ = startWithPhotos(t, v, 20*time.Millisecond)

	name := addPhoto(t, v.photosDir())
	at := filepath.Join(v.cfg.Folder, "photos", name)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(at); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the photo was never copied")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestAVenueWithNoPhotos has no folder at all, and that is not an error.
func TestAVenueWithNoPhotos(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	database, engine := startWithPhotos(t, v, time.Hour)
	sell(t, database, 1, 3)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if got := engine.Status(); got.State != backup.StateOK {
		t.Errorf("Status = %+v, want ok", got)
	}
	n, err := backup.RestorePhotos(ctx, v.cfg, filepath.Join(t.TempDir(), "photos"))
	if err != nil || n != 0 {
		t.Errorf("RestorePhotos = %d, %v, want none and no error", n, err)
	}
}

// TestPhotosThatCannotBeCopiedAreSaid: with the sales safe and the
// pictures not, "ok" would be read as everything.
func TestPhotosThatCannotBeCopiedAreSaid(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	addPhoto(t, v.photosDir())
	// A file where the destination's photos folder should be.
	if err := os.MkdirAll(v.cfg.Folder, 0o750); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(v.cfg.Folder, "photos")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}

	database, engine := startWithPhotos(t, v, 20*time.Millisecond)
	sell(t, database, 1, 3)
	if err := engine.BackUpNow(ctx); err == nil {
		t.Fatal("BackUpNow succeeded with nowhere to put the photos")
	}
	got := engine.Status()
	if got.State != backup.StateFailing || !strings.HasPrefix(got.Error, "photos: ") {
		t.Errorf("Status = %+v, want failing and saying it is the photos", got)
	}
	// The database went all the same.
	if rows, _ := restored(t, v.cfg); rows != 3 {
		t.Errorf("restored %d rows, want the 3 sold: the photos must not hold the sales back", rows)
	}

	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	waitForState(t, engine, backup.StateOK)
}

// TestADamagedPhotoAtTheDestinationIsNotRestored: the name is the check,
// and a file that fails it is not put on a menu.
func TestADamagedPhotoAtTheDestinationIsNotRestored(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	good := addPhoto(t, v.photosDir())
	bad := addPhoto(t, v.photosDir())
	_, engine := startWithPhotos(t, v, time.Hour)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(v.cfg.Folder, "photos", bad), []byte("rotted on the drive"), 0o600); err != nil {
		t.Fatal(err)
	}

	to := filepath.Join(t.TempDir(), "photos")
	n, err := backup.RestorePhotos(ctx, v.cfg, to)
	if !errors.Is(err, backup.ErrPhotoDamaged) {
		t.Fatalf("RestorePhotos = %v, want ErrPhotoDamaged", err)
	}
	if n != 1 {
		t.Errorf("restored %d, want the one good photo", n)
	}
	if got := photosIn(t, to); !sameNames(got, []string{good}) {
		t.Errorf("restored %v, want only %s", got, good)
	}
}

// TestAPhotoDeletedAtTheDestinationGoesAgain: "back up now" asks the
// destination what it holds and does not trust what it remembers.
func TestAPhotoDeletedAtTheDestinationGoesAgain(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	name := addPhoto(t, v.photosDir())
	_, engine := startWithPhotos(t, v, time.Hour)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	at := filepath.Join(v.cfg.Folder, "photos", name)
	if err := os.Remove(at); err != nil {
		t.Fatalf("deleting it at the destination: %v", err)
	}
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if _, err := os.Stat(at); err != nil {
		t.Errorf("the photo was not sent again: %v", err)
	}
}

// TestABucketOfSomeoneElsesKeys: what is restored is what is named like a
// photo and hashes like one, whatever else has been put under photos/.
func TestABucketOfSomeoneElsesKeys(t *testing.T) {
	fake, srv := newFakeS3(t, testBucket, testKey)
	v := bucketVenue(t, srv.URL)
	ctx := context.Background()
	good := addPhoto(t, v.photosDir())
	_, engine := startWithPhotos(t, v, time.Hour)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}

	huge := strings.Repeat("cd", 32) + ".jpg"
	fake.Plant("mez/photos/../../escape.jpg", []byte("x"))
	fake.Plant("mez/photos/sub/"+good, []byte("x"))
	fake.Plant("mez/photos/notes.txt", []byte("x"))
	fake.Plant("mez/photos/"+huge, make([]byte, 3<<20))

	parent := t.TempDir()
	to := filepath.Join(parent, "venue", "photos")
	n, err := backup.RestorePhotos(ctx, v.cfg, to)
	if !errors.Is(err, backup.ErrPhotoDamaged) {
		t.Errorf("RestorePhotos = %v, want the oversized one refused as damaged", err)
	}
	if n != 1 {
		t.Errorf("restored %d, want the one photo", n)
	}
	if got := photosIn(t, to); !sameNames(got, []string{good}) {
		t.Errorf("the folder holds %v, want only %s", got, good)
	}
	if got := photosIn(t, parent); !sameNames(got, []string{"venue"}) {
		t.Errorf("something was written outside the photos folder: %v", got)
	}
}

func TestRestorePhotosRefuses(t *testing.T) {
	ctx := context.Background()
	if _, err := backup.RestorePhotos(ctx, backup.Config{}, t.TempDir()); !errors.Is(err, backup.ErrOff) {
		t.Errorf("with no destination: %v, want ErrOff", err)
	}
	missing := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "unplugged")}
	if _, err := backup.RestorePhotos(ctx, missing, filepath.Join(t.TempDir(), "photos")); !errors.Is(err, backup.ErrDestinationMissing) {
		t.Errorf("from a folder that is not there: %v, want ErrDestinationMissing", err)
	}
}

// TestPhotosGoToABucketAndComeBack is the folder's test through the
// stand-in S3, and counts what it was asked.
func TestPhotosGoToABucketAndComeBack(t *testing.T) {
	fake, srv := newFakeS3(t, testBucket, testKey)
	v := bucketVenue(t, srv.URL)
	ctx := context.Background()
	first, second := addPhoto(t, v.photosDir()), addPhoto(t, v.photosDir())

	database, engine := startWithPhotos(t, v, 20*time.Millisecond)
	sell(t, database, 1, 3)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}

	photoKeys := func() []string {
		var names []string
		for _, k := range fake.Keys() {
			if name, ok := strings.CutPrefix(k, "mez/photos/"); ok {
				names = append(names, name)
			}
		}
		return names
	}
	if got := photoKeys(); !sameNames(got, []string{first, second}) {
		t.Errorf("the bucket holds %v under mez/photos/, want the two photos", got)
	}

	// Left alone, the probe ticks many times and sends nothing: a photo
	// is written to a bucket once.
	puts := fake.CountPrefix("PUT", "mez/photos/")
	time.Sleep(300 * time.Millisecond)
	if again := fake.CountPrefix("PUT", "mez/photos/"); again != puts {
		t.Errorf("photos were written %d more times with nothing new", again-puts)
	}
	// Nor is the bucket asked for its list of photos on every tick.
	lists := fake.CountPrefix("LIST", "mez/photos/")
	time.Sleep(300 * time.Millisecond)
	if again := fake.CountPrefix("LIST", "mez/photos/"); again != lists {
		t.Errorf("the photos were listed %d more times with nothing new", again-lists)
	}

	to := filepath.Join(t.TempDir(), "photos")
	n, err := backup.RestorePhotos(ctx, v.cfg, to)
	if err != nil || n != 2 {
		t.Fatalf("RestorePhotos = %d, %v, want 2", n, err)
	}
	for _, name := range []string{first, second} {
		want, _ := os.ReadFile(filepath.Join(v.photosDir(), name))
		got, _ := os.ReadFile(filepath.Join(to, name))
		if string(got) != string(want) || len(got) == 0 {
			t.Errorf("photo %s did not come back from the bucket as it went", name)
		}
	}
}
