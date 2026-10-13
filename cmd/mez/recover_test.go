package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/service/cloudrecover"
	"github.com/byjoellobo/mez/internal/service/firstrun"
)

const (
	recoverPhone   = "+919876543210"
	recoverOTP     = "482913"
	recoverSession = "ses_not-a-real-session-token"
	recoverCode    = "WXYZ2345"
)

// accountCloud stands in for mez Cloud for one account with one venue: the
// three recovery routes (ADR-332) and pairing (ADR-318).
type accountCloud struct {
	venueID string
	pub     string

	mu     sync.Mutex
	paired []string // the venue ids that paired, in order
	codes  []string
}

func (c *accountCloud) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code  string `json:"code"`
		Venue struct {
			ID string `json:"id"`
		} `json:"venue"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/recover/start":
		_, _ = io.WriteString(w, `{"attempt":"att_1"}`)
	case "/v1/recover/verify":
		if body.Code != recoverOTP {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session": recoverSession, "expires_at": "2026-10-12T21:45:00Z", "owner": map[string]string{"name": "Priya Menon"},
			"venues": []map[string]string{{"id": c.venueID, "name": "The Corner Cafe", "last_backup_at": "2026-10-12T16:02:30Z"}},
		})
	case "/v1/recover/grant":
		if r.Header.Get("Authorization") != "Bearer "+recoverSession || body.Venue.ID != c.venueID {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// A bucket that the test's source turns into the folder the venue
		// was backed up to: there is no S3 here to read.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"backup": map[string]string{
				"bucket": "mez-venues", "path": "v/corner-cafe", "endpoint": "https://acct.r2.cloudflarestorage.com",
				"access_key_id": "AKIDREADONLY", "secret_access_key": "not-a-real-read-only-secret",
			},
			"pairing_code": recoverCode, "expires_at": "2026-10-12T22:30:00Z",
		})
	case "/v1/pair":
		c.mu.Lock()
		c.paired, c.codes = append(c.paired, body.Venue.ID), append(c.codes, body.Code)
		c.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"venue": map[string]string{"slug": "corner-cafe", "hostname": "corner-cafe.mezpos.test"},
			// A relay and a bucket that are not there: the venue is paired
			// all the same (ADR-320).
			"relay": map[string]string{"url": "http://127.0.0.1:9", "key": "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl"},
			"backup": map[string]string{
				"bucket": "mez-venues", "path": "v/corner-cafe", "endpoint": "http://127.0.0.1:9",
				"access_key_id": "AKIDWRITE", "secret_access_key": "not-a-real-bucket-secret",
			},
			"cloud": map[string]string{"public_key": c.pub},
			"owner": map[string]string{"name": "Priya Menon"},
		})
	default:
		http.NotFound(w, r)
	}
}

// asThePage posts JSON to the server as the wizard's page does, from this
// machine, and reads the answer. A server that is not answering, which is
// one between two starts, is status 0.
func asThePage(t *testing.T, addr, path string, header map[string]string, body any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("POST %s: status %d, and an answer that is not JSON: %v", path, resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

// TestAVenueIsBroughtBackByItsOwnersPhone is "I had mez before" as the
// wizard's page does it, from the phone number to a paired venue and a
// paired browser, against one server that nobody stops or starts: it stages
// the venue, starts itself again on it, and pairs it with the code that
// waited (ADR-332 to ADR-335). Only the page itself is missing.
func TestAVenueIsBroughtBackByItsOwnersPhone(t *testing.T) {
	old, from, photo := backedUpVenue(t)
	venueID := venueIDIn(t, old)

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	account := &accountCloud{venueID: venueID, pub: base64.StdEncoding.EncodeToString(pub)}
	cloudSrv := httptest.NewServer(account)
	defer cloudSrv.Close()

	// The new computer: mez installed and started, and nothing set up.
	fresh := t.TempDir()
	args := append(serverArgs(t, fresh), "-cloud-url", cloudSrv.URL)
	addr := args[3] // serverArgs: -data-dir DIR -listen-addr ADDR ...
	opts := runOptions{stdout: io.Discard, source: func(cloud.Grant) backup.Config { return from }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runUntilStopped(ctx, args, opts) }()
	waitForLog(t, fresh, "listening", done)

	token, err := firstrun.ReadToken(fresh)
	if err != nil {
		t.Fatalf("the new install's setup token: %v", err)
	}
	wizard := map[string]string{api.HeaderSetupToken: token}

	// Without the token, nothing: not even a message to somebody's phone.
	if status, _ := asThePage(t, addr, "/api/v1/setup/recover/start", nil, map[string]string{"phone": recoverPhone}); status != http.StatusForbidden {
		t.Fatalf("start with no setup token: status %d", status)
	}

	status, started := asThePage(t, addr, "/api/v1/setup/recover/start", wizard, map[string]string{"phone": "+91 98765 43210"})
	if status != http.StatusOK || started["attempt"] != "att_1" {
		t.Fatalf("start: %d %v", status, started)
	}
	status, signedIn := asThePage(t, addr, "/api/v1/setup/recover/verify", wizard, map[string]any{"attempt": started["attempt"], "code": recoverOTP})
	venues, _ := signedIn["venues"].([]any)
	if status != http.StatusOK || signedIn["owner"] != "Priya Menon" || len(venues) != 1 || venues[0].(map[string]any)["id"] != venueID {
		t.Fatalf("verify: %d %v", status, signedIn)
	}
	status, restored := asThePage(t, addr, "/api/v1/setup/recover/restore", wizard, map[string]string{"venue_id": venueID})
	claim, _ := restored["claim"].(string)
	if status != http.StatusAccepted || restored["venue_name"] != "The Corner Cafe" || restored["photos"] != float64(1) || restored["photos_incomplete"] != false || claim == "" {
		t.Fatalf("restore: %d %v\n%s", status, restored, readLog(t, fresh))
	}

	// Nobody starts it again. It does, and the page asks until it answers.
	claimed := map[string]string{api.HeaderRecoverClaim: claim}
	var outcome map[string]any
	for deadline := time.Now().Add(45 * time.Second); ; {
		select {
		case err := <-done:
			t.Fatalf("the server stopped: %v\n%s", err, readLog(t, fresh))
		default:
		}
		status, outcome = asThePage(t, addr, "/api/v1/setup/recover/outcome", claimed, map[string]string{})
		if status == http.StatusOK && outcome["cloud"] != string(cloudrecover.CloudPairing) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the page was never told how it went: last %d %v; the log:\n%s", status, outcome, readLog(t, fresh))
		}
		// At a page's pace: asking is rate limited, and the pairing waits
		// some seconds on a bucket that is not there.
		time.Sleep(750 * time.Millisecond)
	}
	if outcome["cloud"] != string(cloudrecover.CloudPaired) || outcome["hostname"] != "corner-cafe.mezpos.test" || outcome["venue_id"] != venueID || outcome["venue_name"] != "The Corner Cafe" {
		t.Errorf("the outcome is %v; the log:\n%s", outcome, readLog(t, fresh))
	}
	if _, has := outcome["pairing_code"]; has {
		t.Errorf("a device code was made with no device named: %v", outcome)
	}
	account.mu.Lock()
	paired, codes := account.paired, account.codes
	account.mu.Unlock()
	if len(paired) != 1 || paired[0] != venueID || codes[0] != recoverCode {
		t.Errorf("the cloud saw pairings %v with %v; want the restored venue's own id, once, with the code that waited", paired, codes)
	}
	if kept, err := cloud.Load(fresh); err != nil || kept == nil || kept.Venue.Hostname != "corner-cafe.mezpos.test" {
		t.Errorf("the pairing kept on the new computer: %v, %v", kept, err)
	}

	// The start that brought the venue back ended the wizard: its token
	// opens nothing now, and the venue is not brought back twice.
	if status, refused := asThePage(t, addr, "/api/v1/setup/recover/start", wizard, map[string]string{"phone": recoverPhone}); status != http.StatusForbidden {
		t.Errorf("the wizard's token after the venue is back: status %d %v", status, refused)
	}
	// And a stranger at this machine is told nothing of it.
	if status, _ := asThePage(t, addr, "/api/v1/setup/recover/outcome", map[string]string{api.HeaderRecoverClaim: strings.Repeat("0", 64)}, map[string]string{}); status != http.StatusNotFound {
		t.Errorf("an outcome asked for with a claim that is not the one: status %d", status)
	}

	// The browser the owner is at is no device of this venue's. Naming it
	// buys a code, the same one however often, and the code pairs it.
	device := map[string]string{"device_name": "Front counter", "device_kind": "terminal"}
	status, named := asThePage(t, addr, "/api/v1/setup/recover/outcome", claimed, device)
	deviceCode, _ := named["pairing_code"].(string)
	if status != http.StatusOK || deviceCode == "" || named["expires_at"] == nil {
		t.Fatalf("the outcome with a device named: %d %v", status, named)
	}
	if _, again := asThePage(t, addr, "/api/v1/setup/recover/outcome", claimed, device); again["pairing_code"] != deviceCode {
		t.Errorf("asked again, the code is %v and was %s", again["pairing_code"], deviceCode)
	}
	status, pairedDevice := asThePage(t, addr, "/api/v1/devices/pair", nil, map[string]string{"code": deviceCode})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("pairing the browser with the code it was given: %d %v", status, pairedDevice)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the server, stopped: %v", err)
		}
	case <-time.After(shutdownTimeout + 10*time.Second):
		t.Fatal("the server did not stop")
	}

	if name := venueNameIn(t, fresh); name != "The Corner Cafe" {
		t.Errorf("the venue on the new computer: %q", name)
	}
	if !exists(t, filepath.Join(fresh, "photos", photo)) {
		t.Error("the menu's photo was not brought back")
	}
	d, err := db.Open(context.Background(), db.Options{Path: filepath.Join(fresh, db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(context.Background())
	var counters int
	if err := d.Reader().QueryRow("SELECT COUNT(*) FROM devices WHERE name = 'Front counter'").Scan(&counters); err != nil || counters != 1 {
		t.Errorf("devices called Front counter in the restored venue: %d, %v", counters, err)
	}

	log := readLog(t, fresh)
	for _, said := range []string{"starting again, as asked", "a restored venue was put in place", "paired to its mez Cloud account again"} {
		if !strings.Contains(log, said) {
			t.Errorf("the log does not say %q:\n%s", said, log)
		}
	}
	for _, secret := range []string{claim, recoverCode, recoverSession, recoverOTP, token, "not-a-real-read-only-secret", "not-a-real-bucket-secret", deviceCode} {
		if strings.Contains(log, secret) {
			t.Errorf("a secret (%s) is in the log:\n%s", secret, log)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(fresh, "restore.json"))
	if len(raw) != 0 {
		t.Errorf("word of the restore is still on disk: %s", raw)
	}
}

// TestRunUntilStopped: a server asked to stop stops, whatever it was doing
// about starting again.
func TestRunUntilStopped(t *testing.T) {
	fresh := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runUntilStopped(ctx, serverArgs(t, fresh), runOptions{stdout: io.Discard}) }()
	waitForLog(t, fresh, "listening", done)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runUntilStopped: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("the server did not stop")
	}
	if strings.Contains(readLog(t, fresh), "starting again") {
		t.Error("a server that was stopped says it is starting again")
	}

	// A start that did not unwind cleanly is not followed by another.
	closing := errors.New("closing the database: it is busy")
	for _, tt := range []struct {
		name string
		err  error
		want error
	}{
		{"only asked to start again", errStartAgain, nil},
		{"as run's shutdown joins it with nothing", errors.Join(errStartAgain, nil), nil},
		{"with a database that would not close", errors.Join(errStartAgain, closing), closing},
		{"joined twice over", errors.Join(errors.Join(errStartAgain, nil), errors.Join(closing)), closing},
	} {
		got := besidesStartAgain(tt.err)
		if (tt.want == nil) != (got == nil) || (tt.want != nil && !errors.Is(got, tt.want)) || errors.Is(got, errStartAgain) {
			t.Errorf("%s: besidesStartAgain = %v, want %v", tt.name, got, tt.want)
		}
	}

	// Asked to start again and to stop at once: stopping wins.
	process, stop := context.WithCancel(context.Background())
	serving, startAgain := restartable(process)
	defer serving.stop()
	startAgain()
	if serving.ctx.Err() == nil || !serving.again(process) {
		t.Error("asking to start again did not end what was served under, or was not remembered")
	}
	stop()
	if serving.again(process) {
		t.Error("a process asked to stop would start again")
	}
}
