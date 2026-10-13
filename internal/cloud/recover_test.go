package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
)

const (
	testPhone   = "+919876543210"
	testOTP     = "482913"
	testAttempt = "att_5f2c9d1e7a"
	testSession = "ses_not-a-real-session-token"
	testReadKey = "not-a-real-read-only-secret"
)

func TestCleanPhone(t *testing.T) {
	for typed, want := range map[string]string{
		"+919876543210":      "+919876543210",
		"+91 98765 43210":    "+919876543210",
		" +91-98765-43210 ":  "+919876543210",
		"+1 (415) 555.0134":  "+14155550134",
		"+4915112345678":     "+4915112345678",
		"+123456789012345":   "+123456789012345",
		"+65\t8123\t4567":    "+6581234567",
		"+91 (98765) 43210 ": "+919876543210",
		"+919876543210\r\n":  "+919876543210",
		// As a messaging app hands one over: wrapped in marks that say
		// which way it reads, with a space that does not break.
		"\u202a+91\u00a098765 43210\u202c": "+919876543210",
	} {
		got, err := CleanPhone(typed)
		if err != nil || got != want {
			t.Errorf("CleanPhone(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	for _, typed := range []string{
		"", "+", "9876543210", "919876543210", "0091 98765 43210", "+0919876543210", "+91 98765", "+1234567890123456",
		"+91 98765 4321O", "+91 98765 43210 ext 2", "++919876543210", "+91+9876543210", "+91 ९८७६५ ४३२१०",
	} {
		if got, err := CleanPhone(typed); !errors.Is(err, ErrPhoneShape) {
			t.Errorf("CleanPhone(%q) = %q, %v; want ErrPhoneShape", typed, got, err)
		}
	}
}

func TestCleanOTP(t *testing.T) {
	for typed, want := range map[string]string{"482913": "482913", "482 913": "482913", " 482-913 ": "482913", "000000": "000000", "482913\r\n": "482913", "\u202a482\u00a0913\u202c": "482913"} {
		got, err := CleanOTP(typed)
		if err != nil || got != want {
			t.Errorf("CleanOTP(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	for _, typed := range []string{"", "48291", "4829134", "48291O", "48291a", "४८२९१३", "482.913"} {
		if got, err := CleanOTP(typed); !errors.Is(err, ErrOTPShape) {
			t.Errorf("CleanOTP(%q) = %q, %v; want ErrOTPShape", typed, got, err)
		}
	}
}

// recoveryServer is a stand-in for mez Cloud's three recovery routes. It
// knows one account, by its phone, and sends its one-time code nowhere: the
// test knows it.
type recoveryServer struct {
	verify VerifyAnswer
	grant  GrantAnswer

	mu     sync.Mutex
	asked  []asked
	status map[string]int // by path, answered instead when set
	raw    map[string]string
}

// asked is one request as the stand-in saw it.
type asked struct {
	Path   string
	Bearer string
	Body   map[string]any
}

func (s *recoveryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	if r.Method != http.MethodPost || json.Unmarshal(raw, &body) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, asked{Path: r.URL.Path, Bearer: r.Header.Get("Authorization"), Body: body})
	w.Header().Set("Content-Type", "application/json")
	if status := s.status[r.URL.Path]; status != 0 {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, s.raw[r.URL.Path])
		return
	}
	switch r.URL.Path {
	case RecoverStartPath:
		// Whoever's number it is, the answer is the same.
		_ = json.NewEncoder(w).Encode(map[string]string{"attempt": testAttempt})
	case RecoverVerifyPath:
		if body["attempt"] != testAttempt || body["code"] != testOTP {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":{"code":"otp_invalid","message":"two tries left"}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(s.verify)
	case RecoverGrantPath:
		if r.Header.Get("Authorization") != "Bearer "+testSession {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(s.grant)
	default:
		http.NotFound(w, r)
	}
}

func (s *recoveryServer) answer(path string, status int, raw string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[path], s.raw[path] = status, raw
}

func (s *recoveryServer) askedAt(path string) []asked {
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []asked
	for _, a := range s.asked {
		if a.Path == path {
			found = append(found, a)
		}
	}
	return found
}

func newRecoveryServer(t *testing.T) (*recoveryServer, *Client) {
	t.Helper()
	s := &recoveryServer{
		verify: VerifyAnswer{
			Session:   testSession,
			ExpiresAt: time.Date(2026, 10, 12, 21, 45, 0, 0, time.UTC),
			Owner:     Owner{Name: "Priya Menon"},
			Venues: []AccountVenue{
				{ID: "ven_A1B2C3D4", Name: "Corner Cafe", LastBackupAt: time.Date(2026, 10, 12, 16, 2, 30, 0, time.UTC)},
				{ID: "ven_E5F6G7H8", Name: "Corner Cafe, Indiranagar"},
			},
		},
		grant: GrantAnswer{
			Backup: backup.S3{
				Bucket: "mez-venues", Path: "v/corner-cafe", Endpoint: "https://acct.r2.cloudflarestorage.com",
				AccessKeyID: "AKIDREADONLY", SecretAccessKey: testReadKey,
			},
			PairingCode: "ABCD2345",
			ExpiresAt:   time.Date(2026, 10, 12, 22, 30, 0, 0, time.UTC),
		},
		status: map[string]int{}, raw: map[string]string{},
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return s, c
}

// signedIn is a recovery as far as the list of venues.
func signedIn(t *testing.T, c *Client) *Recovery {
	t.Helper()
	attempt, err := c.RecoverStart(context.Background(), testPhone)
	if err != nil {
		t.Fatalf("RecoverStart: %v", err)
	}
	r, err := c.RecoverVerify(context.Background(), attempt, testOTP)
	if err != nil {
		t.Fatalf("RecoverVerify: %v", err)
	}
	return r
}

// TestRecover: a phone number and the code sent to it buy the account's
// venues, and one of them a key to its backup and a code to pair it with.
func TestRecover(t *testing.T) {
	s, c := newRecoveryServer(t)
	ctx := context.Background()

	attempt, err := c.RecoverStart(ctx, " +91 98765-43210 ")
	if err != nil || attempt != testAttempt {
		t.Fatalf("RecoverStart = %q, %v", attempt, err)
	}
	if got := s.askedAt(RecoverStartPath); len(got) != 1 || got[0].Body["phone"] != testPhone || got[0].Bearer != "" {
		t.Errorf("the cloud was asked %+v, want the number as E.164 and nothing else", got)
	}

	r, err := c.RecoverVerify(ctx, attempt, "482 913")
	if err != nil {
		t.Fatalf("RecoverVerify: %v", err)
	}
	if r.Owner.Name != "Priya Menon" || len(r.Venues) != 2 || !r.ExpiresAt.Equal(s.verify.ExpiresAt) {
		t.Errorf("signed in as %+v", r)
	}
	if v := r.Venues[0]; v.ID != "ven_A1B2C3D4" || v.Name != "Corner Cafe" || !v.LastBackupAt.Equal(s.verify.Venues[0].LastBackupAt) {
		t.Errorf("the first venue is %+v", v)
	}
	if !r.Venues[1].LastBackupAt.IsZero() {
		t.Errorf("a venue never backed up says %v", r.Venues[1].LastBackupAt)
	}
	if got := s.askedAt(RecoverVerifyPath); len(got) != 1 || got[0].Body["attempt"] != testAttempt || got[0].Body["code"] != testOTP {
		t.Errorf("the cloud was asked %+v", got)
	}

	g, err := r.Grant(ctx, "ven_A1B2C3D4")
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if g.Venue.Name != "Corner Cafe" || g.Backup.SecretAccessKey != testReadKey || g.Backup.Path != "v/corner-cafe" ||
		g.PairingCode != "ABCD2345" || !g.ExpiresAt.Equal(s.grant.ExpiresAt) {
		t.Errorf("granted %+v", g)
	}
	got := s.askedAt(RecoverGrantPath)
	if len(got) != 1 || got[0].Bearer != "Bearer "+testSession {
		t.Fatalf("the cloud was asked %+v, want the session as a bearer", got)
	}
	if venue, _ := got[0].Body["venue"].(map[string]any); venue["id"] != "ven_A1B2C3D4" {
		t.Errorf("the cloud was asked for %+v", got[0].Body)
	}
	// What is handed over is what `mez restore` takes.
	if err := (backup.Config{Kind: backup.KindS3, S3: g.Backup}).Validate(t.TempDir()); err != nil {
		t.Errorf("the bucket granted is not one a restore would take: %v", err)
	}
}

// TestRecoverStartRefusals: a number that could not be one is never sent,
// and each answer that is not a yes.
func TestRecoverStartRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("a number that could not be one is not sent", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		if _, err := c.RecoverStart(ctx, "98765 43210"); !errors.Is(err, ErrPhoneShape) {
			t.Errorf("err = %v, want ErrPhoneShape", err)
		}
		if len(s.asked) != 0 {
			t.Errorf("the cloud was asked %+v", s.asked)
		}
	})
	for _, tt := range []struct {
		name   string
		status int
		raw    string
		want   error
	}{
		{"a number it cannot send to", http.StatusUnprocessableEntity, `{"error":{"code":"phone_unsupported"}}`, ErrPhoneRefused},
		{"asked too often", http.StatusTooManyRequests, ``, ErrBusy},
		{"a cloud that is broken", http.StatusBadGateway, `oops`, ErrUnreachable},
		{"a cloud that says look elsewhere", http.StatusFound, ``, ErrUnreachable},
		{"an answer that is not JSON", http.StatusOK, `<html>welcome</html>`, ErrInvalid},
		{"an answer with no attempt", http.StatusOK, `{}`, ErrInvalid},
		{"an attempt with a line break in it", http.StatusOK, `{"attempt":"att_1\r\nX-Injected: 1"}`, ErrInvalid},
		{"an attempt far too long", http.StatusOK, `{"attempt":"` + strings.Repeat("a", maxToken+1) + `"}`, ErrInvalid},
		{"an answer too large to be one", http.StatusOK, `{"attempt":"a","x":"` + strings.Repeat("a", maxAnswer) + `"}`, ErrInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, c := newRecoveryServer(t)
			s.answer(RecoverStartPath, tt.status, tt.raw)
			if _, err := c.RecoverStart(ctx, testPhone); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestRecoverVerifyRefusals: a wrong code reads as every other refusal of
// one, and an answer is checked whole.
func TestRecoverVerifyRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("a code that could not be one is not sent", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		if _, err := c.RecoverVerify(ctx, testAttempt, "48291"); !errors.Is(err, ErrOTPShape) {
			t.Errorf("err = %v, want ErrOTPShape", err)
		}
		if len(s.asked) != 0 {
			t.Errorf("the cloud was asked %+v", s.asked)
		}
	})
	t.Run("no attempt to give it back with", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		if _, err := c.RecoverVerify(ctx, "", testOTP); !errors.Is(err, ErrSessionOver) {
			t.Errorf("err = %v, want ErrSessionOver", err)
		}
		if len(s.asked) != 0 {
			t.Errorf("the cloud was asked %+v", s.asked)
		}
	})
	t.Run("a wrong code", func(t *testing.T) {
		_, c := newRecoveryServer(t)
		r, err := c.RecoverVerify(ctx, testAttempt, "000000")
		if r != nil || !errors.Is(err, ErrOTPRefused) {
			t.Errorf("RecoverVerify = %v, %v; want ErrOTPRefused", r, err)
		}
		if strings.Contains(err.Error(), "two tries left") {
			t.Errorf("the refusal repeats what the cloud said of it: %v", err)
		}
	})

	good := func() VerifyAnswer {
		s, _ := newRecoveryServer(t)
		return s.verify
	}
	marshal := func(spoil func(*VerifyAnswer)) string {
		a := good()
		spoil(&a)
		raw, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	manyVenues := func(a *VerifyAnswer) {
		a.Venues = nil
		for i := range maxVenues + 1 {
			a.Venues = append(a.Venues, AccountVenue{ID: fmt.Sprintf("ven_A1B2C3%02d", i), Name: "Cafe"})
		}
	}
	for _, tt := range []struct {
		name   string
		status int
		raw    string
		want   error
	}{
		{"an attempt that ran out", http.StatusGone, `{"error":{"code":"attempt_expired"}}`, ErrOTPRefused},
		{"asked too often", http.StatusTooManyRequests, ``, ErrBusy},
		{"a cloud that is broken", http.StatusInternalServerError, `oops`, ErrUnreachable},
		{"an answer that is not JSON", http.StatusOK, `ok`, ErrInvalid},
		{"an answer with nothing in it", http.StatusOK, `{}`, ErrInvalid},
		{"no session", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Session = "" }), ErrInvalid},
		{"a session with a space in it", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Session = "ses 1" }), ErrInvalid},
		{"no time it ends", http.StatusOK, marshal(func(a *VerifyAnswer) { a.ExpiresAt = time.Time{} }), ErrInvalid},
		{"no owner", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Owner.Name = "" }), ErrInvalid},
		{"an owner with a line break", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Owner.Name = "Priya\nadmin" }), ErrInvalid},
		{"an owner whose name reads backwards", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Owner.Name = "Priya\u202enimda" }), ErrInvalid},
		{"a venue with an escape in its name", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[0].Name = "Cafe\x1b[31m" }), ErrInvalid},
		{"a venue with an id that is not one", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[0].ID = "../../etc" }), ErrInvalid},
		{"a venue with a staff member's id", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[0].ID = "stf_A1B2C3D4" }), ErrInvalid},
		{"a venue listed twice", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[1].ID = a.Venues[0].ID }), ErrInvalid},
		{"a venue with no name", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[0].Name = " " }), ErrInvalid},
		{"a venue with a name far too long", http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues[0].Name = strings.Repeat("a", maxVenueName+1) }), ErrInvalid},
		{"more venues than an account has", http.StatusOK, marshal(manyVenues), ErrInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, c := newRecoveryServer(t)
			s.answer(RecoverVerifyPath, tt.status, tt.raw)
			r, err := c.RecoverVerify(ctx, testAttempt, testOTP)
			if r != nil || !errors.Is(err, tt.want) {
				t.Errorf("RecoverVerify = %v, %v; want %v", r, err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), testSession) {
				t.Errorf("the refusal repeats the session: %v", err)
			}
		})
	}

	t.Run("a name written with joiners is a name", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		s.answer(RecoverVerifyPath, http.StatusOK, marshal(func(a *VerifyAnswer) {
			a.Owner.Name, a.Venues[0].Name = "അവൻ‍ നായർ", "ಕಾಫಿ‌ಮನೆ"
		}))
		r, err := c.RecoverVerify(ctx, testAttempt, testOTP)
		if err != nil || r.Venues[0].Name != "ಕಾಫಿ‌ಮನೆ" {
			t.Errorf("RecoverVerify = %v, %v", r, err)
		}
	})
	t.Run("an account with no venues is still an account", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		s.answer(RecoverVerifyPath, http.StatusOK, marshal(func(a *VerifyAnswer) { a.Venues = nil }))
		r, err := c.RecoverVerify(ctx, testAttempt, testOTP)
		if err != nil || r == nil || len(r.Venues) != 0 {
			t.Errorf("RecoverVerify = %v, %v", r, err)
		}
	})
	t.Run("a venue whose last backup is null was never backed up", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		s.answer(RecoverVerifyPath, http.StatusOK, `{"session":"`+testSession+`","expires_at":"2026-10-12T21:45:00Z","owner":{"name":"Priya Menon"},
			"venues":[{"id":"ven_A1B2C3D4","name":"Corner Cafe","last_backup_at":null}]}`)
		r, err := c.RecoverVerify(ctx, testAttempt, testOTP)
		if err != nil || len(r.Venues) != 1 || !r.Venues[0].LastBackupAt.IsZero() {
			t.Errorf("RecoverVerify = %v, %v", r, err)
		}
	})
}

// TestGrantRefusals: what is not sent, each way the cloud says no, and an
// answer checked whole before any of it is handed on.
func TestGrantRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("a venue the account did not list is not asked for", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		r := signedIn(t, c)
		for _, id := range []string{"ven_ZZZZ9999", "", "ven_a1b2c3d4"} {
			if _, err := r.Grant(ctx, id); !errors.Is(err, ErrNotListed) {
				t.Errorf("Grant(%q) = %v, want ErrNotListed", id, err)
			}
		}
		if got := s.askedAt(RecoverGrantPath); len(got) != 0 {
			t.Errorf("the cloud was asked %+v", got)
		}
	})
	t.Run("a venue never backed up is not asked for", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		if _, err := signedIn(t, c).Grant(ctx, "ven_E5F6G7H8"); !errors.Is(err, ErrNeverBackedUp) {
			t.Errorf("err = %v, want ErrNeverBackedUp", err)
		}
		if got := s.askedAt(RecoverGrantPath); len(got) != 0 {
			t.Errorf("the cloud was asked %+v", got)
		}
	})
	t.Run("no recovery at all", func(t *testing.T) {
		var none *Recovery
		if _, err := none.Grant(ctx, "ven_A1B2C3D4"); !errors.Is(err, ErrSessionOver) {
			t.Errorf("a nil recovery: %v, want ErrSessionOver", err)
		}
		if _, err := (&Recovery{Venues: []AccountVenue{{ID: "ven_A1B2C3D4"}}}).Grant(ctx, "ven_A1B2C3D4"); !errors.Is(err, ErrSessionOver) {
			t.Errorf("a recovery nobody signed in to: %v, want ErrSessionOver", err)
		}
	})

	marshal := func(spoil func(*GrantAnswer)) string {
		s, _ := newRecoveryServer(t)
		a := s.grant
		spoil(&a)
		// Written by hand, field by field, as a cloud in another repository
		// would write it: the names are the contract (ADR-332).
		raw, err := json.Marshal(map[string]any{
			"backup": map[string]string{
				"bucket": a.Backup.Bucket, "path": a.Backup.Path, "endpoint": a.Backup.Endpoint, "region": a.Backup.Region,
				"access_key_id": a.Backup.AccessKeyID, "secret_access_key": a.Backup.SecretAccessKey,
			},
			"pairing_code": a.PairingCode, "expires_at": a.ExpiresAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for _, tt := range []struct {
		name   string
		status int
		raw    string
		want   error
	}{
		{"a session that ran out", http.StatusUnauthorized, ``, ErrSessionOver},
		{"a backup no longer kept", http.StatusGone, `{"error":{"code":"backups_expired"}}`, ErrBackupGone},
		{"a venue it will not hand over", http.StatusForbidden, `{"error":{"code":"venue_closed"}}`, ErrGrantRefused},
		{"asked too often", http.StatusTooManyRequests, ``, ErrBusy},
		{"a cloud that is broken", http.StatusInternalServerError, `oops`, ErrUnreachable},
		{"an answer that is not JSON", http.StatusOK, `ok`, ErrInvalid},
		{"an answer with nothing in it", http.StatusOK, `{}`, ErrInvalid},
		{"no bucket", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Bucket = "" }), ErrInvalid},
		{"no bucket secret", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.SecretAccessKey = "" }), ErrInvalid},
		{"a bucket with no folder", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Path = "" }), ErrInvalid},
		{"a bucket in the clear", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Endpoint = "http://storage.example.com" }), ErrInvalid},
		{"a bucket in the clear on the venue's own network", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Endpoint = "http://192.168.1.20:9000" }), ErrInvalid},
		{"a bucket with a path after its address", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Endpoint = "https://storage.example.com/elsewhere" }), ErrInvalid},
		{"a folder with a line break in it", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.Path = "v/corner-cafe\nx" }), ErrInvalid},
		{"a key id with an escape in it", http.StatusOK, marshal(func(a *GrantAnswer) { a.Backup.AccessKeyID = "AKID\x1b[31m" }), ErrInvalid},
		{"no pairing code", http.StatusOK, marshal(func(a *GrantAnswer) { a.PairingCode = "" }), ErrInvalid},
		{"a pairing code that is not one", http.StatusOK, marshal(func(a *GrantAnswer) { a.PairingCode = "ABCD234O" }), ErrInvalid},
		{"a pairing code not written as one is sent", http.StatusOK, marshal(func(a *GrantAnswer) { a.PairingCode = "abcd-2345" }), ErrInvalid},
		{"no time the key stops", http.StatusOK, marshal(func(a *GrantAnswer) { a.ExpiresAt = time.Time{} }), ErrInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, c := newRecoveryServer(t)
			r := signedIn(t, c)
			s.answer(RecoverGrantPath, tt.status, tt.raw)
			g, err := r.Grant(ctx, "ven_A1B2C3D4")
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if g.PairingCode != "" || g.Backup.SecretAccessKey != "" {
				t.Errorf("a refusal handed something over: %+v", g)
			}
			if err != nil && (strings.Contains(err.Error(), testReadKey) || strings.Contains(err.Error(), testSession)) {
				t.Errorf("the refusal repeats a secret: %v", err)
			}
			if err != nil && len(err.Error()) > 200 {
				t.Errorf("the refusal is %d bytes long: it repeats what the cloud sent", len(err.Error()))
			}
		})
	}

	t.Run("the good answer, written by hand, is taken", func(t *testing.T) {
		s, c := newRecoveryServer(t)
		r := signedIn(t, c)
		s.answer(RecoverGrantPath, http.StatusOK, marshal(func(*GrantAnswer) {}))
		if g, err := r.Grant(ctx, "ven_A1B2C3D4"); err != nil || g.Backup.SecretAccessKey != testReadKey {
			t.Errorf("Grant = %+v, %v", g, err)
		}
	})
	t.Run("a redirect is not followed with the session", func(t *testing.T) {
		var elsewhere int
		other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
		defer other.Close()
		_, c := newRecoveryServer(t)
		r := signedIn(t, c)
		redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, other.URL+RecoverGrantPath, http.StatusTemporaryRedirect)
		}))
		defer redirect.Close()
		r.client.url = redirect.URL
		if _, err := r.Grant(ctx, "ven_A1B2C3D4"); !errors.Is(err, ErrUnreachable) {
			t.Errorf("err = %v, want ErrUnreachable", err)
		}
		if elsewhere != 0 {
			t.Error("the session was sent on to where the cloud pointed")
		}
	})
}

// TestARecoveryKeepsItsSecretsOutOfALog: however it or its grant is printed.
func TestARecoveryKeepsItsSecretsOutOfALog(t *testing.T) {
	_, c := newRecoveryServer(t)
	r := signedIn(t, c)
	g, err := r.Grant(context.Background(), "ven_A1B2C3D4")
	if err != nil {
		t.Fatal(err)
	}
	var none *Recovery

	var logged strings.Builder
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	log.Info("recovering", "recovery", r, "grant", g, "none", none)
	log.Info("recovering", "recovery", *r, "grant", &g)
	// As JSON too: a handler that answered with one by mistake.
	var written []byte
	for _, v := range []any{g, &g, r, *r, map[string]any{"grant": g}, struct{ G Grant }{g}} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		written = append(written, raw...)
	}
	printed := string(written) + fmt.Sprintf("%v %+v %#v %s", r, r, r, r) + fmt.Sprintf("%v %+v %#v %s", *r, *r, *r, *r) +
		fmt.Sprintf("%v %+v %#v %s", g, g, g, g) + fmt.Sprintf("%v %+v %#v", &g, none, none) +
		logged.String() + fmt.Sprint(fmt.Errorf("with %v and %v", r, g))
	for _, secret := range []string{testSession, testReadKey, "ABCD2345"} {
		if strings.Contains(printed, secret) {
			t.Errorf("a secret was printed (%s): %s", secret, printed)
		}
	}
	for _, said := range []string{"Priya Menon", "ven_A1B2C3D4"} {
		if !strings.Contains(printed, said) {
			t.Errorf("what was printed does not say %q: %s", said, printed)
		}
	}
}
