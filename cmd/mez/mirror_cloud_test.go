package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/db"
)

// pairedVenueBackedUp is a venue paired to mez Cloud before its backup was
// written, as Task 4 leaves one: the cloud's owner and the cloud's device
// are in the database, and so in every copy of it.
func pairedVenueBackedUp(t *testing.T) (string, backup.Config) {
	t.Helper()
	dir, cfg, _ := backedUpVenueAfter(t, func(dataDir string) {
		ctx := context.Background()
		d, err := db.Open(ctx, db.Options{Path: filepath.Join(dataDir, db.FileName)})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = d.Close(ctx) }()
		staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := staff.PairCloudOwner(ctx, 1, "Priya's account"); err != nil {
			t.Fatalf("PairCloudOwner: %v", err)
		}
	})
	return dir, cfg
}

// TestTheCloudSignsInOnAMirror is the owner at mez Cloud, with the venue's
// computer off: an assertion the cloud signs opens the back office over the
// copy, to read, with no PIN and no pairing code.
func TestTheCloudSignsInOnAMirror(t *testing.T) {
	ctx := context.Background()
	dataDir, to := pairedVenueBackedUp(t)
	source := opened(t, dataDir)
	var venue string
	if err := source.Reader().QueryRowContext(ctx, `SELECT public_id FROM venues`).Scan(&venue); err != nil {
		t.Fatal(err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	assertion := func(t *testing.T) string {
		t.Helper()
		a, err := cloud.NewAssertion(venue, time.Now(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := cloud.SignAssertion(key, a)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}

	up, said, _ := startMirror(t, to, t.TempDir(), "-follow", "0", "-cloud-key", base64.StdEncoding.EncodeToString(pub))
	base := "http://" + up.Addr
	if !strings.Contains(said.String(), "mez Cloud can sign the account's owner in here") {
		t.Errorf("the mirror did not say the cloud may sign in:\n%s", said.String())
	}

	// With no assertion, and with one signed by somebody else: one voice.
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cloud.NewAssertion(venue, time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := cloud.SignAssertion(stranger, a)
	if err != nil {
		t.Fatal(err)
	}
	for name, header := range map[string]map[string]string{
		"nothing presented":       nil,
		"signed by somebody else": {"Authorization": "Mez-Cloud " + forged},
	} {
		status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/sessions/cloud", "", header)
		if status != http.StatusUnauthorized || !strings.Contains(string(raw), "cloud_sign_in_refused") {
			t.Errorf("%s: status %d, body %s; want 401 cloud_sign_in_refused", name, status, raw)
		}
	}

	presented := assertion(t)
	status, raw, headers := ask(t, http.MethodPost, base+"/api/v1/sessions/cloud", "", map[string]string{"Authorization": "Mez-Cloud " + presented})
	if status != http.StatusCreated {
		t.Fatalf("signing in from the cloud: status %d, body %s", status, raw)
	}
	if headers.Get("X-Mez-Mirror") == "" {
		t.Error("the answer does not say it is a mirror's")
	}
	var in struct {
		Token string `json:"token"`
		Staff struct {
			Name string `json:"name"`
		} `json:"staff"`
		Permissions []string `json:"permissions"`
		Device      struct {
			Name string `json:"name"`
		} `json:"device"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	if in.Token == "" || in.Staff.Name != "Priya's account" || in.Device.Name != auth.CloudDeviceName || len(in.Permissions) == 0 {
		t.Fatalf("the session = %s", raw)
	}

	// The same assertion a second time opens nothing.
	if status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/sessions/cloud", "", map[string]string{"Authorization": "Mez-Cloud " + presented}); status != http.StatusUnauthorized {
		t.Errorf("the assertion a second time: status %d, body %s", status, raw)
	}

	// The token is presented as both, and reads the back office.
	both := map[string]string{"Authorization": "Bearer " + in.Token, "X-Staff-Token": in.Token}
	for _, path := range []string{"/api/v1/sessions/current", "/api/v1/people", "/api/v1/menu/editor", "/api/v1/devices"} {
		if status, raw, _ := ask(t, http.MethodGet, base+path, "", both); status != http.StatusOK {
			t.Errorf("GET %s on the cloud's session: status %d, body %s", path, status, raw)
		}
	}
	// As the device's alone it is signed in as nobody; as the session's alone it is no device.
	if status, _, _ := ask(t, http.MethodGet, base+"/api/v1/people", "", map[string]string{"Authorization": "Bearer " + in.Token}); status != http.StatusUnauthorized {
		t.Errorf("the token as the device's alone: status %d, want 401", status)
	}
	if status, _, _ := ask(t, http.MethodGet, base+"/api/v1/people", "", map[string]string{"X-Staff-Token": in.Token}); status != http.StatusUnauthorized {
		t.Errorf("the token as the session's alone: status %d, want 401", status)
	}

	// It is a mirror still: nothing is changed, whoever vouched.
	status, raw, _ = ask(t, http.MethodPost, base+"/api/v1/menu/categories", `{"name":"Specials"}`, both)
	if status != http.StatusConflict || !strings.Contains(string(raw), "mirror_read_only") {
		t.Errorf("a change on the cloud's session: status %d, body %s; want 409 mirror_read_only", status, raw)
	}
	// And no PIN signs in on the cloud's device.
	var owner string
	if err := source.Reader().QueryRowContext(ctx, `SELECT public_id FROM staff WHERE name = 'Priya'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	status, raw, _ = ask(t, http.MethodPost, base+"/api/v1/sessions", `{"staff_id":"`+owner+`","pin":"246813"}`, map[string]string{"Authorization": "Bearer " + in.Token})
	if status != http.StatusForbidden {
		t.Errorf("a right PIN on the cloud's device: status %d, body %s; want 403", status, raw)
	}

	// Signed out, the token opens nothing.
	if status, raw, _ := ask(t, http.MethodDelete, base+"/api/v1/sessions/current", "", both); status != http.StatusNoContent {
		t.Errorf("signing out: status %d, body %s", status, raw)
	}
	if status, _, _ := ask(t, http.MethodGet, base+"/api/v1/people", "", both); status != http.StatusUnauthorized {
		t.Errorf("after signing out: status %d, want 401", status)
	}

	// The venue's own database was never asked, and its copy holds no session of the cloud's.
	var sessions, nonces int
	if err := source.Reader().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM staff_sessions), (SELECT COUNT(*) FROM cloud_assertions)`).Scan(&sessions, &nonces); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || nonces != 0 {
		t.Errorf("the venue holds %d sessions and %d spent assertions, want none of either", sessions, nonces)
	}
}

// TestAMirrorWithNoKeyLetsTheCloudInNowhere is the mirror a self-hoster
// starts: the venue was paired, and the mirror was told nothing of the cloud.
func TestAMirrorWithNoKeyLetsTheCloudInNowhere(t *testing.T) {
	ctx := context.Background()
	dataDir, to := pairedVenueBackedUp(t)
	source := opened(t, dataDir)
	var venue string
	if err := source.Reader().QueryRowContext(ctx, `SELECT public_id FROM venues`).Scan(&venue); err != nil {
		t.Fatal(err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cloud.NewAssertion(venue, time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := cloud.SignAssertion(key, a)
	if err != nil {
		t.Fatal(err)
	}
	up, said, _ := startMirror(t, to, t.TempDir(), "-follow", "0")
	if strings.Contains(said.String(), "mez Cloud") {
		t.Errorf("a mirror with no key spoke of mez Cloud:\n%s", said.String())
	}
	status, raw, _ := ask(t, http.MethodPost, "http://"+up.Addr+"/api/v1/sessions/cloud", "", map[string]string{"Authorization": "Mez-Cloud " + signed})
	if status != http.StatusUnauthorized || !strings.Contains(string(raw), "cloud_sign_in_refused") {
		t.Errorf("status %d, body %s; want 401 cloud_sign_in_refused", status, raw)
	}
}
