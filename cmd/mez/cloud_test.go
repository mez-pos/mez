package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/relay"
)

const pairedBucketSecret = "not-a-real-bucket-secret"

// pairingTo is a pairing as mez Cloud would give it, for a venue whose
// relay is at relayURL and whose key is key.
func pairingTo(t *testing.T, relayURL, key string) cloud.Pairing {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return cloud.Pairing{
		PairedAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
		CloudURL: cloud.DefaultURL,
		Venue:    cloud.Venue{Slug: "corner-cafe", Hostname: "corner-cafe.mezpos.com"},
		Relay:    cloud.Relay{URL: relayURL, Key: key},
		Backup: backup.S3{
			Bucket: "mez-venues", Path: "v/corner-cafe", Endpoint: "https://acct.r2.cloudflarestorage.com",
			AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: pairedBucketSecret,
		},
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		Owner:     cloud.Owner{Name: "Priya Menon"},
	}
}

const writtenKey = "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl"

// TestWithPairing: no pairing changes nothing; a pairing supplies the relay,
// its key, the address and the back office; and a relay named in the
// settings file wins whole, with nothing taken from the pairing (ADR-319).
func TestWithPairing(t *testing.T) {
	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)

	plain := config.Config{ListenAddr: ":7777"}
	if got, use := withPairing(plain, nil); use != notPaired || got.RelayEnabled() || got.RemoteBackoffice || got.PublicURL != "" {
		t.Errorf("no pairing: %+v, use %v", got, use)
	}

	got, use := withPairing(plain, &p)
	if use != pairingInUse || got.RelayURL != "https://relay.mezpos.com" || got.RelayKey != writtenKey ||
		!got.RemoteBackoffice || got.PublicURL != "https://corner-cafe.mezpos.com" || got.ListenAddr != ":7777" {
		t.Errorf("paired: %+v, use %v", got, use)
	}
	if err := remoteNeedsRelay(got); err != nil {
		t.Errorf("a paired venue's configuration is refused: %v", err)
	}

	// An address of the venue's own in front of the cloud's stays.
	own := plain
	own.PublicURL = "https://order.cornercafe.in"
	if got, _ := withPairing(own, &p); got.PublicURL != "https://order.cornercafe.in" || !got.RelayEnabled() {
		t.Errorf("a venue with its own address: %+v", got)
	}

	theirs := config.Config{RelayURL: "wss://relay.example.com", RelayKey: "mzr_cafe.aaaaaaaaaaaa.sig", PublicURL: "https://cafe.example.com"}
	got, use = withPairing(theirs, &p)
	if use != pairingSetAside || got != theirs {
		t.Errorf("a relay in the settings file: %+v, use %v; want the settings untouched and the pairing set aside", got, use)
	}
}

func TestPairedTo(t *testing.T) {
	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)
	got, ok := pairedTo(&p, pairingInUse)()
	if !ok || got.Owner != "Priya Menon" || got.Hostname != "corner-cafe.mezpos.com" || !got.PairedAt.Equal(p.PairedAt) {
		t.Errorf("in use: %+v, %v", got, ok)
	}
	if _, ok := pairedTo(&p, pairingSetAside)(); ok {
		t.Error("a pairing that is set aside is shown as the venue's")
	}
	if _, ok := pairedTo(nil, notPaired)(); ok {
		t.Error("a venue with no pairing is shown as paired")
	}
}

// TestCloudSyncFloor: half a minute for the bucket the pairing gave, and
// nothing for any other destination or for a venue that is not paired.
func TestCloudSyncFloor(t *testing.T) {
	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)
	theirs := backup.Config{Kind: backup.KindS3, S3: p.Backup}
	floor := cloudSyncFloor(&p)
	if got := floor(theirs); got != 30*time.Second {
		t.Errorf("the cloud's bucket: %v, want 30s", got)
	}
	// Told by the bucket and the folder, not by the key: a key rotated at
	// the next pairing is the same bucket.
	rotated := theirs
	rotated.S3.AccessKeyID, rotated.S3.SecretAccessKey = "ANOTHER", "another-secret"
	if got := floor(rotated); got != 30*time.Second {
		t.Errorf("the cloud's bucket with another key: %v, want 30s", got)
	}
	for name, dest := range map[string]backup.Config{
		"no backup":                         {},
		"a folder":                          {Kind: backup.KindFolder, Folder: "/mnt/usb/mez"},
		"another bucket":                    {Kind: backup.KindS3, S3: backup.S3{Bucket: "my-own", Endpoint: p.Backup.Endpoint, Path: p.Backup.Path}},
		"another folder in the same bucket": {Kind: backup.KindS3, S3: backup.S3{Bucket: p.Backup.Bucket, Endpoint: p.Backup.Endpoint, Path: "v/other"}},
	} {
		if got := floor(dest); got != 0 {
			t.Errorf("%s: floor %v, want none", name, got)
		}
	}
	if got := cloudSyncFloor(nil)(theirs); got != 0 {
		t.Errorf("a venue with no pairing: floor %v, want none", got)
	}
}

// TestAPairedVenueStartsAndIsReached: the whole server, started with a
// pairing in its data directory and nothing about a relay in its settings,
// dials the relay the pairing names with the key it holds, and a guest
// reaches it there. Neither secret is in its log.
func TestAPairedVenueStartsAndIsReached(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	keys, err := relay.NewKeys(secret, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := relay.New(relay.Options{Keys: keys, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { srv.Close(); ts.Close() })
	key, err := keys.Issue(rand.Reader, "corner-cafe", true)
	if err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	if err := cloud.Save(dataDir, pairingTo(t, ts.URL, key)); err != nil {
		t.Fatalf("keeping the pairing: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, serverArgs(t, dataDir), runOptions{stdout: io.Discard}) }()
	waitForLog(t, dataDir, "listening", done)

	// A guest of the venue, through the relay. 503 is a venue that is not
	// connected; anything else came from the venue itself.
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := http.Get(ts.URL + "/v/corner-cafe/")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusServiceUnavailable {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the paired venue never connected to its relay; its log:\n%s", readLog(t, dataDir))
		}
		time.Sleep(20 * time.Millisecond)
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
	lines := readLog(t, dataDir)
	if !strings.Contains(lines, "paired to mez Cloud") || !strings.Contains(lines, "corner-cafe.mezpos.com") {
		t.Errorf("the log does not say the venue is paired, and where:\n%s", lines)
	}
	if strings.Contains(lines, key) || strings.Contains(lines, pairedBucketSecret) {
		t.Error("the server's log carries one of the pairing's secrets")
	}
}

// TestAPairingThatIsWrongStopsTheStart: the owner believes the venue is
// connected, so a file that cannot be read is said, with the way out, and
// before the database is opened.
func TestAPairingThatIsWrongStopsTheStart(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, cloud.File), []byte(`{"relay":{"url":"http://relay.example.com","key":"hunter2"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := run(ctx, serverArgs(t, dataDir), runOptions{stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), cloud.File) || !strings.Contains(err.Error(), "delete that file") {
		t.Fatalf("run = %v, want it to refuse to start, name %s and say how to start without it", err, cloud.File)
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Error("the refusal repeats what was written as the key")
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, db.FileName)); statErr == nil {
		t.Error("the database was opened before the pairing was refused")
	}
}
