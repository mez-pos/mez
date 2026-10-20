package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/service/tablecodes"
)

func TestRequestIDIsAlwaysPresent(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")

	id := resp.Header().Get(HeaderRequestID)
	if id == "" {
		t.Fatal("no request id in the response")
	}
	if len(id) != 32 {
		t.Errorf("generated id = %q, want 32 hex characters", id)
	}
}

func TestRequestIDIsUniquePerRequest(t *testing.T) {
	s := newTestServer(t, testConfig())

	first := get(t, s, "/api/v1/health").Header().Get(HeaderRequestID)
	second := get(t, s, "/api/v1/health").Header().Get(HeaderRequestID)

	if first == second {
		t.Errorf("two requests share the id %q", first)
	}
}

// TestRequestIDFromTheClient covers the trace that starts in the guest app and
// arrives through the relay: one thread, not three.
func TestRequestIDFromTheClient(t *testing.T) {
	tests := []struct {
		name     string
		sent     string
		honoured bool
	}{
		{"a uuid", "3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d", true},
		{"hex", "9fa1b2c3d4e5f60718293a4b5c6d7e8f", true},
		{"empty falls back", "", false},
		// The ones that matter. An id ends up in a log file a human reads, so a
		// newline in it would let a caller forge a second line.
		{"a newline", "abc\ndef", false},
		{"a carriage return", "abc\rdef", false},
		{"a tab", "abc\tdef", false},
		{"too long", strings.Repeat("a", maxRequestIDLen+1), false},
		{"not ascii", "abçde", false},
	}

	s := newTestServer(t, testConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			if tt.sent != "" {
				r.Header.Set(HeaderRequestID, tt.sent)
			}

			got := do(t, s, r).Header().Get(HeaderRequestID)

			switch {
			case tt.honoured && got != tt.sent:
				t.Errorf("id = %q, want the client's %q", got, tt.sent)
			case !tt.honoured && got == tt.sent:
				t.Errorf("id = %q: the client's value was used unchecked", got)
			case !tt.honoured && got == "":
				t.Error("no id generated after rejecting the client's")
			}
		})
	}
}

// TestPanicBecomesA500 is the one that keeps a register selling. A handler will
// be wrong one day; the process must not die with it.
func TestPanicBecomesA500(t *testing.T) {
	s := newTestServer(t, testConfig())
	s.handler = chain(panicking("a nil map, three phases from now"), s.middleware()...)

	resp := get(t, s, "/api/v1/anything")

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}

	var body errorBody
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("panic response is not the envelope: %q", resp.Body)
	}
	if body.Error.Code != CodeInternal {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeInternal)
	}

	// The stack goes to the log, never to the client.
	if strings.Contains(resp.Body.String(), "nil map") {
		t.Errorf("body = %q, want the panic detail kept out of the response", resp.Body)
	}
	if resp.Header().Get(HeaderRequestID) == "" {
		t.Error("no request id on the 500: the log line and the response cannot be matched up")
	}
}

// TestTheAccessLogSeesTheStatusTheClientSaw is what fixes the order of the
// chain. accessLog has to sit outside recoverPanic: inside, a panic unwinds
// past its deferred line before recovery writes the 500, and the line claims a
// 200 for exactly the requests somebody will need to explain. Verified by
// swapping the two, which turns this into "status = 200, want 500".
func TestTheAccessLogSeesTheStatusTheClientSaw(t *testing.T) {
	log, lines := capturingLogger()

	s := New(testConfig(), log, BuildInfo{Version: "test"})
	s.handler = chain(panicking("boom"), s.middleware()...)

	if code := get(t, s, "/api/v1/anything").Code; code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", code, http.StatusInternalServerError)
	}

	entry := findLine(t, lines.String(), "request")
	if got := entry["status"]; got != float64(http.StatusInternalServerError) {
		t.Errorf("access log status = %v, want 500: the chain is in the wrong order", got)
	}
	if entry["request_id"] == "" {
		t.Error("access log line carries no request_id")
	}
}

// TestThePanicIsLoggedWithItsStack: the client is told nothing, so the log is
// the only record of what actually broke.
func TestThePanicIsLoggedWithItsStack(t *testing.T) {
	log, lines := capturingLogger()

	s := New(testConfig(), log, BuildInfo{Version: "test"})
	s.handler = chain(panicking("a nil map"), s.middleware()...)
	get(t, s, "/api/v1/anything")

	entry := findLine(t, lines.String(), "handler panicked")
	if got, _ := entry["panic"].(string); !strings.Contains(got, "a nil map") {
		t.Errorf("logged panic = %q, want the detail", got)
	}
	if got, _ := entry["stack"].(string); !strings.Contains(got, "api.") {
		t.Errorf("logged stack = %q, want a stack trace", got)
	}
}

// TestAbortHandlerPassesThrough: net/http uses ErrAbortHandler to drop a
// response deliberately, and swallowing it would turn an intentional abort into
// a fabricated 500.
func TestAbortHandlerPassesThrough(t *testing.T) {
	s := newTestServer(t, testConfig())
	aborting := chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}), s.recoverPanic)

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Errorf("recovered %v, want it to re-panic with http.ErrAbortHandler", recovered)
		}
	}()

	aborting.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestCORS(t *testing.T) {
	const devOrigin = "http://localhost:5173"

	withCORS := testConfig()
	withCORS.DevOrigin = devOrigin

	t.Run("the configured origin is allowed", func(t *testing.T) {
		resp := withOrigin(t, newTestServer(t, withCORS), devOrigin)
		if got := resp.Header().Get("Access-Control-Allow-Origin"); got != devOrigin {
			t.Errorf("allow-origin = %q, want %q", got, devOrigin)
		}
		if got := resp.Header().Get("Vary"); !strings.Contains(got, "Origin") {
			t.Errorf("Vary = %q, want it to include Origin", got)
		}
	})

	t.Run("any other origin is not", func(t *testing.T) {
		resp := withOrigin(t, newTestServer(t, withCORS), "https://evil.example.com")
		if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want nothing for an unconfigured origin", got)
		}
	})

	// The one that protects a shipped binary: no dev_origin, no CORS at all.
	t.Run("a shipped binary sends no CORS headers", func(t *testing.T) {
		resp := withOrigin(t, newTestServer(t, testConfig()), devOrigin)
		if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want nothing when dev_origin is unset", got)
		}
	})
}

func TestCORSPreflight(t *testing.T) {
	const devOrigin = "http://localhost:5173"
	cfg := testConfig()
	cfg.DevOrigin = devOrigin

	preflight := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", http.MethodPost)
		return do(t, newTestServer(t, cfg), r)
	}

	t.Run("allowed origin", func(t *testing.T) {
		resp := preflight(devOrigin)
		if resp.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", resp.Code, http.StatusNoContent)
		}
		if got := resp.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("no allow-methods on an accepted preflight")
		}
		if got := resp.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Idempotency-Key") {
			t.Errorf("allow-headers = %q, want Idempotency-Key: every mutating endpoint takes it", got)
		}
	})

	t.Run("refused origin gets no permissions", func(t *testing.T) {
		resp := preflight("https://evil.example.com")
		if got := resp.Header().Get("Access-Control-Allow-Methods"); got != "" {
			t.Errorf("allow-methods = %q, want nothing", got)
		}
	})
}

// TestRecorderReportsWhatHappened: the access log is only as true as this, and
// a status of 0 in a log line is how a real incident gets misread.
func TestRecorderReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name   string
		handle func(http.ResponseWriter)
		status int
		bytes  int64
	}{
		{
			name:   "an explicit status",
			handle: func(w http.ResponseWriter) { w.WriteHeader(http.StatusTeapot) },
			status: http.StatusTeapot,
		},
		{
			name:   "a write with no status",
			handle: func(w http.ResponseWriter) { w.Write([]byte("hello")) },
			status: http.StatusOK,
			bytes:  5,
		},
		{
			name:   "a handler that did nothing",
			handle: func(http.ResponseWriter) {},
			status: http.StatusOK,
		},
		{
			name: "only the first status counts",
			handle: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusCreated)
				w.WriteHeader(http.StatusTeapot)
			},
			status: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{ResponseWriter: httptest.NewRecorder()}
			tt.handle(rec)

			if got := rec.statusCode(); got != tt.status {
				t.Errorf("status = %d, want %d", got, tt.status)
			}
			if rec.written != tt.bytes {
				t.Errorf("bytes = %d, want %d", rec.written, tt.bytes)
			}
		})
	}
}

// TestRecorderDoesNotHideTheWriter guards the WebSocket upgrade Phase 04 needs.
// A wrapper without Unwrap is how that breaks, quietly, three phases from now.
func TestRecorderDoesNotHideTheWriter(t *testing.T) {
	under := httptest.NewRecorder()
	rec := &recorder{ResponseWriter: under}

	if rec.Unwrap() != http.ResponseWriter(under) {
		t.Error("Unwrap does not return the underlying writer")
	}
	if _, ok := any(rec).(http.Flusher); !ok {
		t.Error("recorder is not an http.Flusher")
	}
	if _, ok := any(rec).(http.Hijacker); !ok {
		t.Error("recorder is not an http.Hijacker: the WebSocket upgrade will fail")
	}
}

func panicking(with string) http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(with) })
}

func capturingLogger() (*slog.Logger, *bytes.Buffer) {
	var lines bytes.Buffer
	return slog.New(slog.NewJSONHandler(&lines, &slog.HandlerOptions{Level: slog.LevelDebug})), &lines
}

// findLine returns the first log record with the given message.
func findLine(t *testing.T, logged, msg string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(logged), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry["msg"] == msg {
			return entry
		}
	}
	t.Fatalf("no log line with msg %q in:\n%s", msg, logged)
	return nil
}

func withOrigin(t *testing.T, s *Server, origin string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return do(t, s, r)
}

// TestAPanicAfterTheHeadersDoesNotWriteAgain: once a handler has begun its
// response the 500 cannot be written, and trying would at best be noise in
// the log and at worst a write to a hijacked connection.
func TestAPanicAfterTheHeadersDoesNotWriteAgain(t *testing.T) {
	s := newTestServer(t, testConfig())
	s.handler = chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("after the headers")
	}), s.middleware()...)

	resp := get(t, s, "/api/v1/anything")
	if resp.Code != http.StatusAccepted {
		t.Errorf("status = %d, want the 202 the handler wrote first", resp.Code)
	}
	if strings.Contains(resp.Body.String(), string(CodeInternal)) {
		t.Errorf("body = %q, want no error envelope written over a begun response", resp.Body)
	}
}

// TestTheLogMasksAGuestsCode: a guest's path carries the table's code,
// which is the credential, and the log must not hand it to whoever reads
// the log.
func TestTheLogMasksAGuestsCode(t *testing.T) {
	tests := []struct {
		pattern, path, want string
	}{
		{"GET /order/api/t/{code}/menu", "/order/api/t/ABCDEFGHJKLM/menu", "/order/api/t/[code]/menu"},
		{"POST /order/api/t/{code}/orders", "/order/api/t/ABCDEFGHJKLM/orders", "/order/api/t/[code]/orders"},
		{"GET /order/api/c/{code}", "/order/api/c/ABCDEFGHJKLM", "/order/api/c/[code]"},
		{"GET /api/v1/orders/{id}", "/api/v1/orders/ord_1", "/api/v1/orders/ord_1"},
		{"", "/order/api/t/ABCDEFGHJKLM/menux", "/order/api/t/[code]/menux"},
		{"", "/order/api/c/ABCDEFGHJKLM", "/order/api/c/[code]"},
		{"", "/order/t/ABCDEFGHJKLM", "/order/t/ABCDEFGHJKLM"},
		{"", "/api/v1/t/x", "/api/v1/t/x"},
	}
	s := newTestServer(t, testConfig())
	s.guestMount = "/order/"
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.path, nil)
		r.Pattern = tt.pattern
		if got := s.loggedPath(r); got != tt.want {
			t.Errorf("loggedPath(%q, %q) = %q, want %q", tt.pattern, tt.path, got, tt.want)
		}
	}
}

// TestTheAccessLogCarriesNoTableCode goes through the real router: the
// pattern is set by the mux, and the line must still not carry the code.
func TestTheAccessLogCarriesNoTableCode(t *testing.T) {
	log, lines := capturingLogger()
	s := New(testConfig(), log, BuildInfo{Version: "test"}, WithGuests(stubGuests{}, "/order/"))
	get(t, s, "/order/api/t/ABCDEFGHJKLM")
	get(t, s, "/order/api/t/ABCDEFGHJKLM/no-such-thing")
	if out := lines.String(); strings.Contains(out, "ABCDEFGHJKLM") {
		t.Errorf("the access log carries the table code:\n%s", out)
	} else if !strings.Contains(out, "[code]") {
		t.Errorf("the access log does not show where the code was:\n%s", out)
	}
}

// TestAClientMayNotSendASystemIdempotencyKey: the keys the system appends
// under have a namespace of their own, and a client's key in it would make
// a gateway's tender fail as a reuse.
func TestAClientMayNotSendASystemIdempotencyKey(t *testing.T) {
	s := newTestServer(t, testConfig())
	for _, key := range []string{"charge:chg_1", "pickup-ticket:rnd_1", strings.Repeat("k", 129), "has space"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		r.Header.Set(HeaderIdempotencyKey, key)
		resp := httptest.NewRecorder()
		s.Handler().ServeHTTP(resp, r)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("key %q: status = %d, want 400", key, resp.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.Header.Set(HeaderIdempotencyKey, "5f1c2b1e-3a4d-4c9e-9f1a-2b3c4d5e6f70")
	resp := httptest.NewRecorder()
	s.Handler().ServeHTTP(resp, r)
	if resp.Code != http.StatusOK {
		t.Errorf("an ordinary key: status = %d, want 200", resp.Code)
	}
}

// stubGuests is a Guests that knows no code, enough for the router to match
// a guest pattern.
type stubGuests struct{}

func (stubGuests) Resolve(context.Context, string) (tablecodes.Table, error) {
	return tablecodes.Table{}, tablecodes.ErrUnknown
}

func (stubGuests) ResolvePickup(context.Context, string) (tablecodes.Venue, error) {
	return tablecodes.Venue{}, tablecodes.ErrUnknown
}
