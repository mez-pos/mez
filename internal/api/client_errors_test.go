package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

// clientErrorServer is a venue with a paired till, its server's log kept to
// read back.
func clientErrorServer(t *testing.T) (h http.Handler, token string, logs *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_AAAAAAAA", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: at, UpdatedAt: at,
		})
		return err
	}); err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}
	devs, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatalf("NewDevices: %v", err)
	}
	logs = &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h = New(testConfig(), log, BuildInfo{Version: "1.2.3"}, WithDevices(devs)).Handler()
	return h, pairOK(t, h, newCode(t, devs)).Token, logs
}

// failedLine is the server's line for a screen that failed to draw, or nil.
func failedLine(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	for _, raw := range bytes.Split(logs.Bytes(), []byte("\n")) {
		var line map[string]any
		if json.Unmarshal(raw, &line) == nil && line["msg"] == "a screen failed to draw" {
			return line
		}
	}
	return nil
}

// TestAScreenThatFailedIsLogged (TECH-8, ADR-402): a paired device tells the
// server, with no one signed in, and it is one line at error level naming
// the venue, the device, the app, the boundary and the screen.
func TestAScreenThatFailedIsLogged(t *testing.T) {
	h, token, logs := clientErrorServer(t)
	body := `{"app":"terminal","where":"rail","path":"/order","message":"Cannot read properties of undefined (reading 'qty')",` +
		`"stack":"TypeError: Cannot read\n    at OrderRail (OrderRail.tsx:40:12)","component_stack":"\n    at OrderRail\n    at OrderScreen"}`
	rec := send(h, http.MethodPost, "/api/v1/client-errors", body, bearerOf(token))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	line := failedLine(t, logs)
	if line == nil {
		t.Fatalf("no line in the log:\n%s", logs)
	}
	for key, want := range map[string]any{
		"level": "ERROR", "app": "terminal", "where": "rail", "path": "/order", "device_kind": "terminal",
		"message": "Cannot read properties of undefined (reading 'qty')", "venue_id": float64(1),
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
	if s, _ := line["stack"].(string); !strings.Contains(s, "\n    at OrderRail") {
		t.Errorf("the stack lost its lines: %q", line["stack"])
	}
	if line["request_id"] == "" || line["device_id"] == nil {
		t.Errorf("the line names no request or device: %v", line)
	}
}

func TestAScreenThatFailedIsRefused(t *testing.T) {
	h, token, logs := clientErrorServer(t)
	tests := []struct {
		name   string
		body   string
		header map[string]string
		status int
		code   ErrorCode
	}{
		{"no device token", `{"app":"terminal"}`, nil, http.StatusUnauthorized, CodeUnauthenticated},
		{"a token that opens nothing", `{"app":"terminal"}`, bearerOf("not-a-token"), http.StatusUnauthorized, ""},
		{"the guest's app", `{"app":"guest"}`, bearerOf(token), http.StatusUnprocessableEntity, CodeInvalidRequest},
		{"no app", `{"message":"x"}`, bearerOf(token), http.StatusUnprocessableEntity, CodeInvalidRequest},
		{"not JSON", `screen=broken`, bearerOf(token), http.StatusBadRequest, CodeInvalidRequest},
		{"a field it does not take", `{"app":"terminal","user":"x"}`, bearerOf(token), http.StatusBadRequest, CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(h, http.MethodPost, "/api/v1/client-errors", tt.body, tt.header)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
			if got := errorOf(t, rec); tt.code != "" && got.Code != tt.code {
				t.Errorf("code %q, want %q", got.Code, tt.code)
			}
		})
	}
	if line := failedLine(t, logs); line != nil {
		t.Errorf("a refused report reached the log: %v", line)
	}
}

// TestAScreenThatFailedIsCut: a stack is cut, not refused, and what a
// browser sent cannot move a terminal's cursor in the log.
func TestAScreenThatFailedIsCut(t *testing.T) {
	h, token, logs := clientErrorServer(t)
	stack := strings.Repeat("at Frame (x.tsx:1:1)\n", 400)
	body, err := json.Marshal(clientErrorRequest{App: "backoffice", Where: "route", Path: "/menu", Message: "boom\x1b[2J\x07", Stack: stack})
	if err != nil {
		t.Fatal(err)
	}
	if rec := send(h, http.MethodPost, "/api/v1/client-errors", string(body), bearerOf(token)); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	line := failedLine(t, logs)
	if line == nil {
		t.Fatal("no line in the log")
	}
	if got, _ := line["stack"].(string); len([]rune(got)) != maxClientStack+3 || !strings.HasSuffix(got, "...") {
		t.Errorf("the stack is %d characters, want %d and an ellipsis", len([]rune(got)), maxClientStack+3)
	}
	if line["message"] != "boom[2J" {
		t.Errorf("message = %q, want the control characters out", line["message"])
	}
}

// The route is open from away and on a mirror: the back office from away
// fails like any other, and a log line changes nothing (ADR-402).
func TestAScreenThatFailedFromAway(t *testing.T) {
	if !remoteAllows("POST /api/v1/client-errors") {
		t.Error("the relay refuses it")
	}
	if !mirrorAllows("POST /api/v1/client-errors") {
		t.Error("a mirror refuses it")
	}
}

func TestClip(t *testing.T) {
	for _, tt := range []struct {
		in   string
		n    int
		want string
	}{
		{"  short  ", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"eleven char", 10, "eleven cha..."},
		{"कखगघङ", 3, "कखग..."},
		{"line\nand\ttab\r\x00", 40, "line\nand\ttab"},
	} {
		if got := clip(tt.in, tt.n); got != tt.want {
			t.Errorf("clip(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

// TestAScreenThatFailedIsCountedPerDevice: a device may report 30 a minute;
// requests refused for want of a token count for nothing, so a stranger
// through the relay cannot spend a till's reports (ADR-402).
func TestAScreenThatFailedIsCountedPerDevice(t *testing.T) {
	h, token, _ := clientErrorServer(t)
	for range 40 {
		if rec := send(h, http.MethodPost, "/api/v1/client-errors", `{"app":"terminal"}`, nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("no token: status %d", rec.Code)
		}
	}
	body := `{"app":"terminal","where":"route","message":"boom"}`
	for i := range clientErrorLimit.requests {
		if rec := send(h, http.MethodPost, "/api/v1/client-errors", body, bearerOf(token)); rec.Code != http.StatusNoContent {
			t.Fatalf("report %d: status %d, body %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := send(h, http.MethodPost, "/api/v1/client-errors", body, bearerOf(token))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("report %d: status %d, Retry-After %q; want 429 and a wait", clientErrorLimit.requests+1, rec.Code, rec.Header().Get("Retry-After"))
	}
}
