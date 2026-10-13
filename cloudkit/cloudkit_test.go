package cloudkit_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/cloudkit"
	"github.com/byjoellobo/mez/internal/cloud"
)

// The cloud in these tests is written with cloudkit and the standard library
// and nothing else of this module, as mez Cloud is. The venue is the venue's
// own client, internal/cloud. If the two can come to disagree, it is here.

const (
	relaySecret = "a-relay-secret-of-at-least-thirty-two-bytes"
	venueID     = "ven_A1B2C3D4"
	slug        = "corner-cafe"
)

func bucket(path string) cloudkit.Bucket {
	return cloudkit.Bucket{
		Bucket: "mez-venues", Path: path, Endpoint: "https://acct.r2.cloudflarestorage.com",
		AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "not-a-real-bucket-secret",
	}
}

func read(t *testing.T, r *http.Request, into any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		t.Errorf("%s: the venue sent something that is not the shape: %v", r.URL.Path, err)
	}
}

func write(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func venueClient(t *testing.T, h http.Handler) *cloud.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := cloud.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// TestACloudBuiltOnTheDoorPairsAVenue: the answer a cloud composes from these
// types is one the venue takes whole, the key it minted is the venue's on the
// relay, and what it signs afterwards the venue accepts.
func TestACloudBuiltOnTheDoorPairsAVenue(t *testing.T) {
	keys, err := cloudkit.NewKeys(relaySecret, nil)
	if err != nil {
		t.Fatalf("NewKeys: %v", err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var asked cloudkit.PairRequest

	c := venueClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != cloudkit.PairPath {
			http.NotFound(w, r)
			return
		}
		read(t, r, &asked)
		code, err := cloudkit.CleanCode(asked.Code)
		if err != nil || code != "ABCD2345" {
			http.Error(w, `{"error":{"code":"pairing_code_invalid"}}`, http.StatusNotFound)
			return
		}
		key, err := keys.Issue(rand.Reader, slug, true)
		if err != nil {
			t.Errorf("Issue: %v", err)
		}
		write(w, cloudkit.PairAnswer{
			Venue:  cloudkit.Venue{Slug: slug, Hostname: slug + ".mezpos.com"},
			Relay:  cloudkit.Relay{URL: "https://relay.mezpos.com", Key: key},
			Backup: bucket("v/" + slug),
			Cloud:  cloudkit.CloudKey{PublicKey: base64.StdEncoding.EncodeToString(public)},
			Owner:  cloudkit.Owner{Name: "Priya Menon"},
		})
	}))

	if _, err := c.Pair(context.Background(), "ZZZZ9999", cloud.About{VenueID: venueID, Name: "Corner Cafe", Version: "v1"}); err == nil {
		t.Fatal("a code the cloud does not know was exchanged")
	}
	p, err := c.Pair(context.Background(), "abcd-2345", cloud.About{VenueID: venueID, Name: "Corner Cafe", Version: "v1"})
	if err != nil {
		t.Fatalf("the venue does not take the cloud's answer: %v", err)
	}
	if asked.Venue != (cloudkit.PairingVenue{ID: venueID, Name: "Corner Cafe", Version: "v1"}) {
		t.Errorf("the cloud read %+v of the venue", asked.Venue)
	}
	if p.Venue.Hostname != slug+".mezpos.com" || p.Owner.Name != "Priya Menon" || p.Backup.Path != "v/"+slug {
		t.Errorf("paired as %+v", p)
	}

	// The key the venue now holds is its own on the relay, and lets the
	// back office through.
	checked, err := keys.Check(p.Relay.Key)
	if err != nil || checked.VenueID != slug || !checked.Remote {
		t.Errorf("the venue's key checks as %+v, %v", checked, err)
	}

	// The owner comes in from away: the cloud signs, the venue checks with
	// the key its pairing carries.
	now := time.Now()
	a, err := cloudkit.NewAssertion(venueID, now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := cloudkit.SignAssertion(private, a)
	if err != nil {
		t.Fatalf("SignAssertion: %v", err)
	}
	held, err := p.Key()
	if err != nil {
		t.Fatalf("the pairing's key: %v", err)
	}
	got, err := cloud.VerifyAssertion(held, signed, venueID, now)
	if err != nil || got.Subject != cloudkit.SubjectOwner || got.Nonce != a.Nonce {
		t.Errorf("the venue makes %+v, %v of what the cloud signed", got, err)
	}
	if _, err := cloud.VerifyAssertion(held, signed, "ven_SOMEONE1", now); err == nil {
		t.Error("an assertion for one venue was accepted by another")
	}
}

// TestACloudBuiltOnTheDoorHandsAVenueBack: the three requests of a recovery,
// answered from these types, take the venue's client from a phone number to a
// bucket it can read and a code it can pair with.
func TestACloudBuiltOnTheDoorHandsAVenueBack(t *testing.T) {
	const session = "ses_0123456789abcdef"
	until := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	backedUp := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)

	c := venueClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case cloudkit.RecoverStartPath:
			var q cloudkit.StartRequest
			read(t, r, &q)
			if q.Phone != "+919876500000" {
				t.Errorf("the cloud read the phone as %q", q.Phone)
			}
			write(w, cloudkit.StartAnswer{Attempt: "att_0123456789abcdef"})
		case cloudkit.RecoverVerifyPath:
			var q cloudkit.VerifyRequest
			read(t, r, &q)
			if q.Attempt != "att_0123456789abcdef" || len(q.Code) != cloudkit.OTPLen {
				http.Error(w, "{}", http.StatusForbidden)
				return
			}
			write(w, cloudkit.VerifyAnswer{
				Session: session, ExpiresAt: until, Owner: cloudkit.Owner{Name: "Priya Menon"},
				Venues: []cloudkit.AccountVenue{{ID: venueID, Name: "Corner Cafe", LastBackupAt: backedUp}},
			})
		case cloudkit.RecoverGrantPath:
			var q cloudkit.GrantRequest
			read(t, r, &q)
			if r.Header.Get("Authorization") != "Bearer "+session || q.Venue != (cloudkit.GrantVenue{ID: venueID}) {
				http.Error(w, "{}", http.StatusUnauthorized)
				return
			}
			write(w, cloudkit.GrantAnswer{Backup: bucket("v/" + slug), PairingCode: "ABCD2345", ExpiresAt: until})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	attempt, err := c.RecoverStart(ctx, "+91 98765 00000")
	if err != nil {
		t.Fatalf("RecoverStart: %v", err)
	}
	rec, err := c.RecoverVerify(ctx, attempt, "123456")
	if err != nil {
		t.Fatalf("RecoverVerify: %v", err)
	}
	if len(rec.Venues) != 1 || rec.Venues[0].ID != venueID || !rec.Venues[0].LastBackupAt.Equal(backedUp) {
		t.Errorf("the venue lists %+v", rec.Venues)
	}
	grant, err := rec.Grant(ctx, venueID)
	if err != nil {
		t.Fatalf("the venue does not take the cloud's hand-over: %v", err)
	}
	if grant.PairingCode != "ABCD2345" || grant.Backup.Path != "v/"+slug {
		t.Errorf("handed %v", grant)
	}
}

// carriedUntil is a cloud's entitlements: one venue, to a date.
type carriedUntil struct {
	venue string
	until time.Time
}

func (c carriedUntil) Entitled(_ context.Context, venueID string) (cloudkit.Entitlement, error) {
	if venueID != c.venue {
		return cloudkit.Entitlement{}, nil
	}
	return cloudkit.Entitlement{Until: c.until, Remote: true, State: "active", PaidUntil: c.until.Add(-7 * 24 * time.Hour)}, nil
}

// TestARelayIsMadeThroughTheDoor: a relay with keys and entitlements of the
// cloud's own is built and serves, which is all the door owes: what a relay
// does with them is internal/relay's to test.
func TestARelayIsMadeThroughTheDoor(t *testing.T) {
	keys, err := cloudkit.NewKeys(relaySecret, nil)
	if err != nil {
		t.Fatalf("NewKeys: %v", err)
	}
	var who cloudkit.Entitlements = carriedUntil{venue: slug, until: time.Now().Add(24 * time.Hour)}
	srv, err := cloudkit.NewRelay(cloudkit.RelayOptions{Keys: keys, Entitlements: who, VenueDomain: "mezpos.com", Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	defer srv.Close()

	// A venue nobody is connected as is not there, and the relay says so
	// to the guest; anything but a panic or a 5xx of its own making.
	req := httptest.NewRequest(http.MethodGet, "https://"+slug+".mezpos.com/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusInternalServerError || rec.Code == 0 {
		t.Errorf("the relay answered %d for a venue that is not connected", rec.Code)
	}

	if len(relaySecret) < cloudkit.MinRelaySecretLen {
		t.Error("the test's secret is shorter than a relay takes")
	}
	if !cloudkit.ValidVenueID(slug) || cloudkit.ReservedVenueID(slug) || !cloudkit.ReservedVenueID("www") {
		t.Error("the door's word on venue names is not the relay's")
	}
	if !strings.HasPrefix(cloudkit.AssertionScheme, "Mez-") {
		t.Errorf("AssertionScheme = %q", cloudkit.AssertionScheme)
	}
}
