package cloudkit_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mez-pos/mez/cloudkit"
	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/backup/s3fake"
	"github.com/mez-pos/mez/internal/db"
)

// The venue below is the venue's own backup, internal/backup, writing to a
// stand-in bucket; the cloud reads it back through the door alone.

const (
	restoreBucket = "mez-v-0123456789ab"
	restoreKey    = "AKIDRESTORE"
	restoreSecret = "not-a-real-secret"
)

// backedUpVenue backs a venue's database, holding rows sales and one photo,
// up to a stand-in bucket, and answers the bucket as a pairing carries it
// and the photo's name.
func backedUpVenue(t *testing.T, sales int) (cloudkit.Bucket, string) {
	t.Helper()
	ctx := context.Background()
	srv := httptest.NewServer(s3fake.New(restoreBucket, restoreKey))
	t.Cleanup(srv.Close)
	b := cloudkit.Bucket{Bucket: restoreBucket, Path: "v/" + slug, Endpoint: srv.URL, AccessKeyID: restoreKey, SecretAccessKey: restoreSecret}

	dataDir := t.TempDir()
	photos := filepath.Join(dataDir, "photos")
	body := make([]byte, 2048)
	if _, err := rand.Read(body); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	photo := hex.EncodeToString(sum[:]) + ".jpg"
	if err := os.MkdirAll(photos, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(photos, photo), body, 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dataDir, db.FileName)
	database, err := db.Open(ctx, db.Options{Path: path, LogKeptBy: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: path, Config: backup.Config{Kind: backup.KindS3, S3: b}, PhotosDir: photos})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Writer().ExecContext(ctx, "CREATE TABLE sales_probe (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= sales; i++ {
		if err := database.Tx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO sales_probe (id) VALUES (?)", i)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if err := database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatal(err)
	}
	return b, photo
}

// TestTheCloudRestoresAVenuesBackupThroughTheDoor: the newest copy in the
// bucket, whole, in a new file the venue's own database opens, and the
// photos beside it; a file already there is never written over.
func TestTheCloudRestoresAVenuesBackupThroughTheDoor(t *testing.T) {
	ctx := context.Background()
	b, photo := backedUpVenue(t, 30)

	out := filepath.Join(t.TempDir(), "restored.db")
	if err := cloudkit.RestoreBackup(ctx, b, out); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	restored, err := db.Open(ctx, db.Options{Path: out})
	if err != nil {
		t.Fatalf("opening the restored database: %v", err)
	}
	var rows int
	if err := restored.Reader().QueryRowContext(ctx, "SELECT count(*) FROM sales_probe").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	_ = restored.Close(ctx)
	if rows != 30 {
		t.Errorf("restored %d sales, want 30", rows)
	}

	dir := filepath.Join(t.TempDir(), "photos")
	n, err := cloudkit.RestorePhotos(ctx, b, dir)
	if err != nil || n != 1 {
		t.Fatalf("RestorePhotos: %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dir, photo)); err != nil {
		t.Errorf("the photo is not there: %v", err)
	}

	if err := cloudkit.RestoreBackup(ctx, b, out); !errors.Is(err, cloudkit.ErrWouldOverwrite) {
		t.Errorf("restoring over a file: %v, want ErrWouldOverwrite", err)
	}
}

// TestAnEmptyBucketHasNothingToRestore.
func TestAnEmptyBucketHasNothingToRestore(t *testing.T) {
	srv := httptest.NewServer(s3fake.New(restoreBucket, restoreKey))
	t.Cleanup(srv.Close)
	b := cloudkit.Bucket{Bucket: restoreBucket, Path: "v/" + slug, Endpoint: srv.URL, AccessKeyID: restoreKey, SecretAccessKey: restoreSecret}
	err := cloudkit.RestoreBackup(context.Background(), b, filepath.Join(t.TempDir(), "restored.db"))
	if !errors.Is(err, cloudkit.ErrNothingToRestore) {
		t.Errorf("an empty bucket: %v, want ErrNothingToRestore", err)
	}
}
