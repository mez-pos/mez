package main

import (
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

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/service/cloudrecover"
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

// TestAVenueIsBroughtBackByItsOwnersPhone is "I had mez before" from the
// sign-in to a paired venue, under one server that nobody stops or starts:
// it stages the venue, starts itself again on it, and pairs it with the
// code that waited (ADR-332 to ADR-334). Only the page is missing.
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
	type start struct {
		service *cloudrecover.Service
		arrival *cloudrecover.Arrival
	}
	starts := make(chan start, 4)
	opts := runOptions{
		stdout: io.Discard,
		source: func(cloud.Grant) backup.Config { return from },
		built:  func(s *cloudrecover.Service, a *cloudrecover.Arrival) { starts <- start{s, a} },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runUntilStopped(ctx, append(serverArgs(t, fresh), "-cloud-url", cloudSrv.URL), opts) }()

	next := func() start {
		t.Helper()
		select {
		case s := <-starts:
			return s
		case err := <-done:
			t.Fatalf("the server stopped: %v\n%s", err, readLog(t, fresh))
		case <-time.After(30 * time.Second):
			t.Fatalf("the server never came up; its log:\n%s", readLog(t, fresh))
		}
		return start{}
	}
	first := next()
	if first.arrival != nil {
		t.Fatal("a new install says a venue was brought back")
	}

	attempt, err := first.service.Start(ctx, recoverPhone)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	accountSeen, err := first.service.Verify(ctx, attempt, recoverOTP)
	if err != nil || len(accountSeen.Venues) != 1 || accountSeen.Venues[0].ID != venueID {
		t.Fatalf("Verify = %+v, %v", accountSeen, err)
	}
	restored, err := first.service.Restore(ctx, venueID)
	if err != nil {
		t.Fatalf("Restore: %v\n%s", err, readLog(t, fresh))
	}
	if restored.VenueName != "The Corner Cafe" || restored.Photos != 1 || restored.PhotosIncomplete || restored.Claim == "" {
		t.Errorf("restored %+v", restored)
	}

	// Nobody starts it again. It does.
	second := next()
	if second.arrival == nil {
		t.Fatalf("the start after a restore adopted nothing; its log:\n%s", readLog(t, fresh))
	}
	if _, err := second.service.Start(ctx, recoverPhone); !errors.Is(err, cloudrecover.ErrAlreadySetUp) {
		t.Errorf("a recovery asked of the venue that was brought back = %v, want ErrAlreadySetUp", err)
	}
	if _, err := second.arrival.Outcome("not-the-claim"); !errors.Is(err, cloudrecover.ErrNoClaim) {
		t.Errorf("a stranger's Outcome = %v, want ErrNoClaim", err)
	}

	var outcome cloudrecover.Outcome
	for deadline := time.Now().Add(30 * time.Second); ; {
		if outcome, err = second.arrival.Outcome(restored.Claim); err != nil {
			t.Fatalf("Outcome: %v", err)
		}
		if outcome.Cloud != cloudrecover.CloudPairing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the venue was never paired; its log:\n%s", readLog(t, fresh))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if outcome.Cloud != cloudrecover.CloudPaired || outcome.Hostname != "corner-cafe.mezpos.test" || outcome.VenueID != venueID || outcome.VenueName != "The Corner Cafe" {
		t.Errorf("the outcome is %+v; the log:\n%s", outcome, readLog(t, fresh))
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

	// The browser the owner is at is no device of this venue's: it is given
	// a code, and the code pairs it.
	code, err := second.arrival.DeviceCode(ctx, restored.Claim, "Front counter", auth.KindTerminal)
	if err != nil || code.Code == "" {
		t.Fatalf("DeviceCode = %+v, %v", code, err)
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
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	if paired, err := devices.Redeem(context.Background(), code.Code); err != nil || paired.Device.Name != "Front counter" {
		t.Errorf("redeeming the code made for the browser: %+v, %v", paired, err)
	}

	log := readLog(t, fresh)
	for _, said := range []string{"starting again, as asked", "a restored venue was put in place", "paired to its mez Cloud account again"} {
		if !strings.Contains(log, said) {
			t.Errorf("the log does not say %q:\n%s", said, log)
		}
	}
	for _, secret := range []string{restored.Claim, recoverCode, recoverSession, "not-a-real-read-only-secret", "not-a-real-bucket-secret", code.Code} {
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
