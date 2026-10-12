package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"errors"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/restore"
	"net/http"
	"net/http/httptest"
	"os/exec"
)

// TestTheServerBacksUpAndStopsInOrder runs the whole server over a data
// directory that names a backup folder, stops it the way Ctrl-C does, and
// restores what it left at the destination.
//
// It is here for the order of the shutdown, which no package can see on
// its own: the database closes with the backup still holding its log, and
// the backup closes after it with the last of what was written.
func TestTheServerBacksUpAndStopsInOrder(t *testing.T) {
	serverBacksUpTo(t, backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "backups")})
}

// TestTheServerBacksUpToARealBucket is the same through a bucket named in
// the environment (see TestARealBucket in internal/backup), and skipped
// without one.
func TestTheServerBacksUpToARealBucket(t *testing.T) {
	bucket := os.Getenv("MEZ_TEST_S3_BUCKET")
	if bucket == "" {
		t.Skip("MEZ_TEST_S3_BUCKET is not set: no real bucket to test against")
	}
	serverBacksUpTo(t, backup.Config{Kind: backup.KindS3, S3: backup.S3{
		Bucket:          bucket,
		Endpoint:        os.Getenv("MEZ_TEST_S3_ENDPOINT"),
		Region:          os.Getenv("MEZ_TEST_S3_REGION"),
		AccessKeyID:     os.Getenv("MEZ_TEST_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MEZ_TEST_S3_SECRET_ACCESS_KEY"),
		Path:            "mez-test/server-" + time.Now().UTC().Format("20060102-150405.000"),
	}})
}

func serverBacksUpTo(t *testing.T, cfg backup.Config) {
	t.Helper()
	dataDir := t.TempDir()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, backup.ConfigFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// A photo on the menu, named by its hash as the menu names them.
	picture := []byte("not a picture, and nothing here looks inside one")
	sum := sha256.Sum256(picture)
	photo := hex.EncodeToString(sum[:]) + ".jpg"
	if err := os.MkdirAll(filepath.Join(dataDir, "photos"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "photos", photo), picture, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	args := serverArgs(t, dataDir)
	done := make(chan error, 1)
	go func() { done <- run(ctx, args, runOptions{stdout: io.Discard}) }()

	// Stopped as soon as it is serving, which is sooner than Litestream's
	// first look at the database: the copy has to be made on the way down.
	waitForLog(t, dataDir, "listening", done)

	stopping := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("the server did not stop")
	}
	if took := time.Since(stopping); took > 3*time.Second {
		t.Errorf("stopping took %s with a backup running, want it prompt", took)
	}

	// The order, from the server's own log.
	lines := readLog(t, dataDir)
	if secret := cfg.S3.SecretAccessKey; secret != "" && strings.Contains(lines, secret) {
		t.Error("the server's log carries the bucket's secret")
	}
	closedDB := strings.Index(lines, `subsystem=database`)
	closedBackup := strings.Index(lines, `subsystem=backup took`)
	if closedDB < 0 || closedBackup < 0 || closedBackup < closedDB {
		t.Errorf("want the database closed and then the backup; the log says:\n%s", lines)
	}

	out := filepath.Join(t.TempDir(), db.FileName)
	if err := backup.Restore(context.Background(), cfg, out); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	restored, err := db.Open(context.Background(), db.Options{Path: out})
	if err != nil {
		t.Fatalf("opening the restored database: %v", err)
	}
	defer restored.Close(context.Background())

	// Migrating it again finds nothing to do only if the schema came
	// across whole, goose's own table included.
	if err := restored.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating the restored database: %v", err)
	}
	var tables int
	if err := restored.Reader().QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables < 20 {
		t.Errorf("the restored database has %d tables, want the whole schema", tables)
	}

	// And the photo with it (ADR-110).
	photosBack := filepath.Join(t.TempDir(), "photos")
	if n, err := backup.RestorePhotos(context.Background(), cfg, photosBack); err != nil || n != 1 {
		t.Fatalf("RestorePhotos = %d, %v, want the one photo", n, err)
	}
	if back, err := os.ReadFile(filepath.Join(photosBack, photo)); err != nil || string(back) != string(picture) {
		t.Errorf("the photo did not come back as it went: %v", err)
	}
}

// TestABackupFileThatIsWrongStopsTheStart: the owner who wrote it
// believes the venue is being backed up.
func TestABackupFileThatIsWrongStopsTheStart(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, backup.ConfigFile), []byte(`{"kind":"folder","folder":"relative"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	args := serverArgs(t, dataDir)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := run(ctx, args, runOptions{stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), backup.ConfigFile) {
		t.Fatalf("run = %v, want it to refuse to start and name %s", err, backup.ConfigFile)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, db.FileName)); statErr == nil {
		t.Error("the database was opened before the backup file was refused")
	}
}

// serverArgs starts a server on dataDir, on a port that was free a moment
// ago (the config refuses port 0), with a config file of the test's own so that one beside the
// test binary cannot change what it does.
func serverArgs(t *testing.T, dataDir string) []string {
	t.Helper()
	config := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()
	return []string{"-data-dir", dataDir, "-listen-addr", addr, "-config", config, "-log-format", "text"}
}

func readLog(t *testing.T, dataDir string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dataDir, "logs", "*"))
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(raw)
	}
	return all.String()
}

// waitForLog waits for a line holding want, failing if the server stops
// first.
func waitForLog(t *testing.T, dataDir, want string, done <-chan error) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		if strings.Contains(readLog(t, dataDir), want) {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("the server stopped before it said %q: %v", want, err)
		case <-deadline:
			t.Fatalf("the server never said %q; its log:\n%s", want, readLog(t, dataDir))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// lockProbeEnv names the database a child of this test binary is to try
// to take whole. See TestTheServerKeepsItsLocksWithNoInternet.
const lockProbeEnv = "MEZ_TEST_LOCK_PROBE"

// TestHelperLockProbe is not a test. Run as a child process with
// lockProbeEnv set, it tries to take the database whole, as `mez restore`
// does to find out whether mez is running, and exits 3 if it could.
func TestHelperLockProbe(t *testing.T) {
	path := os.Getenv(lockProbeEnv)
	if path == "" {
		t.Skip("a helper for another test")
	}
	// Said, so that the test that runs this knows it ran: a helper that
	// was never reached would otherwise read as a database nobody could
	// take.
	fmt.Println("lock probe: asking for " + path)
	if err := restore.NotRunning(context.Background(), path); err == nil {
		os.Exit(3)
	}
}

// TestTheServerKeepsItsLocksWithNoInternet is the reason Litestream is
// not started until its destination answers (ADR-279).
//
// A venue with a bucket for a backup starts its day unable to reach it.
// Litestream, started regardless, would open the database, fail to reach
// the bucket, close its handle and try again every second; and on every
// system but Windows closing any handle on a file drops every lock the
// process holds on it. The server would be left with no lock on its own
// database, and another process could take it whole.
//
// Locks are a process's, so the asking is done by another process: this
// test binary, run again as TestHelperLockProbe.
func TestTheServerKeepsItsLocksWithNoInternet(t *testing.T) {
	// A bucket that refuses at once. With no internet at all the S3 client
	// retries for minutes before Litestream gives up and closes its
	// handle; a refusal makes it give up within the second, every second,
	// which is the same thing sooner.
	refuses := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>AccessDenied</Message></Error>`)
	}))
	defer refuses.Close()
	nowhere := refuses.URL

	dataDir := t.TempDir()
	raw, err := json.Marshal(backup.Config{Kind: backup.KindS3, S3: backup.S3{
		Bucket: "corner-cafe", Endpoint: nowhere, AccessKeyID: "AKID", SecretAccessKey: "not-a-real-secret",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, backup.ConfigFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, dataDir), runOptions{stdout: io.Discard}) }()
	waitForLog(t, dataDir, "listening", done)

	// Long enough for a Litestream started regardless to have tried, and
	// given up, several times over.
	time.Sleep(4 * time.Second)

	// took runs the helper and reports whether it got the whole database.
	took := func() bool {
		t.Helper()
		child := exec.Command(os.Args[0], "-test.run=^TestHelperLockProbe$", "-test.v")
		child.Env = append(os.Environ(), lockProbeEnv+"="+filepath.Join(dataDir, db.FileName))
		out, err := child.CombinedOutput()
		if !strings.Contains(string(out), "lock probe: asking for") {
			t.Fatalf("the lock probe did not run: %v\n%s", err, out)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 3 {
			return true
		}
		if err != nil {
			t.Fatalf("the lock probe: %v\n%s", err, out)
		}
		return false
	}
	if took() {
		t.Errorf("another process took the whole database while the server was running on it: the server has lost its locks")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("the server did not stop")
	}

	// And with the server stopped it can be taken: the asking is one that
	// can be answered yes, so the no above was the server's doing.
	if !took() {
		t.Error("the lock probe could not take the database even with nothing running on it: it proves nothing")
	}
}
