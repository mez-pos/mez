package cloud

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
)

const (
	testRelayKey = "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl"
	testSecret   = "not-a-real-bucket-secret"
)

func testPublicKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(pub)
}

// goodPairing is a pairing as a cloud would give it.
func goodPairing(t *testing.T) Pairing {
	t.Helper()
	return Pairing{
		PairedAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
		CloudURL: DefaultURL,
		Venue:    Venue{Slug: "corner-cafe", Hostname: "corner-cafe.mezpos.com"},
		Relay:    Relay{URL: "https://relay.mezpos.com", Key: testRelayKey},
		Backup: backup.S3{
			Bucket: "mez-venues", Path: "v/corner-cafe", Endpoint: "https://acct.r2.cloudflarestorage.com",
			AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: testSecret,
		},
		PublicKey: testPublicKey(t),
		Owner:     Owner{Name: "Priya Menon"},
	}
}

func TestCleanCode(t *testing.T) {
	for typed, want := range map[string]string{
		"ABCD2345":      "ABCD2345",
		"abcd2345":      "ABCD2345",
		"abcd-2345":     "ABCD2345",
		" AB CD 23 45 ": "ABCD2345",
		"zzzz9999":      "ZZZZ9999",
	} {
		got, err := CleanCode(typed)
		if err != nil || got != want {
			t.Errorf("CleanCode(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	for _, typed := range []string{
		"", "ABCD234", "ABCD23456", "ABCD234O", "ABCD2340", "ABCD234I", "ABCD2341", "ABCD_345", "ABCD234é", "12345678",
	} {
		if got, err := CleanCode(typed); !errors.Is(err, ErrCodeShape) {
			t.Errorf("CleanCode(%q) = %q, %v; want ErrCodeShape", typed, got, err)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := goodPairing(t).Validate(); err != nil {
		t.Fatalf("a good pairing: %v", err)
	}
	for name, spoil := range map[string]func(*Pairing){
		"no slug":                         func(p *Pairing) { p.Venue.Slug = "" },
		"a slug that is not a label":      func(p *Pairing) { p.Venue.Slug = "Corner_Cafe" },
		"a hostname that is not one":      func(p *Pairing) { p.Venue.Hostname = "corner cafe" },
		"a hostname with no dot":          func(p *Pairing) { p.Venue.Hostname = "localhost" },
		"a hostname with a path":          func(p *Pairing) { p.Venue.Hostname = "corner-cafe.mezpos.com/x" },
		"a relay in the clear":            func(p *Pairing) { p.Relay.URL = "http://relay.mezpos.com" },
		"a relay with a path":             func(p *Pairing) { p.Relay.URL = "https://relay.mezpos.com/agent" },
		"a relay with a key in it":        func(p *Pairing) { p.Relay.URL = "https://user:pw@relay.mezpos.com" },
		"no relay key":                    func(p *Pairing) { p.Relay.Key = "" },
		"a guests-only key":               func(p *Pairing) { p.Relay.Key = "mzr_corner-cafe.0123456789ab.c2lnbmF0dXJl" },
		"another venue's key":             func(p *Pairing) { p.Relay.Key = "mzr_other-cafe.0123456789ab.remote.c2lnbmF0dXJl" },
		"something that is not a key":     func(p *Pairing) { p.Relay.Key = "hunter2" },
		"no bucket":                       func(p *Pairing) { p.Backup.Bucket = "" },
		"no bucket secret":                func(p *Pairing) { p.Backup.SecretAccessKey = "" },
		"a bucket with no folder":         func(p *Pairing) { p.Backup.Path = "" },
		"a bucket in the clear":           func(p *Pairing) { p.Backup.Endpoint = "http://storage.example.com" },
		"no public key":                   func(p *Pairing) { p.PublicKey = "" },
		"a public key of the wrong size":  func(p *Pairing) { p.PublicKey = base64.StdEncoding.EncodeToString([]byte("short")) },
		"a public key that is not base64": func(p *Pairing) { p.PublicKey = "!!!" },
		"no owner":                        func(p *Pairing) { p.Owner.Name = " " },
		"an owner with a line break":      func(p *Pairing) { p.Owner.Name = "Priya\nadmin" },
		"an owner far too long":           func(p *Pairing) { p.Owner.Name = strings.Repeat("a", maxOwnerName+1) },
		"no time":                         func(p *Pairing) { p.PairedAt = time.Time{} },
		"a cloud in the clear":            func(p *Pairing) { p.CloudURL = "http://api.mezpos.com" },
	} {
		p := goodPairing(t)
		spoil(&p)
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ErrInvalid", name, err)
		} else if strings.Contains(err.Error(), testSecret) || strings.Contains(err.Error(), "c2lnbmF0dXJl") {
			t.Errorf("%s: the refusal repeats a secret: %v", name, err)
		}
	}
}

// TestAPairingIsKeptAndReadBack: in the data directory, whole, replaced by
// the next, and forgotten when removed. No file is no pairing and no error.
func TestAPairingIsKeptAndReadBack(t *testing.T) {
	dir := t.TempDir()
	if got, err := Load(dir); got != nil || err != nil {
		t.Fatalf("with no file: %v, %v; want nothing and no error", got, err)
	}

	first := goodPairing(t)
	if err := Save(dir, first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil || got == nil {
		t.Fatalf("Load: %v, %v", got, err)
	}
	if got.Relay != first.Relay || got.Venue != first.Venue || got.Backup != first.Backup ||
		got.PublicKey != first.PublicKey || got.Owner != first.Owner || !got.PairedAt.Equal(first.PairedAt) || got.CloudURL != first.CloudURL {
		t.Errorf("read back %+v, want what was saved", *got)
	}
	if got.PublicURL() != "https://corner-cafe.mezpos.com" {
		t.Errorf("PublicURL = %q", got.PublicURL())
	}
	if key, err := got.Key(); err != nil || len(key) != ed25519.PublicKeySize {
		t.Errorf("Key = %v, %v", key, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, File))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("the file's mode is %v, %v; it holds two secrets and wants 0600", info.Mode().Perm(), err)
		}
	}

	// Paired again: the new key replaces the old.
	second := first
	second.Relay.Key = "mzr_corner-cafe.ba9876543210.remote.bmV3c2lnbmF0dXJl"
	if err := Save(dir, second); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(dir); got == nil || got.Relay.Key != second.Relay.Key {
		t.Errorf("after pairing again, read back %+v", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d files in the data directory after two saves, want only %s", len(entries), File)
	}

	bad := first
	bad.Relay.Key = "hunter2"
	if err := Save(dir, bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("saving a pairing that is not usable: %v", err)
	}
	if got, _ := Load(dir); got == nil || got.Relay.Key != second.Relay.Key {
		t.Error("a save that was refused changed what is kept")
	}

	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(dir); got != nil || err != nil {
		t.Errorf("after Remove: %v, %v", got, err)
	}
	if err := Remove(dir); err != nil {
		t.Errorf("removing nothing: %v", err)
	}
}

// TestAFileThatIsNotAPairingStopsTheRead: the owner believes the venue is
// connected, so a file that cannot be read is said and not passed over.
func TestAFileThatIsNotAPairingStopsTheRead(t *testing.T) {
	good, err := json.Marshal(goodPairing(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"not JSON":             "relay_key = mzr_x",
		"a field nobody knows": strings.Replace(string(good), `"owner"`, `"extra":1,"owner"`, 1),
		"edited into nonsense": strings.Replace(string(good), "corner-cafe.mezpos.com", "not a host", 1),
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, File), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := Load(dir); got != nil || !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Load = %v, %v; want ErrInvalid", name, got, err)
		}
	}
}

// TestAPairingKeepsItsSecretsOutOfALog: however it is printed.
func TestAPairingKeepsItsSecretsOutOfALog(t *testing.T) {
	p := goodPairing(t)
	var logged strings.Builder
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	log.Info("paired", "pairing", p)
	log.Info("paired", "pairing", &p)
	printed := fmt.Sprintf("%v %+v %#v %s", p, p, p, p) + logged.String() + fmt.Sprint(fmt.Errorf("with %v", p))
	for _, secret := range []string{testSecret, testRelayKey, "c2lnbmF0dXJl"} {
		if strings.Contains(printed, secret) {
			t.Errorf("a secret was printed: %s", printed)
		}
	}
	if !strings.Contains(printed, "corner-cafe") {
		t.Errorf("a printed pairing does not say whose it is: %s", printed)
	}
}

// pairingServer is a stand-in for mez Cloud's one route: it knows one code,
// which is good once.
type pairingServer struct {
	t      *testing.T
	answer PairAnswer

	mu     sync.Mutex
	code   string
	asked  []PairRequest
	status int // answered instead, when not zero
	raw    string
}

func (s *pairingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != PairPath {
		http.NotFound(w, r)
		return
	}
	var req PairRequest
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, req)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case s.status != 0:
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.raw)
	case req.Code != s.code:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"pairing_code_invalid","message":"no such code"}}`)
	default:
		s.code = "" // good once
		_ = json.NewEncoder(w).Encode(s.answer)
	}
}

func newPairingServer(t *testing.T) (*pairingServer, *Client) {
	t.Helper()
	p := goodPairing(t)
	s := &pairingServer{t: t, code: "ABCD2345"}
	s.answer.Venue, s.answer.Relay, s.answer.Backup, s.answer.Owner = p.Venue, p.Relay, p.Backup, p.Owner
	s.answer.Cloud.PublicKey = p.PublicKey
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return s, c
}

// TestPair: a code typed as a person types it is exchanged for a pairing,
// once, and what the venue said of itself went with it.
func TestPair(t *testing.T) {
	s, c := newPairingServer(t)
	at := time.Date(2026, 10, 12, 9, 30, 15, 500, time.FixedZone("IST", 19800))
	c.now = func() time.Time { return at }
	about := About{VenueID: "ven_A1B2C3D4", Name: "Corner Cafe", Version: "v0.1.0"}

	p, err := c.Pair(context.Background(), "abcd-2345", about)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if p.Venue.Slug != "corner-cafe" || p.Relay.Key != testRelayKey || p.Backup.SecretAccessKey != testSecret || p.Owner.Name != "Priya Menon" {
		t.Errorf("paired as %+v", p)
	}
	if want := at.UTC().Truncate(time.Second); !p.PairedAt.Equal(want) || p.PairedAt.Location() != time.UTC {
		t.Errorf("PairedAt = %v, want %v in UTC", p.PairedAt, want)
	}
	if !strings.HasPrefix(p.CloudURL, "http://127.0.0.1:") {
		t.Errorf("CloudURL = %q, want the cloud it was paired with", p.CloudURL)
	}
	if err := Save(t.TempDir(), p); err != nil {
		t.Errorf("what Pair returned cannot be kept: %v", err)
	}
	if len(s.asked) != 1 || s.asked[0].Code != "ABCD2345" || s.asked[0].Venue != (PairingVenue{ID: "ven_A1B2C3D4", Name: "Corner Cafe", Version: "v0.1.0"}) {
		t.Errorf("the cloud was asked %+v", s.asked)
	}

	// Good once.
	if _, err := c.Pair(context.Background(), "ABCD2345", about); !errors.Is(err, ErrCodeRefused) {
		t.Errorf("the same code again: %v, want ErrCodeRefused", err)
	}
}

// TestPairRefusals: each way it does not work, and that a code which could
// not be one is never sent.
func TestPairRefusals(t *testing.T) {
	ctx := context.Background()
	about := About{VenueID: "ven_A1B2C3D4", Name: "Corner Cafe"}

	t.Run("a code that could not be one is not sent", func(t *testing.T) {
		s, c := newPairingServer(t)
		if _, err := c.Pair(ctx, "ABCD-234", about); !errors.Is(err, ErrCodeShape) {
			t.Errorf("err = %v, want ErrCodeShape", err)
		}
		if len(s.asked) != 0 {
			t.Errorf("the cloud was asked %+v", s.asked)
		}
	})
	t.Run("a wrong code", func(t *testing.T) {
		_, c := newPairingServer(t)
		_, err := c.Pair(ctx, "ZZZZ9999", about)
		if !errors.Is(err, ErrCodeRefused) {
			t.Errorf("err = %v, want ErrCodeRefused", err)
		}
		if strings.Contains(err.Error(), "no such code") {
			t.Errorf("the refusal repeats which it was: %v", err)
		}
	})
	for _, tt := range []struct {
		name   string
		status int
		raw    string
		want   error
	}{
		{"an expired code", http.StatusGone, `{"error":{"code":"pairing_code_expired"}}`, ErrCodeRefused},
		{"asked too often", http.StatusTooManyRequests, ``, ErrBusy},
		{"a cloud that is broken", http.StatusInternalServerError, `oops`, ErrUnreachable},
		{"a cloud that says look elsewhere", http.StatusFound, ``, ErrUnreachable},
		{"an answer that is not JSON", http.StatusOK, `<html>welcome</html>`, ErrInvalid},
		{"an answer with nothing in it", http.StatusOK, `{}`, ErrInvalid},
		{"an answer too large to be one", http.StatusOK, `{"owner":{"name":"` + strings.Repeat("a", maxAnswer) + `"}}`, ErrInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, c := newPairingServer(t)
			s.status, s.raw = tt.status, tt.raw
			if _, err := c.Pair(ctx, "ABCD2345", about); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("an answer with a key for another venue", func(t *testing.T) {
		s, c := newPairingServer(t)
		s.answer.Relay.Key = "mzr_other-cafe.0123456789ab.remote.c2lnbmF0dXJl"
		if _, err := c.Pair(ctx, "ABCD2345", about); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v, want ErrInvalid", err)
		}
	})
	t.Run("nobody there", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		c, err := NewClient(url)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pair(ctx, "ABCD2345", about); !errors.Is(err, ErrUnreachable) {
			t.Errorf("err = %v, want ErrUnreachable", err)
		}
	})
	t.Run("a redirect is not followed with the code", func(t *testing.T) {
		var elsewhere int
		other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
		defer other.Close()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+PairPath, http.StatusTemporaryRedirect)
		}))
		defer srv.Close()
		c, err := NewClient(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Pair(ctx, "ABCD2345", about); !errors.Is(err, ErrUnreachable) {
			t.Errorf("err = %v, want ErrUnreachable", err)
		}
		if elsewhere != 0 {
			t.Error("the code was sent on to where the cloud pointed")
		}
	})
}

func TestNewClientRefusesAnAddressAKeyShouldNotTravelTo(t *testing.T) {
	for _, bad := range []string{"", "api.mezpos.com", "http://api.mezpos.com", "ftp://api.mezpos.com", "https://api.mezpos.com/v1", "https://user:pw@api.mezpos.com", "https://api.mezpos.com?x=1"} {
		if _, err := NewClient(bad); err == nil {
			t.Errorf("NewClient(%q) was accepted", bad)
		}
		if err := CheckURL(bad); err == nil {
			t.Errorf("CheckURL(%q) was accepted", bad)
		}
	}
	for _, good := range []string{DefaultURL, "https://api.mezpos.com/", "http://localhost:8787", "http://127.0.0.1:8787"} {
		if _, err := NewClient(good); err != nil {
			t.Errorf("NewClient(%q): %v", good, err)
		}
	}
}

// ADR-346: every code drawn is one CleanCode takes as it is, and the
// alphabet is drawn from evenly enough to see each character.
func TestNewCode(t *testing.T) {
	seen := map[rune]int{}
	for i := 0; i < 2000; i++ {
		code, err := NewCode(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if clean, err := CleanCode(code); err != nil || clean != code {
			t.Fatalf("NewCode drew %q, which CleanCode makes %q, %v", code, clean, err)
		}
		for _, r := range code {
			seen[r]++
		}
	}
	if len(seen) != len(codeAlphabet) {
		t.Errorf("%d of the alphabet's %d characters were drawn", len(seen), len(codeAlphabet))
	}
	if _, err := NewCode(strings.NewReader("short")); err == nil {
		t.Error("a code was drawn from too little entropy")
	}
}
