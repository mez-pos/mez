package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/service/firstrun"
)

// lockedServer is a new install as cmd/mez builds it: first run over HTTP,
// and the setup token it asks for. token is the one on disk.
func lockedServer(t *testing.T) (h http.Handler, d *db.DB, dir, token string, logs *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()
	dir = t.TempDir()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(dir, db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	first, err := firstrun.New(d, devices, nil)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := firstrun.OpenToken(dir, true)
	if err != nil {
		t.Fatalf("OpenToken: %v", err)
	}
	if token, err = firstrun.ReadToken(dir); err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	logs = &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := New(testConfig(), log, BuildInfo{Version: "1.2.3"}, WithDevices(devices), WithFirstRun(first), WithSetupToken(tok))
	return srv.Handler(), d, dir, token, logs
}

// TestSetupNeedsTheToken: on the machine is not enough. Every other user of
// it, and every program on it, is on the machine too.
func TestSetupNeedsTheToken(t *testing.T) {
	const wrong = "00000000000000000000000000000000"
	tests := []struct {
		name string
		who  func(token string) caller
		ok   bool
	}{
		{"the browser the installer opened", func(token string) caller { return atTheTill().with(HeaderSetupToken, token) }, true},
		{"a program on this machine with no token", func(string) caller { return atTheTill() }, false},
		{"one that guesses", func(string) caller { return atTheTill().with(HeaderSetupToken, wrong) }, false},
		{"one that sends half of it", func(token string) caller { return atTheTill().with(HeaderSetupToken, token[:16]) }, false},
		{"one that sends it in capitals", func(token string) caller { return atTheTill().with(HeaderSetupToken, strings.ToUpper(token)+"X") }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, d, _, token, logs := lockedServer(t)
			who := tt.who(token)

			if st := statusOf(t, from(h, who, http.MethodGet, "/api/v1/setup", "")); !st.Needed || !st.Here || st.Unlocked != tt.ok {
				t.Errorf("status = %+v, want needed, here and unlocked=%v", st, tt.ok)
			}
			rec := from(h, who, http.MethodPost, "/api/v1/setup", setupBody)
			if tt.ok {
				if rec.Code != http.StatusCreated || venuesIn(t, d) != 1 {
					t.Fatalf("status %d with %d venues: %s", rec.Code, venuesIn(t, d), rec.Body)
				}
			} else {
				if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeSetupLocked || venuesIn(t, d) != 0 {
					t.Errorf("status %d with %d venues, want 403 %s and none: %s", rec.Code, venuesIn(t, d), CodeSetupLocked, rec.Body)
				}
			}
			// Said to nobody: not in an answer, not in the log.
			if strings.Contains(rec.Body.String(), token) || strings.Contains(logs.String(), token) {
				t.Errorf("the token is in the response or the log")
			}
		})
	}
}

// TestSetupTokenIsNotAnOracleOffTheMachine: a caller on the LAN who somehow
// has the token is told nothing by it, and can do nothing with it.
func TestSetupTokenIsNotAnOracleOffTheMachine(t *testing.T) {
	h, d, _, token, _ := lockedServer(t)
	tablet := caller{remote: "192.168.1.23:40112", host: "192.168.1.10:7777"}.with(HeaderSetupToken, token)
	if st := statusOf(t, from(h, tablet, http.MethodGet, "/api/v1/setup", "")); st.Here || st.Unlocked {
		t.Errorf("status for a tablet holding the token = %+v, want neither here nor unlocked", st)
	}
	rec := from(h, tablet, http.MethodPost, "/api/v1/setup", setupBody)
	if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeSetupNotHere || venuesIn(t, d) != 0 {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

// TestSetupSpendsTheToken: once the venue exists the token is gone from the
// disk and opens nothing, so it cannot be found later and tried.
func TestSetupSpendsTheToken(t *testing.T) {
	h, _, dir, token, _ := lockedServer(t)
	holder := atTheTill().with(HeaderSetupToken, token)
	if rec := from(h, holder, http.MethodPost, "/api/v1/setup", setupBody); rec.Code != http.StatusCreated {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body)
	}
	if _, err := firstrun.ReadToken(dir); !errors.Is(err, firstrun.ErrNoToken) {
		t.Errorf("after setup the token file reads %v, want ErrNoToken", err)
	}
	if st := statusOf(t, from(h, holder, http.MethodGet, "/api/v1/setup", "")); st.Needed || st.Unlocked {
		t.Errorf("status after setup = %+v, want neither needed nor unlocked", st)
	}
	// With the token as it was, a second venue is refused for being locked:
	// the caller holds nothing now.
	rec := from(h, holder, http.MethodPost, "/api/v1/setup", setupBody)
	if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeSetupLocked {
		t.Errorf("a second setup with the spent token: %d %s", rec.Code, rec.Body)
	}
}

// TestSetupTokenSurvivesARefusedAnswer: a form sent back for a mistake is
// still the owner's to correct and send again.
func TestSetupTokenSurvivesARefusedAnswer(t *testing.T) {
	h, d, dir, token, _ := lockedServer(t)
	holder := atTheTill().with(HeaderSetupToken, token)
	bad := strings.Replace(setupBody, `246813`, `2468`, 1)
	if rec := from(h, holder, http.MethodPost, "/api/v1/setup", bad); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a short PIN: %d %s", rec.Code, rec.Body)
	}
	if got, err := firstrun.ReadToken(dir); err != nil || got != token {
		t.Fatalf("a refused answer changed the token: %q, %v", got, err)
	}
	if rec := from(h, holder, http.MethodPost, "/api/v1/setup", setupBody); rec.Code != http.StatusCreated || venuesIn(t, d) != 1 {
		t.Errorf("the corrected answers: %d %s", rec.Code, rec.Body)
	}
}

// TestCORSAllowsTheSetupTokenHeader: the dev server's page sends it, and a
// header the preflight does not name is one the browser will not send.
func TestCORSAllowsTheSetupTokenHeader(t *testing.T) {
	cfg := testConfig()
	cfg.DevOrigin = "http://localhost:5173"
	h := newTestServer(t, cfg).Handler()
	rec := send(h, http.MethodOptions, "/api/v1/setup", "", map[string]string{
		"Origin": cfg.DevOrigin, "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "x-setup-token",
	})
	if allowed := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(allowed, HeaderSetupToken) {
		t.Errorf("Access-Control-Allow-Headers = %q, want %s in it", allowed, HeaderSetupToken)
	}
}
