package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/config"
)

// TestRouting covers the shape of every answer this API can currently give. The
// error cases matter as much as the success one: 03-conventions.md fixes the
// envelope, and net/http's defaults are plain text, so an unhandled path or
// method silently breaks the contract unless it is routed deliberately.
func TestRouting(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
		code   ErrorCode // expected error code, empty for a success
	}{
		{
			name:   "health",
			method: http.MethodGet,
			path:   "/api/v1/health",
			status: http.StatusOK,
		},
		{
			name:   "health rejects other methods",
			method: http.MethodPost,
			path:   "/api/v1/health",
			status: http.StatusMethodNotAllowed,
			code:   CodeMethodNotAllowed,
		},
		{
			name:   "unknown path under the api prefix",
			method: http.MethodGet,
			path:   "/api/v1/orders",
			status: http.StatusNotFound,
			code:   CodeNotFound,
		},
		{
			name:   "unknown path outside it",
			method: http.MethodGet,
			path:   "/nope",
			status: http.StatusNotFound,
			code:   CodeNotFound,
		},
	}

	s := newTestServer(t, testConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := do(t, s, httptest.NewRequest(tt.method, tt.path, nil))

			if resp.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %q)", resp.Code, tt.status, resp.Body)
			}
			if tt.code == "" {
				return
			}

			var body errorBody
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("error response is not the envelope: %q", resp.Body)
			}
			if body.Error.Code != tt.code {
				t.Errorf("error.code = %q, want %q", body.Error.Code, tt.code)
			}
			if body.Error.Message == "" {
				t.Error("error.message is empty: a human reads this one")
			}
			if got := resp.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Errorf("content-type = %q, want JSON even on an error", got)
			}
		})
	}
}

// TestResponsesRefuseToBeSniffed: the register shares a LAN with printers and
// tablets, and nothing it serves should ever be guessed into being executable.
func TestResponsesRefuseToBeSniffed(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// helpers shared by every test in this package.

func testConfig() config.Config {
	return config.Config{
		ListenAddr: "127.0.0.1:0",
		DataDir:    ".",
		LogLevel:   slog.LevelInfo,
		LogFormat:  config.LogFormatJSON,
		Timezone:   time.UTC,
	}
}

func newTestServer(t *testing.T, cfg config.Config, opts ...Option) *Server {
	t.Helper()
	return New(cfg, quietLogger(), BuildInfo{
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-09-17",
	}, opts...)
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, httptest.NewRequest(http.MethodGet, path, nil))
}

func do(t *testing.T, s *Server, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	resp := httptest.NewRecorder()
	s.Handler().ServeHTTP(resp, r)
	return resp
}

func equal(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

// stoppedClock makes uptime testable. Without it the only honest assertion
// about uptime_seconds is that it is a number.
type stoppedClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *stoppedClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *stoppedClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}
