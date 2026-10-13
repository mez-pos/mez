package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/restore"
	"github.com/byjoellobo/mez/internal/service/cloudrecover"
)

const (
	wizardToken  = "0123456789abcdef0123456789abcdef"
	recoverClaim = "c1a1m0000000000000000000000000000000000000000000000000000000c1a1"
	recoverPhone = "+919876543210"
	readOnlyKey  = "not-a-real-read-only-secret"
)

// fixedToken is a setup token that is whatever it was made as.
type fixedToken string

func (f fixedToken) Valid(candidate string) bool { return candidate == string(f) }
func (fixedToken) Spend() error                  { return nil }

// fakeRecovery is the service before the start: it remembers what it was
// asked and answers what it is told to.
type fakeRecovery struct {
	err   error
	asked []string
}

func (f *fakeRecovery) Start(_ context.Context, phone string) (cloud.Attempt, error) {
	f.asked = append(f.asked, "start "+phone)
	return "att_1", f.err
}

func (f *fakeRecovery) Verify(_ context.Context, attempt cloud.Attempt, otp string) (cloudrecover.Account, error) {
	f.asked = append(f.asked, "verify "+string(attempt)+" "+otp)
	if f.err != nil {
		return cloudrecover.Account{}, f.err
	}
	return cloudrecover.Account{
		Owner: "Priya Menon", ExpiresAt: time.Date(2026, 10, 12, 21, 45, 0, 0, time.FixedZone("IST", 19800)),
		Venues: []cloud.AccountVenue{
			{ID: "ven_A1B2C3D4", Name: "Corner Cafe", LastBackupAt: time.Date(2026, 10, 12, 16, 2, 30, 0, time.UTC)},
			{ID: "ven_E5F6G7H8", Name: "Corner Cafe, Indiranagar"},
		},
	}, nil
}

func (f *fakeRecovery) Restore(_ context.Context, venueID string) (cloudrecover.Restored, error) {
	f.asked = append(f.asked, "restore "+venueID)
	if f.err != nil {
		return cloudrecover.Restored{}, f.err
	}
	return cloudrecover.Restored{VenueID: venueID, VenueName: "Corner Cafe", Photos: 12, PhotosIncomplete: true, Claim: recoverClaim}, nil
}

// fakeArrived is the start that adopted a venue, for one claim.
type fakeArrived struct {
	cloud   cloudrecover.CloudState
	codeErr error
	codes   []auth.NewCode
}

func (f *fakeArrived) Outcome(claim string) (cloudrecover.Outcome, error) {
	if claim != recoverClaim {
		return cloudrecover.Outcome{}, cloudrecover.ErrNoClaim
	}
	out := cloudrecover.Outcome{VenueID: "ven_A1B2C3D4", VenueName: "Corner Cafe", Cloud: f.cloud}
	if f.cloud == cloudrecover.CloudPaired {
		out.Hostname = "corner-cafe.mezpos.com"
	}
	return out, nil
}

func (f *fakeArrived) DeviceCode(_ context.Context, claim, name string, kind auth.DeviceKind) (auth.PairingCode, error) {
	if claim != recoverClaim {
		return auth.PairingCode{}, cloudrecover.ErrNoClaim
	}
	if f.codeErr != nil {
		return auth.PairingCode{}, f.codeErr
	}
	f.codes = append(f.codes, auth.NewCode{DeviceName: name, DeviceKind: kind})
	return auth.PairingCode{Code: "7K3M-9QX2", ExpiresAt: time.Date(2026, 10, 12, 17, 10, 0, 0, time.UTC)}, nil
}

// recoveryServer is a new install's API with "I had mez before" on it.
func recoveryServer(t *testing.T, r Recovery, a Arrived) (http.Handler, *bytes.Buffer) {
	t.Helper()
	_, d, _ := setupServer(t, testConfig())
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := New(testConfig(), log, BuildInfo{Version: "1.2.3"},
		WithDevices(devices), WithSetupToken(fixedToken(wizardToken)), WithRecovery(r, a))
	return srv.Handler(), logs
}

// theWizard is the browser the installer opened, holding the setup token.
func theWizard() caller { return atTheTill().with(HeaderSetupToken, wizardToken) }

const (
	startPath   = "/api/v1/setup/recover/start"
	verifyPath  = "/api/v1/setup/recover/verify"
	restorePath = "/api/v1/setup/recover/restore"
	outcomePath = "/api/v1/setup/recover/outcome"
)

// TestAVenueIsBroughtBackOverHTTP: the three requests before the start, as
// the wizard makes them, and what each answers.
func TestAVenueIsBroughtBackOverHTTP(t *testing.T) {
	r := &fakeRecovery{}
	h, logs := recoveryServer(t, r, nil)

	rec := from(h, theWizard(), http.MethodPost, startPath, `{"phone":"+91 98765 43210"}`)
	var started recoverStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &started); rec.Code != http.StatusOK || err != nil || started.Attempt != "att_1" {
		t.Fatalf("start: %d %s", rec.Code, rec.Body)
	}

	rec = from(h, theWizard(), http.MethodPost, verifyPath, `{"attempt":"att_1","code":"482 913"}`)
	var account recoverVerifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &account); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	if account.Owner != "Priya Menon" || len(account.Venues) != 2 || account.ExpiresAt.Location() != time.UTC {
		t.Errorf("signed in as %+v", account)
	}
	if v := account.Venues[0]; v.ID != "ven_A1B2C3D4" || v.LastBackupAt == nil || !v.LastBackupAt.Equal(time.Date(2026, 10, 12, 16, 2, 30, 0, time.UTC)) {
		t.Errorf("the first venue is %+v", v)
	}
	// A venue never backed up says nothing of when, and not the year 1.
	if account.Venues[1].LastBackupAt != nil || strings.Contains(rec.Body.String(), "0001-01-01") {
		t.Errorf("a venue never backed up: %s", rec.Body)
	}

	rec = from(h, theWizard(), http.MethodPost, restorePath, `{"venue_id":"ven_A1B2C3D4"}`)
	var restored recoverRestoreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &restored); rec.Code != http.StatusAccepted || err != nil {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if restored != (recoverRestoreResponse{VenueID: "ven_A1B2C3D4", VenueName: "Corner Cafe", Photos: 12, PhotosIncomplete: true, Claim: recoverClaim}) {
		t.Errorf("restored %+v", restored)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("the answer that holds the claim has Cache-Control %q", got)
	}

	want := []string{"start +91 98765 43210", "verify att_1 482 913", "restore ven_A1B2C3D4"}
	if fmt.Sprint(r.asked) != fmt.Sprint(want) {
		t.Errorf("the service was asked %q, want %q", r.asked, want)
	}
	if strings.Contains(logs.String(), recoverClaim) || strings.Contains(logs.String(), "482") {
		t.Errorf("the claim or the one-time code is in the log:\n%s", logs.String())
	}
}

// TestRecoveryIsTheWizardsAlone: each of the three is behind everything
// first run is behind, and none of them reaches the service otherwise.
func TestRecoveryIsTheWizardsAlone(t *testing.T) {
	bodies := map[string]string{
		startPath: `{"phone":"` + recoverPhone + `"}`, verifyPath: `{"attempt":"att_1","code":"482913"}`, restorePath: `{"venue_id":"ven_A1B2C3D4"}`,
	}
	tests := []struct {
		name   string
		who    caller
		status int
		code   ErrorCode
	}{
		{"a program on this machine with no token", atTheTill(), http.StatusForbidden, CodeSetupLocked},
		{"one that guesses the token", atTheTill().with(HeaderSetupToken, strings.Repeat("0", 32)), http.StatusForbidden, CodeSetupLocked},
		{"one that shows a claim in its place", atTheTill().with(HeaderRecoverClaim, recoverClaim), http.StatusForbidden, CodeSetupLocked},
		{"a tablet on the venue's network", caller{remote: "192.168.1.40:51000", host: "192.168.1.10:7777", header: map[string]string{HeaderSetupToken: wizardToken}}, http.StatusForbidden, CodeSetupNotHere},
		{"a page on another site, posting to localhost", theWizard().with("Origin", "https://evil.example"), http.StatusForbidden, CodeSetupNotHere},
		{"a name pointed at this machine", caller{remote: "127.0.0.1:51000", host: "evil.example:7777", header: map[string]string{HeaderSetupToken: wizardToken}}, http.StatusForbidden, CodeSetupNotHere},
		{"through a proxy on this machine", theWizard().with("X-Forwarded-For", "203.0.113.9"), http.StatusForbidden, CodeSetupNotHere},
		{"a form, which cannot post JSON", theWizard().with("Content-Type", "text/plain"), http.StatusUnsupportedMediaType, CodeInvalidRequest},
	}
	for _, tt := range tests {
		for path, body := range bodies {
			t.Run(tt.name+" "+path, func(t *testing.T) {
				r := &fakeRecovery{}
				h, _ := recoveryServer(t, r, nil)
				rec := from(h, tt.who, http.MethodPost, path, body)
				if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
					t.Errorf("status %d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
				}
				if len(r.asked) != 0 {
					t.Errorf("the service was asked %q", r.asked)
				}
			})
		}
	}

	t.Run("a body that is not what is asked for", func(t *testing.T) {
		for path := range bodies {
			// A server to each: nine refusals are more than one address has.
			r := &fakeRecovery{}
			h, _ := recoveryServer(t, r, nil)
			for _, body := range []string{`{`, `{"phone":1}`, `{"surprise":true}`} {
				if rec := from(h, theWizard(), http.MethodPost, path, body); rec.Code != http.StatusBadRequest || errorOf(t, rec).Code != CodeInvalidRequest {
					t.Errorf("%s %s: status %d %s", path, body, rec.Code, rec.Body)
				}
			}
			if len(r.asked) != 0 {
				t.Errorf("the service was asked %q", r.asked)
			}
		}
	})
	t.Run("only POST", func(t *testing.T) {
		h, _ := recoveryServer(t, &fakeRecovery{}, nil)
		for _, path := range []string{startPath, verifyPath, restorePath, outcomePath} {
			if rec := from(h, theWizard(), http.MethodGet, path, ""); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s: status %d", path, rec.Code)
			}
		}
	})
	t.Run("a server that cannot bring a venue back has no such routes", func(t *testing.T) {
		h, _, _ := setupServer(t, testConfig())
		for _, path := range []string{startPath, outcomePath} {
			if rec := from(h, theWizard(), http.MethodPost, path, `{}`); rec.Code != http.StatusNotFound {
				t.Errorf("POST %s: status %d", path, rec.Code)
			}
		}
	})
	t.Run("through the relay, whatever it holds", func(t *testing.T) {
		r := &fakeRecovery{}
		a := &fakeArrived{cloud: cloudrecover.CloudPaired}
		h, _ := recoveryServer(t, r, a)
		for path, body := range map[string]string{startPath: bodies[startPath], outcomePath: `{"device_name":"Phone"}`} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.RemoteAddr, req.Host = "127.0.0.1:51000", "localhost:7777"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(HeaderSetupToken, wizardToken)
			req.Header.Set(HeaderRecoverClaim, recoverClaim)
			req.Header.Set(HeaderVia, "relay")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK || rec.Code == http.StatusAccepted {
				t.Errorf("POST %s through the relay: status %d %s", path, rec.Code, rec.Body)
			}
		}
		if len(r.asked) != 0 || len(a.codes) != 0 {
			t.Errorf("a relayed request reached the service (%q) or made a code (%v)", r.asked, a.codes)
		}
	})
}

// TestRecoveryRefusalsAreInAnOwnersWords: each way it does not work, with
// a code the page can act on and a sentence that is not a command's.
func TestRecoveryRefusalsAreInAnOwnersWords(t *testing.T) {
	underneath := errors.New("restore: there is a database at C:\\ProgramData\\mez\\mez.db; restore into another with -data-dir, key " + readOnlyKey)
	tests := []struct {
		name   string
		err    error
		status int
		code   ErrorCode
		field  string
		says   string
	}{
		{"a number that could not be one", cloud.ErrPhoneShape, http.StatusUnprocessableEntity, CodeRecoverPhoneInvalid, "phone", "country code"},
		{"a number it cannot send to", cloud.ErrPhoneRefused, http.StatusUnprocessableEntity, CodeRecoverPhoneRefused, "phone", "cannot send"},
		{"a code that could not be one", cloud.ErrOTPShape, http.StatusUnprocessableEntity, CodeRecoverCodeInvalid, "code", "6 digits"},
		{"a code it does not accept", cloud.ErrOTPRefused, http.StatusConflict, CodeRecoverCodeRefused, "code", "ask for another"},
		{"a sign-in that ran out", cloud.ErrSessionOver, http.StatusConflict, CodeRecoverSignedOut, "", "new code"},
		{"a venue that is not theirs", cloud.ErrNotListed, http.StatusNotFound, CodeRecoverVenueNotFound, "", "not one of this account's"},
		{"a venue never backed up", cloud.ErrNeverBackedUp, http.StatusConflict, CodeRecoverNoBackup, "", "new venue"},
		{"a bucket with nothing in it", fmt.Errorf("restore: %w", backup.ErrNothingToRestore), http.StatusConflict, CodeRecoverNoBackup, "", "nothing to bring back"},
		{"a backup no longer kept", cloud.ErrBackupGone, http.StatusGone, CodeRecoverBackupGone, "", "no longer keeps"},
		{"a venue it will not hand over", cloud.ErrGrantRefused, http.StatusConflict, CodeRecoverRefused, "", "support"},
		{"a cloud that is busy", cloud.ErrBusy, http.StatusTooManyRequests, CodeRateLimited, "", "in a minute"},
		{"no internet", fmt.Errorf("%w: dial tcp 203.0.113.9:443", cloud.ErrUnreachable), http.StatusBadGateway, CodeCloudUnreachable, "", "internet"},
		{"an answer this build cannot use", fmt.Errorf("%w: no session", cloud.ErrInvalid), http.StatusBadGateway, CodeCloudUnreachable, "", "Update mez"},
		{"a computer that has its venue", cloudrecover.ErrAlreadySetUp, http.StatusConflict, CodeAlreadySetUp, "", "already has a venue"},
		{"a venue made while it came down", fmt.Errorf("%w: %w", restore.ErrVenueThere, underneath), http.StatusConflict, CodeAlreadySetUp, "", "already has a venue"},
		{"one already under way", cloudrecover.ErrUnderWay, http.StatusConflict, CodeRecoverUnderWay, "", "Wait"},
		{"a backup of another venue", fmt.Errorf("restore: %w", restore.ErrWrongVenue), http.StatusBadGateway, CodeRecoverBackupFailed, "", "Nothing here was changed"},
		{"a bucket that stopped answering", context.DeadlineExceeded, http.StatusBadGateway, CodeRecoverBackupFailed, "", "try again"},
		{"anything else", underneath, http.StatusBadGateway, CodeRecoverBackupFailed, "", "Nothing here was changed"},
	}
	t.Run("a fault of this computer's own, with no backup coming down", func(t *testing.T) {
		for path, body := range map[string]string{startPath: `{"phone":"` + recoverPhone + `"}`, verifyPath: `{"attempt":"att_1","code":"482913"}`} {
			h, _ := recoveryServer(t, &fakeRecovery{err: underneath}, nil)
			rec := from(h, theWizard(), http.MethodPost, path, body)
			if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "backup") || strings.Contains(rec.Body.String(), "-data-dir") {
				t.Errorf("%s: status %d %s; want a plain 500 that speaks of no backup", path, rec.Code, rec.Body)
			}
		}
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, logs := recoveryServer(t, &fakeRecovery{err: tt.err}, nil)
			rec := from(h, theWizard(), http.MethodPost, restorePath, `{"venue_id":"ven_A1B2C3D4"}`)
			got := errorOf(t, rec)
			if rec.Code != tt.status || got.Code != tt.code || got.Field != tt.field || !strings.Contains(got.Message, tt.says) {
				t.Errorf("status %d %+v; want %d %s on %q saying %q", rec.Code, got, tt.status, tt.code, tt.field, tt.says)
			}
			for _, commands := range []string{"-data-dir", "mez.db", "restore:", "203.0.113.9", readOnlyKey, "cloudrecover"} {
				if strings.Contains(rec.Body.String(), commands) {
					t.Errorf("the answer repeats %q from underneath: %s", commands, rec.Body)
				}
			}
			if strings.Contains(logs.String(), readOnlyKey) && tt.err != underneath && !errors.Is(tt.err, restore.ErrVenueThere) {
				t.Errorf("a key is in the log:\n%s", logs.String())
			}
			if tt.code == CodeRateLimited && rec.Header().Get("Retry-After") == "" {
				t.Error("a busy cloud is answered with no Retry-After")
			}
		})
	}
}

// TestTheOutcomeIsToldToWhoeverHoldsTheClaim: after the start, with the
// setup token gone.
func TestTheOutcomeIsToldToWhoeverHoldsTheClaim(t *testing.T) {
	withClaim := atTheTill().with(HeaderRecoverClaim, recoverClaim)

	t.Run("how it went, while it is pairing and once it has", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPairing}
		h, _ := recoveryServer(t, &fakeRecovery{}, a)
		rec := from(h, withClaim, http.MethodPost, outcomePath, `{}`)
		var got recoverOutcomeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("status %d %s", rec.Code, rec.Body)
		}
		if got != (recoverOutcomeResponse{VenueID: "ven_A1B2C3D4", VenueName: "Corner Cafe", Cloud: cloudrecover.CloudPairing}) {
			t.Errorf("while pairing: %+v", got)
		}
		if strings.Contains(rec.Body.String(), "pairing_code") || strings.Contains(rec.Body.String(), "hostname") || len(a.codes) != 0 {
			t.Errorf("an outcome asked for with no device named carries a code, or made one: %s", rec.Body)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Error("the outcome may be kept by a browser")
		}

		a.cloud = cloudrecover.CloudPaired
		rec = from(h, withClaim, http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"terminal"}`)
		got = recoverOutcomeResponse{}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("status %d %s", rec.Code, rec.Body)
		}
		if got.Cloud != cloudrecover.CloudPaired || got.Hostname != "corner-cafe.mezpos.com" || got.PairingCode != "7K3M-9QX2" ||
			got.ExpiresAt == nil || !got.ExpiresAt.Equal(time.Date(2026, 10, 12, 17, 10, 0, 0, time.UTC)) {
			t.Errorf("once paired, with a device named: %+v", got)
		}
		if len(a.codes) != 1 || a.codes[0] != (auth.NewCode{DeviceName: "Front counter", DeviceKind: auth.KindTerminal}) {
			t.Errorf("the code was made for %+v", a.codes)
		}
	})

	t.Run("nobody else is told anything, and all in one voice", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPaired}
		h, logs := recoveryServer(t, &fakeRecovery{}, a)
		adopted := map[string]caller{
			"no claim":                        atTheTill(),
			"a wrong claim":                   atTheTill().with(HeaderRecoverClaim, strings.Repeat("0", 64)),
			"the setup token in its place":    atTheTill().with(HeaderRecoverClaim, wizardToken),
			"the setup token, and no claim":   theWizard(),
			"the claim in the token's header": atTheTill().with(HeaderSetupToken, recoverClaim),
		}
		var voice string
		for name, who := range adopted {
			rec := from(h, who, http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"terminal"}`)
			if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeRecoverNothing {
				t.Errorf("%s: status %d %s", name, rec.Code, rec.Body)
			}
			if voice == "" {
				voice = errorOf(t, rec).Message
			}
			if errorOf(t, rec).Message != voice {
				t.Errorf("%s is answered differently: %s", name, rec.Body)
			}
		}
		// And a server that brought nothing back says the same.
		none, _ := recoveryServer(t, &fakeRecovery{}, nil)
		rec := from(none, withClaim, http.MethodPost, outcomePath, `{}`)
		if rec.Code != http.StatusNotFound || errorOf(t, rec).Message != voice {
			t.Errorf("a start that adopted nothing: status %d %s", rec.Code, rec.Body)
		}
		if len(a.codes) != 0 {
			t.Errorf("a code was made for a stranger: %+v", a.codes)
		}
		if strings.Contains(logs.String(), recoverClaim) {
			t.Errorf("a claim is in the log:\n%s", logs.String())
		}
	})

	t.Run("the claim, from anywhere but this machine", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPaired}
		h, _ := recoveryServer(t, &fakeRecovery{}, a)
		for name, who := range map[string]caller{
			"a tablet on the venue's network": {remote: "192.168.1.40:51000", host: "192.168.1.10:7777", header: map[string]string{HeaderRecoverClaim: recoverClaim}},
			"a page on another site":          withClaim.with("Origin", "https://evil.example"),
		} {
			rec := from(h, who, http.MethodPost, outcomePath, `{"device_name":"Phone","device_kind":"terminal"}`)
			if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeSetupNotHere {
				t.Errorf("%s: status %d %s", name, rec.Code, rec.Body)
			}
		}
		if len(a.codes) != 0 {
			t.Errorf("a code was made for somewhere else: %+v", a.codes)
		}
	})

	t.Run("a form, which cannot post JSON, even with the claim", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPaired}
		h, _ := recoveryServer(t, &fakeRecovery{}, a)
		rec := from(h, withClaim.with("Content-Type", "text/plain"), http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"terminal"}`)
		if rec.Code != http.StatusUnsupportedMediaType || len(a.codes) != 0 {
			t.Errorf("status %d %s, %d codes made", rec.Code, rec.Body, len(a.codes))
		}
	})

	t.Run("a device that cannot be named so", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPaired, codeErr: &auth.InvalidDevice{Field: "kind", Message: `kind "toaster" is not a device that can be paired`}}
		h, _ := recoveryServer(t, &fakeRecovery{}, a)
		rec := from(h, withClaim, http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"toaster"}`)
		if got := errorOf(t, rec); rec.Code != http.StatusBadRequest || got.Code != CodeInvalidRequest || got.Field != "device_kind" {
			t.Errorf("status %d %+v", rec.Code, got)
		}
	})

	t.Run("a code that cannot be made", func(t *testing.T) {
		a := &fakeArrived{cloud: cloudrecover.CloudPaired, codeErr: errors.New("disk full")}
		h, _ := recoveryServer(t, &fakeRecovery{}, a)
		rec := from(h, withClaim, http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"terminal"}`)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "disk full") {
			t.Errorf("status %d %s", rec.Code, rec.Body)
		}
	})
}

// TestRecoveryIsRateLimited: signing in costs a message to somebody's
// phone, and is held to a person's pace.
func TestRecoveryIsRateLimited(t *testing.T) {
	r := &fakeRecovery{}
	h, _ := recoveryServer(t, r, nil)
	for i := range recoverLimit.requests {
		if rec := from(h, theWizard(), http.MethodPost, startPath, `{"phone":"`+recoverPhone+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := from(h, theWizard(), http.MethodPost, verifyPath, `{"attempt":"att_1","code":"482913"}`)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("request %d: status %d, Retry-After %q; the three share one allowance", recoverLimit.requests+1, rec.Code, rec.Header().Get("Retry-After"))
	}
	if len(r.asked) != recoverLimit.requests {
		t.Errorf("the service was asked %d times, want %d", len(r.asked), recoverLimit.requests)
	}
	// A caller with no token is refused whatever is left, and spends none
	// of it: every caller here has this machine's one address, and a limit
	// in front of the door would be the owner's for any program to use up.
	r2 := &fakeRecovery{}
	h2, _ := recoveryServer(t, r2, nil)
	for range 3 * recoverLimit.requests {
		if rec := from(h2, atTheTill(), http.MethodPost, startPath, `{"phone":"`+recoverPhone+`"}`); rec.Code != http.StatusForbidden {
			t.Fatalf("with no token: status %d %s", rec.Code, rec.Body)
		}
	}
	if rec := from(h2, theWizard(), http.MethodPost, startPath, `{"phone":"`+recoverPhone+`"}`); rec.Code != http.StatusOK {
		t.Errorf("the wizard, after a stranger's tries: status %d %s", rec.Code, rec.Body)
	}

	// So with the outcome: who holds no claim is not counted, and who does
	// is held to a page's pace.
	a := &fakeArrived{cloud: cloudrecover.CloudPairing}
	h3, _ := recoveryServer(t, &fakeRecovery{}, a)
	withClaim := atTheTill().with(HeaderRecoverClaim, recoverClaim)
	for range 2 * outcomeLimit.requests {
		if rec := from(h3, atTheTill(), http.MethodPost, outcomePath, `{}`); rec.Code != http.StatusNotFound {
			t.Fatalf("an outcome with no claim: status %d %s", rec.Code, rec.Body)
		}
	}
	for i := range outcomeLimit.requests {
		if rec := from(h3, withClaim, http.MethodPost, outcomePath, `{}`); rec.Code != http.StatusOK {
			t.Fatalf("the page's request %d: status %d %s", i+1, rec.Code, rec.Body)
		}
	}
	if rec := from(h3, withClaim, http.MethodPost, outcomePath, `{"device_name":"Front counter","device_kind":"terminal"}`); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || len(a.codes) != 0 {
		t.Errorf("the page's request %d: status %d, Retry-After %q, %d codes made", outcomeLimit.requests+1, rec.Code, rec.Header().Get("Retry-After"), len(a.codes))
	}

	// What the wizard itself may spend is another allowance.
	if recoverLimit == setupLimit || recoverLimit == cloudPairLimit || recoverLimit == pairLimit || outcomeLimit == recoverLimit {
		t.Error("recovery shares its allowance with another route: limits are told apart by their shape")
	}
}
