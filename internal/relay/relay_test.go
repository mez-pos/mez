package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

// testTimeout bounds every wait in these tests. Nothing here should take a
// tenth of it.
const testTimeout = 10 * time.Second

// syncBuffer is a log sink the server's goroutines and the test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fixture is a relay on a real listener, and the keys to connect to it.
type fixture struct {
	t    *testing.T
	srv  *Server
	http *httptest.Server
	keys *Keys
	logs *syncBuffer
}

func newFixture(t *testing.T, tune func(*Options)) *fixture {
	t.Helper()
	logs := &syncBuffer{}
	keys := mustKeys(t, testSecret)
	opts := Options{
		Keys:    keys,
		Version: "test",
		Log:     slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	if tune != nil {
		tune(&opts)
	}
	srv, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		srv.Close()
		ts.Close()
	})
	return &fixture{t: t, srv: srv, http: ts, keys: opts.Keys, logs: logs}
}

// dial connects as a venue and returns the raw socket.
func (f *fixture) dial(key string) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	header := http.Header{}
	if key != "" {
		header.Set("Authorization", "Bearer "+key)
	}
	ws, resp, err := websocket.Dial(ctx, f.http.URL+"/agent", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		ws.SetReadLimit(wire.MaxFrame)
		f.t.Cleanup(func() { _ = ws.CloseNow() })
	}
	return ws, resp, err
}

// connect is dial for a venue that must get in, and waits until the relay
// routes to it.
func (f *fixture) connect(venueID string) *websocket.Conn {
	f.t.Helper()
	ws, _, err := f.dial(mustIssue(f.t, f.keys, venueID))
	if err != nil {
		f.t.Fatalf("venue %s could not connect: %v", venueID, err)
	}
	f.waitFor("venue "+venueID+" registered", func() bool {
		a := f.srv.agentFor(venueID)
		return a != nil && a.ctx.Err() == nil
	})
	return ws
}

func (f *fixture) waitFor(what string, cond func() bool) {
	f.t.Helper()
	deadline := time.Now().Add(testTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			f.t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// exchange is one guest request as a venue received it.
type exchange struct {
	head wire.RequestHead
	body []byte
}

// answer is what a test venue sends back.
type answer struct {
	status int
	header http.Header
	body   []byte
}

// serve runs a well-behaved venue on ws: it collects each request, asks
// handle for the answer, and sends it back in chunks. It stops when the
// socket does.
func serve(t *testing.T, ws *websocket.Conn, handle func(exchange) answer) {
	t.Helper()
	go func() {
		pending := map[uint64]*exchange{}
		for {
			f, err := readFrame(ws)
			if err != nil {
				return
			}
			switch f.Type {
			case wire.TypeRequest:
				head, err := wire.DecodeRequest(f)
				if err != nil {
					t.Errorf("venue got an unreadable request head: %v", err)
					return
				}
				pending[f.Stream] = &exchange{head: head}
			case wire.TypeBody:
				if ex := pending[f.Stream]; ex != nil {
					ex.body = append(ex.body, f.Payload...)
				}
			case wire.TypeReset:
				delete(pending, f.Stream)
			case wire.TypeEnd:
				ex := pending[f.Stream]
				delete(pending, f.Stream)
				if ex == nil {
					continue
				}
				if err := respond(ws, f.Stream, handle(*ex)); err != nil {
					return
				}
			}
		}
	}()
}

func respond(ws *websocket.Conn, stream uint64, a answer) error {
	if err := writeHead(ws, stream, a.status, a.header); err != nil {
		return err
	}
	for body := a.body; len(body) > 0; {
		n := min(len(body), wire.MaxChunk)
		if err := writeFrame(ws, wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: body[:n]}); err != nil {
			return err
		}
		body = body[n:]
	}
	return writeFrame(ws, wire.Frame{Type: wire.TypeEnd, Stream: stream})
}

func readFrame(ws *websocket.Conn) (wire.Frame, error) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, msg, err := ws.Read(ctx)
	if err != nil {
		return wire.Frame{}, err
	}
	return wire.Decode(msg)
}

func writeFrame(ws *websocket.Conn, f wire.Frame) error {
	msg, err := wire.Encode(f)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	return ws.Write(ctx, websocket.MessageBinary, msg)
}

func writeHead(ws *websocket.Conn, stream uint64, status int, header http.Header) error {
	msg, err := wire.EncodeHead(wire.TypeResponse, stream, wire.ResponseHead{Status: status, Header: header})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	return ws.Write(ctx, websocket.MessageBinary, msg)
}

// awaitRequest reads frames until a request has fully arrived, for tests
// that drive the venue's side by hand.
func awaitRequest(t *testing.T, ws *websocket.Conn) (uint64, wire.RequestHead) {
	t.Helper()
	var head wire.RequestHead
	for {
		f, err := readFrame(ws)
		if err != nil {
			t.Fatalf("venue waiting for a request: %v", err)
		}
		switch f.Type {
		case wire.TypeRequest:
			if head, err = wire.DecodeRequest(f); err != nil {
				t.Fatalf("venue got an unreadable request head: %v", err)
			}
		case wire.TypeEnd:
			return f.Stream, head
		}
	}
}

// awaitReset reads frames until the relay resets a stream.
func awaitReset(t *testing.T, ws *websocket.Conn, stream uint64) {
	t.Helper()
	for {
		f, err := readFrame(ws)
		if err != nil {
			t.Fatalf("venue waiting for a reset: %v", err)
		}
		if f.Type == wire.TypeReset && f.Stream == stream {
			return
		}
	}
}

func (f *fixture) do(method, path string, body io.Reader, header http.Header) *http.Response {
	f.t.Helper()
	req, err := http.NewRequest(method, f.http.URL+path, body)
	if err != nil {
		f.t.Fatalf("building the request: %v", err)
	}
	for name, values := range header {
		req.Header[name] = values
	}
	client := &http.Client{
		Timeout:       testTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	f.t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	return string(b)
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var body errorBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("the response is not an error envelope: %v", err)
	}
	return body.Error.Code
}

func TestNewRefusesIncompleteOptions(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	keys := mustKeys(t, testSecret)
	tests := []struct {
		name string
		opts Options
	}{
		{"no keys", Options{Log: log}},
		{"no log", Options{Keys: keys}},
		{"a rate with no burst", Options{Keys: keys, Log: log, Limit: Limit{PerMinute: 60}}},
		{"a negative rate", Options{Keys: keys, Log: log, Limit: Limit{PerMinute: -1, Burst: 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.opts); err == nil {
				t.Error("New accepted it")
			}
		})
	}
}

func TestHealth(t *testing.T) {
	f := newFixture(t, nil)
	resp := f.do("GET", "/healthz", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got.Status != "ok" || got.Version != "test" {
		t.Errorf("health = %+v", got)
	}
}

func TestAgentAuth(t *testing.T) {
	f := newFixture(t, func(o *Options) {
		keys, err := NewKeys(testSecret, []string{"shut"})
		if err != nil {
			t.Fatal(err)
		}
		o.Keys = keys
	})
	issuer := mustKeys(t, testSecret)

	tests := []struct {
		name string
		key  string
	}{
		{"no key", ""},
		{"not a key", "hunter2"},
		{"signed by another relay", mustIssue(t, mustKeys(t, strings.Repeat("z", MinSecretLen)), "cafe")},
		{"revoked venue", mustIssue(t, issuer, "shut")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, resp, err := f.dial(tt.key)
			if err == nil {
				t.Fatal("the relay let it in")
			}
			if resp == nil || resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("response = %v, want 401", resp)
			}
			if code := errorCode(t, resp); code != codeUnauthorized {
				t.Errorf("code = %q, want %q", code, codeUnauthorized)
			}
		})
	}
	if strings.Contains(f.logs.String(), "hunter2") {
		t.Error("a refused key was written to the log")
	}
}

func TestGuestRoundTrip(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	got := make(chan exchange, 1)
	serve(t, ws, func(ex exchange) answer {
		got <- ex
		return answer{
			status: http.StatusCreated,
			header: http.Header{
				"Content-Type": {"application/json"},
				"Set-Cookie":   {"a=1", "b=2"},
				"Connection":   {"X-Venue-Only"},
				"X-Venue-Only": {"must not reach the guest"},
			},
			body: []byte(`{"ok":true}`),
		}
	})

	resp := f.do("POST", "/v/cafe/t/a%2Fb?seat=2&x=%20", strings.NewReader("two flat whites"), http.Header{
		"X-Forwarded-For":    {"6.6.6.6"},
		"X-Forwarded-Prefix": {"/evil"},
		"X-Real-Ip":          {"6.6.6.6"},
		"X-Request-Id":       {"trace-123"},
		"Content-Type":       {"text/plain"},
		"Connection":         {"X-Guest-Hop"},
		"X-Guest-Hop":        {"must not reach the venue"},
	})

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if body := readAll(t, resp); body != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cookies := resp.Header.Values("Set-Cookie"); len(cookies) != 2 {
		t.Errorf("Set-Cookie = %q, want both", cookies)
	}
	if resp.Header.Get("X-Venue-Only") != "" {
		t.Error("a header the venue named in Connection reached the guest")
	}
	if id := resp.Header.Get("X-Request-Id"); id != "trace-123" {
		t.Errorf("X-Request-Id = %q, want the guest's own", id)
	}

	ex := <-got
	if ex.head.Method != "POST" {
		t.Errorf("method = %q", ex.head.Method)
	}
	// The prefix is gone, and the rest is byte for byte what the guest sent.
	if ex.head.Path != "/t/a%2Fb?seat=2&x=%20" {
		t.Errorf("path = %q", ex.head.Path)
	}
	if string(ex.body) != "two flat whites" {
		t.Errorf("body = %q", ex.body)
	}
	h := ex.head.Header
	if got := h.Values("X-Forwarded-For"); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Errorf("X-Forwarded-For = %q, want only the address the relay saw", got)
	}
	if got := h.Get("X-Forwarded-Prefix"); got != "/v/cafe" {
		t.Errorf("X-Forwarded-Prefix = %q", got)
	}
	if got := h.Get("X-Forwarded-Proto"); got != "http" {
		t.Errorf("X-Forwarded-Proto = %q", got)
	}
	if got := h.Get("X-Forwarded-Host"); got != strings.TrimPrefix(f.http.URL, "http://") {
		t.Errorf("X-Forwarded-Host = %q", got)
	}
	if h.Get("X-Real-Ip") != "" || h.Get("X-Guest-Hop") != "" || h.Get("Connection") != "" {
		t.Errorf("headers that should have been dropped reached the venue: %v", h)
	}
	if h.Get("X-Request-Id") != "trace-123" || h.Get("Content-Type") != "text/plain" {
		t.Errorf("headers that should have been kept did not reach the venue: %v", h)
	}

	// One structured line for the request, naming the venue.
	f.waitFor("the access log line", func() bool {
		for _, line := range strings.Split(f.logs.String(), "\n") {
			var entry map[string]any
			if json.Unmarshal([]byte(line), &entry) != nil || entry["path"] != "/v/cafe/t/a/b" {
				continue
			}
			return entry["venue_id"] == "cafe" && entry["request_id"] == "trace-123" &&
				entry["status"] == float64(201) && entry["method"] == "POST"
		}
		return false
	})
	if strings.Contains(f.logs.String(), "seat=2") {
		t.Error("the query string was written to the log")
	}
}

func TestCopyResponseHeaderKeepsAVenueToItsPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   http.Header
		want http.Header
	}{
		{"an ordinary header passes, canonical",
			http.Header{"content-type": {"text/html"}},
			http.Header{"Content-Type": {"text/html"}}},
		{"a cookie for the root is kept to the venue",
			http.Header{"Set-Cookie": {"sid=abc; Path=/; HttpOnly; SameSite=Lax"}},
			http.Header{"Set-Cookie": {"sid=abc; Path=/v/cafe/; HttpOnly; SameSite=Lax"}}},
		{"a cookie for a path keeps it, under the venue",
			http.Header{"Set-Cookie": {"cart=1; Path=/t/abc"}},
			http.Header{"Set-Cookie": {"cart=1; Path=/v/cafe/t/abc"}}},
		{"a cookie with no path gets the venue's",
			http.Header{"Set-Cookie": {"a=1"}},
			http.Header{"Set-Cookie": {"a=1; Path=/v/cafe/"}}},
		{"a cookie may not name a domain",
			http.Header{"Set-Cookie": {"a=1; Domain=example.com; Path=/"}},
			http.Header{"Set-Cookie": {"a=1; Path=/v/cafe/"}}},
		{"an unreadable cookie is dropped, a good one beside it kept",
			http.Header{"Set-Cookie": {"=; Path=/", "b=2; Path=/"}},
			http.Header{"Set-Cookie": {"b=2; Path=/v/cafe/"}}},
		{"a redirect to the venue's own path goes under the prefix",
			http.Header{"Location": {"/t/abc?x=1"}},
			http.Header{"Location": {"/v/cafe/t/abc?x=1"}}},
		{"a redirect elsewhere is left alone",
			http.Header{"Location": {"https://pay.example/upi"}},
			http.Header{"Location": {"https://pay.example/upi"}}},
		{"a scheme-relative redirect is not a path",
			http.Header{"Location": {"//pay.example/upi"}},
			http.Header{"Location": {"//pay.example/upi"}}},
		{"a relative redirect is already under the prefix",
			http.Header{"Location": {"done"}},
			http.Header{"Location": {"done"}}},
		{"headers that act on the whole origin are refused",
			http.Header{"Service-Worker-Allowed": {"/"}, "Clear-Site-Data": {`"*"`}, "service-worker-allowed": {"/"}, "Etag": {"x"}},
			http.Header{"Etag": {"x"}}},
		{"hop-by-hop headers are refused",
			http.Header{"Transfer-Encoding": {"chunked"}, "Upgrade": {"websocket"}, "Etag": {"x"}},
			http.Header{"Etag": {"x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := http.Header{}
			copyResponseHeader(got, tt.in, "/v/cafe")
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrustProxy(t *testing.T) {
	tests := []struct {
		name      string
		trust     bool
		wantIP    string
		wantProto string
	}{
		// 9.9.9.9 is what the client claimed; 203.0.113.7 is what the
		// operator's proxy appended after it.
		{"trusted: the proxy's entry, the last one", true, "203.0.113.7", "https"},
		{"untrusted: the socket's address", false, "127.0.0.1", "http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, func(o *Options) { o.TrustProxy = tt.trust })
			ws := f.connect("cafe")
			got := make(chan exchange, 1)
			serve(t, ws, func(ex exchange) answer {
				got <- ex
				return answer{status: http.StatusNoContent}
			})
			f.do("GET", "/v/cafe/", nil, http.Header{
				"X-Forwarded-For":   {"9.9.9.9, 203.0.113.7"},
				"X-Forwarded-Proto": {"https"},
			})
			h := (<-got).head.Header
			if ip := h.Get("X-Forwarded-For"); ip != tt.wantIP {
				t.Errorf("X-Forwarded-For = %q, want %q", ip, tt.wantIP)
			}
			if proto := h.Get("X-Forwarded-Proto"); proto != tt.wantProto {
				t.Errorf("X-Forwarded-Proto = %q, want %q", proto, tt.wantProto)
			}
		})
	}
}

func TestLargeBodiesBothWays(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")
	// Several frames each way, and not a multiple of the chunk size.
	up := bytes.Repeat([]byte("order "), 20000)
	down := bytes.Repeat([]byte("bundle"), 50001)
	serve(t, ws, func(ex exchange) answer {
		if !bytes.Equal(ex.body, up) {
			t.Errorf("the venue received %d bytes, want %d intact", len(ex.body), len(up))
		}
		return answer{status: http.StatusOK, body: down}
	})

	resp := f.do("PUT", "/v/cafe/upload", bytes.NewReader(up), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if body := readAll(t, resp); body != string(down) {
		t.Errorf("the guest received %d bytes, want %d intact", len(body), len(down))
	}
}

func TestConcurrentGuestsShareOneSocket(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")
	serve(t, ws, func(ex exchange) answer {
		return answer{status: http.StatusOK, body: []byte("you asked for " + ex.head.Path)}
	})

	const guests = 40
	var wg sync.WaitGroup
	errs := make(chan error, guests)
	for i := range guests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := fmt.Sprintf("/item/%d", i)
			resp, err := http.Get(f.http.URL + "/v/cafe" + path)
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "you asked for "+path {
				errs <- fmt.Errorf("guest %d got %q", i, body)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestResponseStreamsAsItArrives(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	// The guest opens the gate once it has the first event in hand.
	gate := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		stream, _ := awaitRequest(t, ws)
		if err := writeHead(ws, stream, http.StatusOK, http.Header{"Content-Type": {"text/event-stream"}}); err != nil {
			t.Errorf("venue: %v", err)
			return
		}
		_ = writeFrame(ws, wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: []byte("event: accepted\n")})
		// The stream stays open until the guest has proved it got the first
		// event, so a relay that buffered would hang here and fail.
		<-gate
		_ = writeFrame(ws, wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: []byte("event: ready\n")})
		_ = writeFrame(ws, wire.Frame{Type: wire.TypeEnd, Stream: stream})
	}()

	resp := f.do("GET", "/v/cafe/orders/1/events", nil, nil)
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != "event: accepted\n" {
		t.Fatalf("first event = %q, %v", line, err)
	}
	close(gate)
	rest, err := io.ReadAll(reader)
	if err != nil || string(rest) != "event: ready\n" {
		t.Errorf("rest = %q, %v", rest, err)
	}
	<-done
}

func TestGuestRefusals(t *testing.T) {
	f := newFixture(t, nil)

	t.Run("venue not connected, to an API client", func(t *testing.T) {
		resp := f.do("GET", "/v/nowhere/t/abc", nil, nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", resp.StatusCode)
		}
		if code := errorCode(t, resp); code != codeVenueOffline {
			t.Errorf("code = %q", code)
		}
	})
	t.Run("venue not connected, to a browser", func(t *testing.T) {
		resp := f.do("GET", "/v/nowhere/t/abc", nil, http.Header{"Accept": {"text/html,*/*"}})
		body := readAll(t, resp)
		if resp.StatusCode != http.StatusServiceUnavailable ||
			!strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") ||
			!strings.Contains(body, "order at the counter") {
			t.Errorf("status %d, type %q, body %q", resp.StatusCode, resp.Header.Get("Content-Type"), body)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Error("an offline page may be cached, and would outlive the outage")
		}
	})
	t.Run("venue id that cannot be one", func(t *testing.T) {
		for _, path := range []string{"/v/Cafe/t/abc", "/v/a.b/", "/v/-x/", "/v/caf%65/menu", "/v/Cafe"} {
			if resp := f.do("GET", path, nil, nil); resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s: status = %d, want 404", path, resp.StatusCode)
			}
		}
	})
	t.Run("bare venue redirects under its prefix", func(t *testing.T) {
		resp := f.do("GET", "/v/cafe?x=1", nil, nil)
		if resp.StatusCode != http.StatusPermanentRedirect || resp.Header.Get("Location") != "/v/cafe/?x=1" {
			t.Errorf("status %d, Location %q", resp.StatusCode, resp.Header.Get("Location"))
		}
	})
	t.Run("nothing else is served", func(t *testing.T) {
		for _, path := range []string{"/", "/api/v1/health", "/v/"} {
			if resp := f.do("GET", path, nil, nil); resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s: status = %d, want 404", path, resp.StatusCode)
			}
		}
	})
}

func TestRequestBodyTooLarge(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	reset := make(chan struct{})
	go func() {
		defer close(reset)
		first, err := readFrame(ws)
		if err != nil {
			t.Errorf("venue: %v", err)
			return
		}
		awaitReset(t, ws, first.Stream)
	}()

	resp := f.do("POST", "/v/cafe/orders", bytes.NewReader(make([]byte, maxRequestBody+1)), nil)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != codeTooLarge {
		t.Errorf("code = %q", code)
	}
	// The venue is told to drop the half-sent request.
	<-reset
}

func TestRateLimitIsPerVenue(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	f := newFixture(t, func(o *Options) {
		o.Limit = Limit{PerMinute: 60, Burst: 2}
		o.Now = clock
	})
	for _, venue := range []string{"cafe", "bar"} {
		serve(t, f.connect(venue), func(exchange) answer { return answer{status: http.StatusNoContent} })
	}

	for i := range 2 {
		if resp := f.do("GET", "/v/cafe/", nil, nil); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("request %d of the burst: status = %d", i+1, resp.StatusCode)
		}
	}
	resp := f.do("GET", "/v/cafe/", nil, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("past the burst: status = %d, want 429", resp.StatusCode)
	}
	if retry := resp.Header.Get("Retry-After"); retry != "2" {
		t.Errorf("Retry-After = %q, want 2 (one second, rounded up and never zero)", retry)
	}
	if code := errorCode(t, resp); code != codeRateLimited {
		t.Errorf("code = %q", code)
	}

	// One venue's rush is not another's problem.
	if resp := f.do("GET", "/v/bar/", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("the other venue: status = %d", resp.StatusCode)
	}

	mu.Lock()
	now = now.Add(time.Second)
	mu.Unlock()
	if resp := f.do("GET", "/v/cafe/", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("a second later: status = %d", resp.StatusCode)
	}
}

func TestVenueNeverAnswers(t *testing.T) {
	f := newFixture(t, func(o *Options) { o.HeadTimeout = 50 * time.Millisecond })
	ws := f.connect("cafe")

	reset := make(chan struct{})
	go func() {
		defer close(reset)
		stream, _ := awaitRequest(t, ws)
		awaitReset(t, ws, stream)
	}()

	resp := f.do("GET", "/v/cafe/menu", nil, nil)
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != codeTimeout {
		t.Errorf("code = %q", code)
	}
	<-reset

	// The abandoned stream is not left counting against the venue.
	f.waitFor("the stream to be forgotten", func() bool { return f.srv.agentFor("cafe").inFlight() == 0 })
}

func TestVenueMisbehaves(t *testing.T) {
	tests := []struct {
		name string
		send func(ws *websocket.Conn, stream uint64) error
	}{
		{"resets before answering", func(ws *websocket.Conn, stream uint64) error {
			return writeFrame(ws, wire.Frame{Type: wire.TypeReset, Stream: stream, Payload: []byte("no")})
		}},
		{"ends before answering", func(ws *websocket.Conn, stream uint64) error {
			return writeFrame(ws, wire.Frame{Type: wire.TypeEnd, Stream: stream})
		}},
		{"sends a body before a head", func(ws *websocket.Conn, stream uint64) error {
			return writeFrame(ws, wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: []byte("x")})
		}},
		{"answers with a status that is not one", func(ws *websocket.Conn, stream uint64) error {
			return writeHead(ws, stream, 99, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, nil)
			ws := f.connect("cafe")
			go func() {
				stream, _ := awaitRequest(t, ws)
				if err := tt.send(ws, stream); err != nil {
					t.Errorf("venue: %v", err)
				}
			}()
			resp := f.do("GET", "/v/cafe/menu", nil, nil)
			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != codeBadGateway {
				t.Errorf("code = %q", code)
			}
		})
	}
}

func TestVenueSpeaksNonsenseIsDisconnected(t *testing.T) {
	tests := []struct {
		name string
		send func(ws *websocket.Conn) error
	}{
		{"a text message", func(ws *websocket.Conn) error {
			return ws.Write(context.Background(), websocket.MessageText, []byte("hello"))
		}},
		{"a frame too short to be one", func(ws *websocket.Conn) error {
			return ws.Write(context.Background(), websocket.MessageBinary, []byte{1, 2, 3})
		}},
		{"a request, which only the relay may send", func(ws *websocket.Conn) error {
			msg, err := wire.EncodeHead(wire.TypeRequest, 1, wire.RequestHead{Method: "GET", Path: "/"})
			if err != nil {
				return err
			}
			return ws.Write(context.Background(), websocket.MessageBinary, msg)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, nil)
			ws := f.connect("cafe")
			if err := tt.send(ws); err != nil {
				t.Fatalf("venue: %v", err)
			}
			_, err := readFrame(ws)
			if got := websocket.CloseStatus(err); got != wire.CloseProtocol {
				t.Errorf("close status = %d (%v), want %d", got, err, wire.CloseProtocol)
			}
			f.waitFor("the venue to be forgotten", func() bool { return f.srv.agentFor("cafe") == nil })
		})
	}
}

func TestVenueDropsMidResponse(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")
	go func() {
		stream, _ := awaitRequest(t, ws)
		_ = writeHead(ws, stream, http.StatusOK, http.Header{"Content-Length": {"1000"}})
		_ = writeFrame(ws, wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: []byte("half a ")})
		_ = ws.CloseNow()
	}()

	resp := f.do("GET", "/v/cafe/bundle.js", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// The guest must be able to tell: a truncated script that reads as
	// whole is worse than an error.
	if body, err := io.ReadAll(resp.Body); err == nil {
		t.Errorf("the guest read %q with no error, as if it were complete", body)
	}
	f.waitFor("the venue to be forgotten", func() bool { return f.srv.agentFor("cafe") == nil })

	if resp := f.do("GET", "/v/cafe/bundle.js", nil, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("after the drop: status = %d, want 503", resp.StatusCode)
	}
}

func TestNewerConnectionReplacesOlder(t *testing.T) {
	f := newFixture(t, nil)
	old := f.connect("cafe")
	first := f.srv.agentFor("cafe")

	replacement := f.connect("cafe")
	f.waitFor("the new socket to take over", func() bool {
		a := f.srv.agentFor("cafe")
		return a != nil && a != first
	})
	serve(t, replacement, func(exchange) answer { return answer{status: http.StatusOK, body: []byte("new")} })

	_, err := readFrame(old)
	if got := websocket.CloseStatus(err); got != wire.CloseReplaced {
		t.Errorf("the old socket closed with %d (%v), want %d", got, err, wire.CloseReplaced)
	}
	if body := readAll(t, f.do("GET", "/v/cafe/", nil, nil)); body != "new" {
		t.Errorf("the request went to %q, want the new socket", body)
	}
	// The old handler winding up must not take the new registration with it.
	time.Sleep(20 * time.Millisecond)
	if a := f.srv.agentFor("cafe"); a == nil || a == first {
		t.Error("the old socket's exit unregistered the new one")
	}
}

func TestGuestLeavingResetsTheStream(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", f.http.URL+"/v/cafe/slow", nil)
	if err != nil {
		t.Fatal(err)
	}
	gone := make(chan error, 1)
	go func() {
		_, err := http.DefaultClient.Do(req)
		gone <- err
	}()

	stream, _ := awaitRequest(t, ws)
	cancel()
	if err := <-gone; !errors.Is(err, context.Canceled) {
		t.Fatalf("guest: %v", err)
	}
	awaitReset(t, ws, stream)
}

func TestSlowGuestCannotStallTheVenue(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	// A guest that asks and then never reads a byte.
	conn, err := net.Dial("tcp", strings.TrimPrefix(f.http.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET /v/cafe/huge HTTP/1.1\r\nHost: relay\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	stream, _ := awaitRequest(t, ws)
	sawReset := make(chan struct{})
	go func() {
		defer close(sawReset)
		awaitReset(t, ws, stream)
	}()

	if err := writeHead(ws, stream, http.StatusOK, nil); err != nil {
		t.Fatal(err)
	}
	chunk := wire.Frame{Type: wire.TypeBody, Stream: stream, Payload: make([]byte, wire.MaxChunk)}
	// Far more than the socket buffers and the stream's queue hold between
	// them. The venue is never made to wait: the relay keeps reading and
	// cuts the one stream.
	const ceiling = 8192 // 256 MB
	sent := 0
loop:
	for ; sent < ceiling; sent++ {
		select {
		case <-sawReset:
			break loop
		default:
		}
		if err := writeFrame(ws, chunk); err != nil {
			t.Fatalf("the venue's socket stalled or closed after %d chunks: %v", sent, err)
		}
	}
	select {
	case <-sawReset:
	case <-time.After(testTimeout):
		t.Fatalf("the relay took %d chunks for a guest that reads nothing and never reset the stream", sent)
	}

	// And the venue is still serving everyone else.
	go func() {
		next, _ := awaitRequest(t, ws)
		_ = respond(ws, next, answer{status: http.StatusOK, body: []byte("still here")})
	}()
	if body := readAll(t, f.do("GET", "/v/cafe/menu", nil, nil)); body != "still here" {
		t.Errorf("after cutting the slow guest, another got %q", body)
	}
}

func TestTooManyStreams(t *testing.T) {
	f := newFixture(t, nil)
	f.connect("cafe")
	a := f.srv.agentFor("cafe")
	for range maxStreams {
		if _, err := a.open(); err != nil {
			t.Fatalf("open: %v", err)
		}
	}
	resp := f.do("GET", "/v/cafe/", nil, nil)
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("status = %d, Retry-After %q; want 503 with a wait", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	if code := errorCode(t, resp); code != codeVenueBusy {
		t.Errorf("code = %q", code)
	}
}

func TestCloseDisconnectsVenues(t *testing.T) {
	f := newFixture(t, nil)
	ws := f.connect("cafe")

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		f.srv.Close()
	}()
	_, err := readFrame(ws)
	if got := websocket.CloseStatus(err); got != websocket.StatusGoingAway {
		t.Errorf("close status = %d (%v), want going away", got, err)
	}
	// The venue has to answer the close for the handshake to finish.
	_ = ws.Close(websocket.StatusNormalClosure, "")
	select {
	case <-closed:
	case <-time.After(testTimeout):
		t.Fatal("Close did not return")
	}

	// A venue arriving after Close is turned away, and Close stays safe to
	// call again.
	late, _, err := f.dial(mustIssue(t, f.keys, "late"))
	if err == nil {
		if _, err := readFrame(late); websocket.CloseStatus(err) != websocket.StatusGoingAway {
			t.Errorf("a venue connecting after Close: %v, want going away", err)
		}
	}
	if f.srv.agentFor("late") != nil {
		t.Error("a venue registered after Close")
	}
	f.srv.Close()
}

func TestVenueFromHost(t *testing.T) {
	tests := []struct {
		host string
		want string // empty: not a venue
	}{
		{"cafe.jskin.in", "cafe"},
		{"cafe.jskin.in:443", "cafe"},
		{"CAFE.JSkin.IN.", "cafe"},
		{"blue-tokai-12.jskin.in", "blue-tokai-12"},
		{"jskin.in", ""},
		{"relay.jskin.in", ""},
		{"www.jskin.in", ""},
		{"a.b.jskin.in", ""},
		{"-x.jskin.in", ""},
		{"cafe.jskin.in.evil.example", ""},
		{"cafejskin.in", ""},
		{"evil.example", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got, ok := venueFromHost(tt.host, "jskin.in")
		if got != tt.want || ok != (tt.want != "") {
			t.Errorf("venueFromHost(%q) = %q, %v; want %q", tt.host, got, ok, tt.want)
		}
	}
}

func TestNewRefusesAVenueDomainThatIsNotOne(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, domain := range []string{"localhost", "jskin .in", "https://jskin.in", "jskin.in/x", "-a.in", "a-.in"} {
		if _, err := New(Options{Keys: mustKeys(t, testSecret), Log: log, VenueDomain: domain}); err == nil {
			t.Errorf("New accepted venue domain %q", domain)
		}
	}
	if _, err := New(Options{Keys: mustKeys(t, testSecret), Log: log, VenueDomain: " JSkin.in. "}); err != nil {
		t.Errorf("New refused a domain that only needed tidying: %v", err)
	}
}

func TestIssueRefusesAReservedName(t *testing.T) {
	keys := mustKeys(t, testSecret)
	for _, name := range []string{"relay", "www", "admin"} {
		if _, err := keys.Issue(rand.Reader, name); err == nil {
			t.Errorf("Issue made a key for %q, which would be a hostname the operator needs", name)
		}
	}
}

// TestAHostnamePerVenue: with a venue domain set, a venue is its hostname
// and the whole path is its own.
func TestAHostnamePerVenue(t *testing.T) {
	f := newFixture(t, func(o *Options) { o.VenueDomain = "jskin.in" })
	ws := f.connect("cafe")
	got := make(chan exchange, 8)
	serve(t, ws, func(ex exchange) answer {
		got <- ex
		return answer{status: http.StatusOK, body: []byte("cafe says hello"), header: http.Header{
			"Set-Cookie":      {"sid=1; Path=/t; Domain=jskin.in", "b=2"},
			"Location":        {"/t/abc"},
			"Clear-Site-Data": {`"storage"`},
		}}
	})

	as := func(host, path string) *http.Response {
		t.Helper()
		req, err := http.NewRequest("GET", f.http.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		client := &http.Client{Timeout: testTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s%s: %v", host, path, err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}

	resp := as("cafe.jskin.in", "/t/a%2Fb?seat=2")
	if body := readAll(t, resp); resp.StatusCode != http.StatusOK || body != "cafe says hello" {
		t.Fatalf("status %d, body %q", resp.StatusCode, body)
	}
	ex := <-got
	if ex.head.Path != "/t/a%2Fb?seat=2" {
		t.Errorf("path = %q, want it whole and as sent", ex.head.Path)
	}
	if ex.head.Header.Get("X-Forwarded-Host") != "cafe.jskin.in" || ex.head.Header.Get("X-Forwarded-Prefix") != "" {
		t.Errorf("forwarded headers = %v", ex.head.Header)
	}
	// Paths are the venue's own, so nothing is moved; but a cookie still
	// may not claim the whole domain, which every other venue shares.
	if cookies := resp.Header.Values("Set-Cookie"); len(cookies) != 2 || cookies[0] != "sid=1; Path=/t" || cookies[1] != "b=2; Path=/" {
		t.Errorf("Set-Cookie = %q", cookies)
	}
	if loc := resp.Header.Get("Location"); loc != "/t/abc" {
		t.Errorf("Location = %q, want it untouched", loc)
	}
	if resp.Header.Get("Clear-Site-Data") == "" {
		t.Error("a venue on its own hostname could not clear its own site data")
	}

	// Paths that mean something on the relay's own hostname are the
	// venue's on the venue's.
	for _, path := range []string{"/healthz", "/agent", "/v/bar/t/abc"} {
		if body := readAll(t, as("cafe.jskin.in", path)); body != "cafe says hello" {
			t.Errorf("cafe.jskin.in%s was answered by the relay: %q", path, body)
		}
		if ex := <-got; ex.head.Path != path {
			t.Errorf("the venue saw %q for %s", ex.head.Path, path)
		}
	}

	tests := []struct {
		name, host, path string
		want             int
	}{
		{"a venue that is not connected", "bar.jskin.in", "/t/abc", http.StatusServiceUnavailable},
		{"the relay's own hostname still answers health", "relay.jskin.in", "/healthz", http.StatusOK},
		{"a reserved name is not a venue", "www.jskin.in", "/t/abc", http.StatusNotFound},
		{"the bare domain is not a venue", "jskin.in", "/t/abc", http.StatusNotFound},
		{"a venue by path is gone: it would put every venue back on one origin", "relay.jskin.in", "/v/cafe/t/abc", http.StatusNotFound},
		{"and by path on its own hostname's sibling", "jskin.in", "/v/cafe/", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if resp := as(tt.host, tt.path); resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}

	// A venue still connects at the relay's hostname, whatever it is.
	serve(t, f.connect("bar"), func(exchange) answer { return answer{status: http.StatusOK, body: []byte("bar")} })
	if body := readAll(t, as("bar.jskin.in", "/t/abc")); body != "bar" {
		t.Errorf("a second venue at its hostname answered %q", body)
	}
}
