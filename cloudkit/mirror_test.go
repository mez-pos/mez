package cloudkit_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/cloudkit"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/backup/s3fake"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

const mirroredVenue = "ven_CORNER01"

// pairedVenueInBucket is a venue paired to the cloud, so that the cloud's
// person is in its database, backed up by the venue's own backup to a
// stand-in bucket.
func pairedVenueInBucket(t *testing.T) cloudkit.Bucket {
	t.Helper()
	ctx := context.Background()
	srv := httptest.NewServer(s3fake.New(restoreBucket, restoreKey))
	t.Cleanup(srv.Close)
	b := cloudkit.Bucket{Bucket: restoreBucket, Path: "v/" + slug, Endpoint: srv.URL, AccessKeyID: restoreKey, SecretAccessKey: restoreSecret}

	dataDir := t.TempDir()
	photos := filepath.Join(dataDir, "photos")
	if err := os.MkdirAll(photos, 0o750); err != nil {
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
	// The cloud's person is made in the role that grants everything, as
	// setup makes the owner's.
	everything := auth.PermissionSet{}
	for _, p := range auth.Permissions() {
		everything[p] = struct{}{}
	}
	owner, err := everything.JSON()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	var venueID int64
	if err := database.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: mirroredVenue, Name: "The Corner Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		venueID = v.ID
		_, err = q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.ID, Name: "owner", Permissions: owner, CreatedAt: now})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: database})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staff.PairCloudOwner(ctx, venueID, "Priya's account"); err != nil {
		t.Fatalf("PairCloudOwner: %v", err)
	}

	engine, err := backup.Start(ctx, backup.Options{DBPath: path, Config: backup.Config{Kind: backup.KindS3, S3: b}, PhotosDir: photos})
	if err != nil {
		t.Fatal(err)
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
	return b
}

// askMirror sends one request and answers its status, body and headers.
func askMirror(t *testing.T, method, url, body string, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, raw, res.Header
}

// TestTheCloudOpensAMirrorThroughTheDoor is the owner at the cloud with the
// venue's computer off: a mirror opened on the venue's bucket, the owner let
// in on an assertion the cloud signs, the back office read, nothing changed,
// and nothing left once it is closed.
func TestTheCloudOpensAMirrorThroughTheDoor(t *testing.T) {
	ctx := context.Background()
	b := pairedVenueInBucket(t)
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	copies := t.TempDir()

	m, err := cloudkit.OpenMirror(ctx, cloudkit.MirrorOptions{
		Bucket: b, StateDir: filepath.Join(t.TempDir(), "mirrors"), CopyDir: copies,
		CloudKey: pub, Build: cloudkit.BuildInfo{Version: "test"},
	})
	if err != nil {
		t.Fatalf("OpenMirror: %v", err)
	}
	if got := m.Venue(); got != (cloudkit.MirrorVenue{PublicID: mirroredVenue, Name: "The Corner Cafe"}) {
		t.Errorf("Venue() = %+v", got)
	}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)

	a, err := cloudkit.NewAssertion(mirroredVenue, time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := cloudkit.SignAssertion(key, a)
	if err != nil {
		t.Fatal(err)
	}
	status, raw, headers := askMirror(t, http.MethodPost, srv.URL+"/api/v1/sessions/cloud", "",
		map[string]string{"Authorization": cloudkit.AssertionScheme + " " + signed})
	if status != http.StatusCreated {
		t.Fatalf("the owner let in on the cloud's word: status %d, body %s", status, raw)
	}
	if headers.Get(cloudkit.HeaderMirror) == "" {
		t.Errorf("the answer does not say, in %s, that it is a mirror's", cloudkit.HeaderMirror)
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || in.Token == "" {
		t.Fatalf("the session = %s (%v)", raw, err)
	}
	both := map[string]string{"Authorization": "Bearer " + in.Token, "X-Staff-Token": in.Token}
	if status, raw, _ := askMirror(t, http.MethodGet, srv.URL+"/api/v1/people", "", both); status != http.StatusOK {
		t.Errorf("reading People: status %d, body %s", status, raw)
	}
	if status, raw, _ := askMirror(t, http.MethodPost, srv.URL+"/api/v1/menu/categories", `{"name":"Specials"}`, both); status != http.StatusConflict || !strings.Contains(string(raw), "mirror_read_only") {
		t.Errorf("a change: status %d, body %s; want 409 mirror_read_only", status, raw)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	left, err := filepath.Glob(filepath.Join(copies, cloudkit.MirrorDirPrefix+"*"))
	if err != nil || len(left) != 0 {
		t.Errorf("a closed mirror left %v (%v)", left, err)
	}
}

// TestAMirrorOfAnEmptyBucketIsRefused: a venue paired and never backed up.
func TestAMirrorOfAnEmptyBucketIsRefused(t *testing.T) {
	srv := httptest.NewServer(s3fake.New(restoreBucket, restoreKey))
	t.Cleanup(srv.Close)
	b := cloudkit.Bucket{Bucket: restoreBucket, Path: "v/" + slug, Endpoint: srv.URL, AccessKeyID: restoreKey, SecretAccessKey: restoreSecret}
	copies := t.TempDir()
	m, err := cloudkit.OpenMirror(context.Background(), cloudkit.MirrorOptions{Bucket: b, StateDir: t.TempDir(), CopyDir: copies})
	if err == nil || m != nil {
		t.Fatalf("OpenMirror: %v, %v; want it refused", m, err)
	}
	if !strings.Contains(err.Error(), cloudkit.ErrNothingToRestore.Error()) {
		t.Errorf("err = %v, want it to say %v", err, cloudkit.ErrNothingToRestore)
	}
	if left, _ := os.ReadDir(copies); len(left) != 0 {
		t.Errorf("a refused mirror left %d entries", len(left))
	}
}
