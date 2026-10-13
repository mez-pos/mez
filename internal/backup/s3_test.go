package backup_test

import (
	"context"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/backup/s3fake"
	"github.com/byjoellobo/mez/internal/db"
)

const (
	testBucket = "the-corner-cafe"
	testKey    = "AKIDTEST"
	testSecret = "not-a-real-secret"
)

// bucketVenue is a venue whose backup goes to the stand-in S3.
func bucketVenue(t *testing.T, endpoint string) venue {
	t.Helper()
	return venue{
		dataDir: t.TempDir(),
		cfg: backup.Config{Kind: backup.KindS3, S3: backup.S3{
			Bucket: testBucket, Endpoint: endpoint, AccessKeyID: testKey, SecretAccessKey: testSecret,
		}},
	}
}

// TestABucketHoldsWhatWasWritten is the folder's test again, to a bucket:
// restored from the bucket alone, everything is there, across a restart.
func TestABucketHoldsWhatWasWritten(t *testing.T) {
	fake, srv := newFakeS3(t, testBucket, testKey)
	v := bucketVenue(t, srv.URL)
	ctx := context.Background()

	database, engine, stop := v.open(t)
	sell(t, database, 1, 40)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	wantTables := tableCount(t, database)
	if rows, tables := restored(t, v.cfg); rows != 40 || tables != wantTables {
		t.Errorf("restored after BackUpNow: %d rows and %d tables, want 40 and %d", rows, tables, wantTables)
	}
	if got := engine.Status(); got.State != backup.StateOK {
		t.Errorf("Status = %+v, want ok", got)
	}
	sell(t, database, 41, 25)
	stop()

	database, _, stop = v.open(t)
	sell(t, database, 66, 10)
	stop()

	if rows, _ := restored(t, v.cfg); rows != 75 {
		t.Errorf("restored %d rows after a restart, want 75", rows)
	}

	// Everything under mez/db/, so the photos can go beside it and a
	// shared bucket is not written to at its root.
	keys := fake.Keys()
	if len(keys) == 0 {
		t.Fatal("the bucket is empty")
	}
	for _, k := range keys {
		if !strings.HasPrefix(k, "mez/db/") {
			t.Errorf("wrote %q, want everything under mez/db/", k)
		}
	}
}

func TestABucketPathOfTheOwnersChoosing(t *testing.T) {
	fake, srv := newFakeS3(t, testBucket, testKey)
	v := bucketVenue(t, srv.URL)
	v.cfg.S3.Path = "venues/corner"

	database, engine, _ := v.open(t)
	sell(t, database, 1, 5)
	if err := engine.BackUpNow(context.Background()); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	for _, k := range fake.Keys() {
		if !strings.HasPrefix(k, "venues/corner/db/") {
			t.Errorf("wrote %q, want everything under venues/corner/db/", k)
		}
	}
}

// TestABucketThatRefuses covers the two mistakes an owner makes typing
// these in. Neither stops the till; both say why.
func TestABucketThatRefuses(t *testing.T) {
	tests := []struct {
		name   string
		change func(*backup.S3)
		want   string // in the status's reason
	}{
		{name: "a key it does not know", change: func(s *backup.S3) { s.AccessKeyID = "AKIDWRONG" }, want: "InvalidAccessKeyId"},
		{name: "a bucket that is not there", change: func(s *backup.S3) { s.Bucket = "some-other-cafe" }, want: "NoSuchBucket"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, srv := newFakeS3(t, testBucket, testKey)
			v := bucketVenue(t, srv.URL)
			tt.change(&v.cfg.S3)
			ctx := context.Background()

			database, engine, _ := v.open(t)
			sell(t, database, 1, 5)
			err := engine.BackUpNow(ctx)
			if err == nil {
				t.Fatal("BackUpNow succeeded, want it refused")
			}
			if strings.Contains(err.Error(), testSecret) {
				t.Errorf("the error carries the secret: %v", err)
			}
			got := engine.Status()
			if got.State != backup.StateFailing || !strings.Contains(got.Error, tt.want) {
				t.Errorf("Status = %+v, want failing and saying %s", got, tt.want)
			}
			if strings.Contains(got.Error, testSecret) {
				t.Errorf("the status carries the secret: %q", got.Error)
			}
			sell(t, database, 6, 5) // the till still works

			out := filepath.Join(t.TempDir(), db.FileName)
			if err := backup.Restore(ctx, v.cfg, out); err == nil {
				t.Error("Restore succeeded from a bucket that refuses")
			}
		})
	}
}

// TestNoInternetStopsNothing is the venue's normal morning: the bucket
// cannot be reached. The backup starts, says it is failing, sales go
// through, the server stops promptly, and when the line comes back
// everything written meanwhile goes.
func TestNoInternetStopsNothing(t *testing.T) {
	// An address nothing is listening on, then the stand-in at that address.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	v := bucketVenue(t, "http://"+addr)
	ctx := context.Background()

	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, ProbeEvery: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("Start with no internet: %v, want it to start", err)
	}
	if took := time.Since(started); took > 2*time.Second {
		t.Errorf("Start took %s with no internet, want it not to wait on the bucket", took)
	}
	sell(t, database, 1, 30)
	waitForState(t, engine, backup.StateFailing)
	if got := engine.Status(); got.Error == "" {
		t.Errorf("Status = %+v, want a reason", got)
	}

	if err := database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	budget, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stopping := time.Now()
	if err := engine.Close(budget); err != nil {
		t.Errorf("Close = %v", err)
	}
	if took := time.Since(stopping); took > 6*time.Second {
		t.Errorf("Close took %s with no internet, want it not to spend the shutdown waiting", took)
	}

	// The line comes back, at the same address.
	fake := s3fake.New(testBucket, testKey)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("could not take %s back to stand in for the bucket: %v", addr, err)
	}
	srv := newServerOn(t, listener, fake)
	defer srv.Close()

	database, _, stop := v.open(t)
	sell(t, database, 31, 10)
	stop()
	if rows, _ := restored(t, v.cfg); rows != 40 {
		t.Errorf("restored %d rows once the bucket could be reached, want all 40", rows)
	}
}

// TestTheProbeAsksABucketCheaply: a quiet venue must not run up a bill,
// and what it asks must be a question a wrong key fails.
func TestTheProbeAsksABucketCheaply(t *testing.T) {
	fake, srv := newFakeS3(t, testBucket, testKey)
	v := bucketVenue(t, srv.URL)
	ctx := context.Background()

	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, ProbeEvery: 25 * time.Millisecond})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { database.Close(ctx); engine.Close(ctx) })

	// Litestream writes its first snapshot once the bucket answers, on its
	// own clock: on a fast machine that lands before the probe's third
	// look, on a slow one after. What must hold is that the probe's own
	// looks write nothing: over three more looks, the objects written do
	// not grow. The first baseline that holds still is the proof.
	deadline := time.Now().Add(10 * time.Second)
	waitForLooks := func(n int) {
		for fake.Count("GET bucket") < n {
			if time.Now().After(deadline) {
				t.Fatalf("the probe asked %d times, want it to keep asking", fake.Count("GET bucket"))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitForLooks(3)
	for {
		looks, wrote := fake.Count("GET bucket"), fake.Count("PUT object")
		waitForLooks(looks + 3)
		if fake.Count("PUT object") == wrote {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the probe keeps writing: %d objects after %d looks, want it only to look", fake.Count("PUT object"), fake.Count("GET bucket"))
		}
	}
	if got := engine.Status(); got.State == backup.StateFailing {
		t.Errorf("Status = %+v with a bucket that answers, want it not failing", got)
	}
}

// TestARealBucket runs against a real S3: Amazon's, R2, a MinIO. It is
// skipped unless it is told where, because a venue's kind of machine has
// none:
//
//	MEZ_TEST_S3_BUCKET, MEZ_TEST_S3_ACCESS_KEY_ID, MEZ_TEST_S3_SECRET_ACCESS_KEY,
//	and MEZ_TEST_S3_ENDPOINT or MEZ_TEST_S3_REGION.
//
// It is what the stand-in cannot be: a server that checks the signature,
// so a wrong secret is refused, and that was written by somebody else.
// Each run writes under a path of its own and leaves it there.
func TestARealBucket(t *testing.T) {
	real := backup.S3{
		Bucket:          os.Getenv("MEZ_TEST_S3_BUCKET"),
		Endpoint:        os.Getenv("MEZ_TEST_S3_ENDPOINT"),
		Region:          os.Getenv("MEZ_TEST_S3_REGION"),
		AccessKeyID:     os.Getenv("MEZ_TEST_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MEZ_TEST_S3_SECRET_ACCESS_KEY"),
		Path:            "mez-test/" + time.Now().UTC().Format("20060102-150405.000"),
	}
	if real.Bucket == "" {
		t.Skip("MEZ_TEST_S3_BUCKET is not set: no real bucket to test against")
	}
	ctx := context.Background()

	t.Run("backed up, restarted and restored", func(t *testing.T) {
		v := venue{dataDir: t.TempDir(), cfg: backup.Config{Kind: backup.KindS3, S3: real}}

		database, engine, stop := v.open(t)
		sell(t, database, 1, 200)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		if got := engine.Status(); got.State != backup.StateOK {
			t.Errorf("Status = %+v, want ok", got)
		}
		wantTables := tableCount(t, database)
		sell(t, database, 201, 100)
		stop()

		database, _, stop = v.open(t)
		sell(t, database, 301, 50)
		stop()

		if rows, tables := restored(t, v.cfg); rows != 350 || tables != wantTables {
			t.Errorf("restored %d rows and %d tables, want 350 and %d", rows, tables, wantTables)
		}
	})

	// The SDK sends anything over 5 MB in parts, which the stand-in does
	// not speak, and a venue's database is over 5 MB within the year.
	t.Run("a database bigger than one part", func(t *testing.T) {
		big := real
		big.Path += "-big"
		v := venue{dataDir: t.TempDir(), cfg: backup.Config{Kind: backup.KindS3, S3: big}}

		database, engine, stop := v.open(t)
		if _, err := database.Writer().ExecContext(ctx, "CREATE TABLE backup_ballast (id INTEGER PRIMARY KEY, weight BLOB NOT NULL)"); err != nil {
			t.Fatal(err)
		}
		// Random, so that compressing it saves nothing.
		for i := range 12 {
			weight := make([]byte, 1<<20)
			if _, err := rand.Read(weight); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Writer().ExecContext(ctx, "INSERT INTO backup_ballast (id, weight) VALUES (?, ?)", i, weight); err != nil {
				t.Fatal(err)
			}
		}
		sell(t, database, 1, 10)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		stop()

		out := filepath.Join(t.TempDir(), db.FileName)
		if err := backup.Restore(ctx, v.cfg, out); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		restoredDB, err := db.Open(ctx, db.Options{Path: out})
		if err != nil {
			t.Fatal(err)
		}
		defer restoredDB.Close(ctx)
		var n, bytes int
		if err := restoredDB.Reader().QueryRowContext(ctx, "SELECT count(*), sum(length(weight)) FROM backup_ballast").Scan(&n, &bytes); err != nil {
			t.Fatal(err)
		}
		if n != 12 || bytes != 12<<20 {
			t.Errorf("restored %d rows of %d bytes, want 12 of %d", n, bytes, 12<<20)
		}
	})

	t.Run("photos go up and come back", func(t *testing.T) {
		withPhotos := real
		withPhotos.Path += "-photos"
		v := venue{dataDir: t.TempDir(), cfg: backup.Config{Kind: backup.KindS3, S3: withPhotos}}
		names := []string{addPhoto(t, v.photosDir()), addPhoto(t, v.photosDir()), addPhoto(t, v.photosDir())}

		database, engine := startWithPhotos(t, v, time.Hour)
		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow: %v", err)
		}
		if err := engine.BackUpNow(ctx); err != nil {
			t.Fatalf("BackUpNow a second time, with nothing new: %v", err)
		}
		to := filepath.Join(t.TempDir(), "photos")
		n, err := backup.RestorePhotos(ctx, v.cfg, to)
		if err != nil || n != 3 {
			t.Fatalf("RestorePhotos = %d, %v, want 3", n, err)
		}
		if got := photosIn(t, to); !sameNames(got, names) {
			t.Errorf("restored %v, want %v", got, names)
		}
		if rows, _ := restored(t, v.cfg); rows != 5 {
			t.Errorf("restored %d rows beside the photos, want 5", rows)
		}
	})

	t.Run("a wrong secret is refused, and said", func(t *testing.T) {
		wrong := real
		wrong.Path += "-wrong"
		wrong.SecretAccessKey = "this-is-not-the-secret"
		v := venue{dataDir: t.TempDir(), cfg: backup.Config{Kind: backup.KindS3, S3: wrong}}

		database, engine, _ := v.open(t)
		sell(t, database, 1, 5)
		if err := engine.BackUpNow(ctx); err == nil {
			t.Fatal("BackUpNow succeeded with the wrong secret")
		}
		got := engine.Status()
		if got.State != backup.StateFailing || got.Error == "" {
			t.Errorf("Status = %+v, want failing with a reason", got)
		}
		t.Logf("what a wrong secret reads as: %s", got.Error)
	})
}
