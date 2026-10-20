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

	"github.com/mez-pos/mez/internal/api"
	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/relay"
	"github.com/mez-pos/mez/internal/tunnel"
	"github.com/mez-pos/mez/web"
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
	got, ok := newVenueLink("", &p, pairingInUse).pairedTo()
	if !ok || got.Owner != "Priya Menon" || got.Hostname != "corner-cafe.mezpos.com" || !got.PairedAt.Equal(p.PairedAt) {
		t.Errorf("in use: %+v, %v", got, ok)
	}
	if _, ok := newVenueLink("", &p, pairingSetAside).pairedTo(); ok {
		t.Error("a pairing that is set aside is shown as the venue's")
	}
	if _, ok := newVenueLink("", nil, notPaired).pairedTo(); ok {
		t.Error("a venue with no pairing is shown as paired")
	}
}

// TestTheVenuesAddress: what the settings file says, over the pairing's
// hostname, over nothing.
func TestTheVenuesAddress(t *testing.T) {
	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)
	if got := newVenueLink("", nil, notPaired).publicURL(); got != "" {
		t.Errorf("not paired, nothing set: %q", got)
	}
	if got := newVenueLink("", &p, pairingInUse).publicURL(); got != "https://corner-cafe.mezpos.com" {
		t.Errorf("paired: %q", got)
	}
	if got := newVenueLink("https://order.cornercafe.in", &p, pairingInUse).publicURL(); got != "https://order.cornercafe.in" {
		t.Errorf("paired, with an address of its own: %q", got)
	}
	if got := newVenueLink("https://cafe.example.com", &p, pairingSetAside).publicURL(); got != "https://cafe.example.com" {
		t.Errorf("a pairing set aside: %q", got)
	}
}

// TestCloudSyncFloor: half a minute for the bucket the pairing gave, and
// nothing for any other destination or for a venue that is not paired.
func TestCloudSyncFloor(t *testing.T) {
	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)
	theirs := backup.Config{Kind: backup.KindS3, S3: p.Backup}
	floor := newVenueLink("", &p, pairingInUse).syncFloor
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
	if got := newVenueLink("", nil, notPaired).syncFloor(theirs); got != 0 {
		t.Errorf("a venue with no pairing: floor %v, want none", got)
	}
	// Set aside for the relay, the bucket is still theirs.
	if got := newVenueLink("", &p, pairingSetAside).syncFloor(theirs); got != 30*time.Second {
		t.Errorf("a pairing set aside: floor %v, want 30s", got)
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

// TestPairingWhileRunningTakesHoldAtOnce: a venue with no relay is paired,
// and from that moment it is reached at the relay the pairing names, says
// who it is paired to, and has an address. Paired again, with a new key, the
// new tunnel takes the old one's place and the venue is still reached. No
// restart in either (ADR-320).
func TestPairingWhileRunningTakesHoldAtOnce(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	keys, err := relay.NewKeys(secret, nil)
	if err != nil {
		t.Fatal(err)
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := relay.New(relay.Options{Keys: keys, Log: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { srv.Close(); ts.Close() })
	issue := func() string {
		t.Helper()
		key, err := keys.Issue(rand.Reader, "corner-cafe", true)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}

	venue := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "the venue") })
	dialled := 0
	dial := func(p cloud.Pairing) (*tunnel.Client, error) {
		dialled++
		return tunnel.New(tunnel.Options{RelayURL: p.Relay.URL, Key: p.Relay.Key, Handler: venue, Mount: web.MountGuest, BackOffice: true, Log: quiet})
	}
	reached := func(what string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			resp, err := http.Get(ts.URL + "/v/corner-cafe/")
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: a guest never reached the venue through the relay", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	link := newVenueLink("", nil, notPaired)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = link.Close(ctx)
	})
	if got := link.status(); got.State != api.TunnelOff {
		t.Fatalf("before pairing: %+v, want off", got)
	}

	ctx := context.Background()
	first := pairingTo(t, ts.URL, issue())
	if restart, err := link.apply(ctx, first, dial); err != nil || restart {
		t.Fatalf("apply: restart %v, %v", restart, err)
	}
	reached("paired")
	if got, ok := link.pairedTo(); !ok || got.Hostname != "corner-cafe.mezpos.com" {
		t.Errorf("paired to %+v, %v", got, ok)
	}
	if got := link.publicURL(); got != "https://corner-cafe.mezpos.com" {
		t.Errorf("address %q", got)
	}
	if got := link.status(); got.State == api.TunnelOff || !got.BackOffice {
		t.Errorf("status %+v, want a tunnel that carries the back office", got)
	}
	if got := link.syncFloor(backup.Config{Kind: backup.KindS3, S3: first.Backup}); got != 30*time.Second {
		t.Errorf("the floor for the bucket just given: %v", got)
	}

	// Paired again: a new key, and the old tunnel is not left to fight it.
	second := pairingTo(t, ts.URL, issue())
	second.Owner.Name = "Kabir Rao"
	if restart, err := link.apply(ctx, second, dial); err != nil || restart {
		t.Fatalf("apply again: restart %v, %v", restart, err)
	}
	reached("paired again")
	if got, _ := link.pairedTo(); got.Owner != "Kabir Rao" {
		t.Errorf("paired to %+v, want the second", got)
	}
	// Still there a moment later: two tunnels on one venue would be taking
	// each other's place.
	time.Sleep(300 * time.Millisecond)
	reached("a moment after pairing again")
	if got := link.status(); got.State != api.TunnelConnected {
		t.Errorf("status %+v, want connected", got)
	}
	if dialled != 2 {
		t.Errorf("%d tunnels were built, want one for each pairing", dialled)
	}

	// A tunnel that cannot be built leaves the pairing to the next start.
	bad := second
	bad.Relay.Key = "hunter2"
	if restart, err := link.apply(ctx, bad, dial); err == nil || !restart {
		t.Errorf("a pairing whose tunnel cannot be built: restart %v, %v", restart, err)
	}
	if got, _ := link.pairedTo(); got.Owner != "Kabir Rao" {
		t.Error("a pairing that could not be put to use replaced the one that is")
	}

	// And nothing is started once mez is stopping.
	closing, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := link.Close(closing); err != nil {
		t.Fatal(err)
	}
	if restart, err := link.apply(ctx, second, dial); err == nil || !restart {
		t.Errorf("after Close: restart %v, %v", restart, err)
	}
}

// TestAPairedVenueIsGivenItsCloudOwnerAtStart: a venue paired by a build
// from before the cloud could sign anyone in has nobody for it to be, and
// is given them once. A start does not bring back one who was turned off.
func TestAPairedVenueIsGivenItsCloudOwnerAtStart(t *testing.T) {
	ctx := context.Background()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir, args := setupArgs(t)
	runSetupIn(t, args)
	d := opened(t, dir)
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	owners := func() (n int, name string, active bool) {
		t.Helper()
		rows, err := d.Reader().QueryContext(ctx, `SELECT name, active FROM staff WHERE cloud = 1`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			n++
			if err := rows.Scan(&name, &active); err != nil {
				t.Fatal(err)
			}
		}
		return n, name, active
	}

	// Not paired: nobody.
	ensureCloudOwner(ctx, d, staff, nil, quiet)
	if n, _, _ := owners(); n != 0 {
		t.Fatalf("a venue that is not paired has %d cloud owners", n)
	}
	defer func() {
		// And a venue that was paired and is no longer: a start on no
		// pairing ends what mez Cloud had opened.
		if _, err := d.Writer().ExecContext(ctx, `UPDATE staff SET active = 1 WHERE cloud = 1`); err != nil {
			t.Fatal(err)
		}
		in, err := staff.SignInCloud(ctx, 1, auth.Vouched{Nonce: "nonce-0123456789abcdef", KeepUntil: time.Now().Add(time.Minute)})
		if err != nil {
			t.Fatalf("SignInCloud: %v", err)
		}
		ensureCloudOwner(ctx, d, staff, nil, quiet)
		if _, err := staff.CloudDevice(ctx, in.Token); err == nil {
			t.Error("a session mez Cloud opened outlived the pairing at a start")
		}
	}()

	p := pairingTo(t, "https://relay.mezpos.com", "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl")
	ensureCloudOwner(ctx, d, staff, &p, quiet)
	if n, name, active := owners(); n != 1 || name != p.Owner.Name || !active {
		t.Fatalf("after a start: %d cloud owners, %q, active %v", n, name, active)
	}

	// Turned off at the venue, and started again under another name.
	if _, err := d.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE cloud = 1`); err != nil {
		t.Fatal(err)
	}
	p.Owner.Name = "Somebody Else"
	ensureCloudOwner(ctx, d, staff, &p, quiet)
	if n, name, active := owners(); n != 1 || name == "Somebody Else" || active {
		t.Errorf("after a second start: %d cloud owners, %q, active %v; want the one there left as found", n, name, active)
	}
}

// TestNoVenueNoCloudOwner: a machine paired by hand before the wizard has
// run has no venue to give anyone to, and starts all the same.
func TestNoVenueNoCloudOwner(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	p := pairingTo(t, "https://relay.mezpos.com", "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl")
	ensureCloudOwner(ctx, d, staff, &p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var n int
	if err := d.Reader().QueryRowContext(ctx, `SELECT COUNT(*) FROM staff`).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d staff (err %v), want none", n, err)
	}
}

// TestOnlyTheTunnelSaysARequestWasRelayed: the mark is taken off whatever
// arrives on the server's own socket, however it is spelled, so that a
// client on the venue's network cannot say it came through the relay.
func TestOnlyTheTunnelSaysARequestWasRelayed(t *testing.T) {
	var saw []string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { saw = r.Header.Values(api.HeaderVia) })
	ts := httptest.NewServer(fromTheNetwork(inner))
	defer ts.Close()
	for _, name := range []string{"X-Mez-Via", "x-mez-via", "X-MEZ-VIA"} {
		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header[name] = []string{"relay", "anything"}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if len(saw) != 0 {
			t.Errorf("sent as %s, the handler saw %v", name, saw)
		}
	}
	// The tunnel calls the handler itself, and its mark stands.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(tunnel.HeaderVia, tunnel.ViaRelay)
	inner.ServeHTTP(httptest.NewRecorder(), req)
	if len(saw) != 1 {
		t.Errorf("through the tunnel the handler saw %v", saw)
	}
}

// ADR-345: the copy of the pairing a sign-in is checked with takes the
// cloud's next key only while it still holds the key that led there.
func TestLearnKey(t *testing.T) {
	newKey := func(t *testing.T) ed25519.PublicKey {
		t.Helper()
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return pub
	}

	p := pairingTo(t, "https://relay.mezpos.com", writtenKey)
	held, err := p.Key()
	if err != nil {
		t.Fatal(err)
	}
	was := p.PublicKey
	next := newKey(t)
	link := newVenueLink("", &p, pairingInUse)

	link.learnKey(newKey(t), next)
	if got, _ := link.cloudKey(); !got.Equal(held) {
		t.Error("a key nobody held was replaced")
	}
	link.learnKey(held, next)
	if got, ok := link.cloudKey(); !ok || !got.Equal(next) {
		t.Error("the new key was not taken")
	}
	if p.PublicKey != was {
		t.Error("the pairing the link was made with was changed in place")
	}
	if to, ok := link.pairedTo(); !ok || to.Hostname != p.Venue.Hostname || to.Owner != p.Owner.Name {
		t.Errorf("more than the key changed: %+v", to)
	}

	// Nothing paired: nothing to learn, and nothing goes wrong.
	newVenueLink("", nil, notPaired).learnKey(held, next)
}
