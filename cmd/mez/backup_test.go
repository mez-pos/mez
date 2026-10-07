package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
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
