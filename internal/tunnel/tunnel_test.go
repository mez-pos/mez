package tunnel

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/relay"
	"github.com/byjoellobo/mez/internal/relay/wire"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef"
	testTimeout = 10 * time.Second
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testRelay is a real relay on a real port, which can be killed and brought
// back on the same address, as a relay that crashed and restarted would be.
type testRelay struct {
	t    *testing.T
	addr string
	keys *relay.Keys
	// entitled and every are the relay's entitlements and how often it asks
	// again. Nil carries every key, as most tests want.
	entitled relay.Entitlements
	every    time.Duration

	mu   sync.Mutex
	srv  *relay.Server
	http *http.Server
}

func startRelay(t *testing.T) *testRelay {
	t.Helper()
	return startRelayWith(t, nil, 0)
}

// startRelayWith is a relay that asks entitled which venues it carries, and
// asks again every so often.
func startRelayWith(t *testing.T, entitled relay.Entitlements, every time.Duration) *testRelay {
	t.Helper()
	keys, err := relay.NewKeys(testSecret, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := &testRelay{t: t, keys: keys, entitled: entitled, every: every}
	r.listen("127.0.0.1:0")
	t.Cleanup(r.kill)
	return r
}

func (r *testRelay) listen(addr string) {
	r.t.Helper()
	var ln net.Listener
	var err error
	// The port was this relay's a moment ago; the system may need a beat
	// to hand it back.
	for deadline := time.Now().Add(testTimeout); ; {
		if ln, err = net.Listen("tcp", addr); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		r.t.Fatalf("relay could not listen on %s: %v", addr, err)
	}
	srv, err := relay.New(relay.Options{Keys: r.keys, Log: quiet(), Entitlements: r.entitled, EntitlementEvery: r.every})
	if err != nil {
		r.t.Fatal(err)
	}
	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() { _ = httpSrv.Serve(ln) }()

	r.mu.Lock()
	r.addr, r.srv, r.http = ln.Addr().String(), srv, httpSrv
	r.mu.Unlock()
}

// kill stops the relay the way a crash does: every socket cut, no goodbye.
func (r *testRelay) kill() {
	r.mu.Lock()
	srv, httpSrv := r.srv, r.http
	r.srv, r.http = nil, nil
	r.mu.Unlock()
	if httpSrv == nil {
		return
	}
	_ = httpSrv.Close()
	srv.Close()
}

func (r *testRelay) restart() { r.listen(r.addr) }

func (r *testRelay) key(venueID string) string {
	r.t.Helper()
	return r.keyFor(venueID, false)
}

// remoteKey is a key the relay carries /backoffice/ and /api/ for (ADR-306),
// so a test of what the tunnel itself lets through is not answered by the
// relay first.
func (r *testRelay) remoteKey(venueID string) string {
	r.t.Helper()
	return r.keyFor(venueID, true)
}

func (r *testRelay) keyFor(venueID string, remote bool) string {
	r.t.Helper()
	key, err := r.keys.Issue(rand.Reader, venueID, remote)
	if err != nil {
		r.t.Fatal(err)
	}
	return key
}

// guest makes a request the way a phone does, through the relay.
func (r *testRelay) guest(method, venuePath string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://"+r.addr+"/v/cafe"+venuePath, body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: testTimeout}
	return client.Do(req)
}

func (r *testRelay) mustGuest(method, venuePath string, body io.Reader) (int, string) {
	r.t.Helper()
	resp, err := r.guest(method, venuePath, body)
	if err != nil {
		r.t.Fatalf("guest %s %s: %v", method, venuePath, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		r.t.Fatalf("guest reading %s: %v", venuePath, err)
	}
	return resp.StatusCode, string(b)
}

// venue starts a tunnel client for "cafe" in front of handler, with waits
// short enough for a test.
func (r *testRelay) venue(handler http.Handler, tune func(*Options)) *Client {
	r.t.Helper()
	opts := Options{
		RelayURL:    "http://" + r.addr,
		Key:         r.key("cafe"),
		Handler:     handler,
		Mount:       "/order/",
		Log:         quiet(),
		MinBackoff:  5 * time.Millisecond,
		MaxBackoff:  20 * time.Millisecond,
		LongBackoff: time.Hour,
	}
	if tune != nil {
		tune(&opts)
	}
	c, err := New(opts)
	if err != nil {
		r.t.Fatalf("New: %v", err)
	}
	c.Start()
	r.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()
		_ = c.Close(ctx)
	})
	return c
}

func waitState(t *testing.T, c *Client, want State) Status {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		s := c.Status()
		if s.State == want {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("tunnel is %q (%s), want %q", s.State, s.LastError, want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// server is a stand-in for mez: a guest app under /order/, and a staff API
// beside it that no guest must ever reach.
func server() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/order/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Saw-Remote", r.RemoteAddr)
		w.Header().Set("X-Saw-Prefix", r.Header.Get("X-Forwarded-Prefix"))
		fmt.Fprintf(w, "guest app: %s %s %s", r.Method, r.URL.RequestURI(), body)
	})
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "STAFF ONLY")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "THE TERMINAL")
	})
	return mux
}

func TestNewRefuses(t *testing.T) {
	good := Options{RelayURL: "wss://relay.example", Key: "mzr_cafe.0123456789ab.sig", Handler: server(), Mount: "/order/", Log: quiet()}
	tests := []struct {
		name string
		tune func(*Options)
		want string
	}{
		{"no handler", func(o *Options) { o.Handler = nil }, "required"},
		{"no log", func(o *Options) { o.Log = nil }, "required"},
		{"no key", func(o *Options) { o.Key = "" }, "relay_key is not"},
		{"something that is not a venue key", func(o *Options) { o.Key = "hunter2-hunter2" }, "does not look like"},
		{"the back office on a guests-only key", func(o *Options) { o.BackOffice = true }, "guests-only key"},
		{"the root as the mount, which would be everything", func(o *Options) { o.Mount = "/" }, "mount"},
		{"a mount that is not a directory", func(o *Options) { o.Mount = "/order" }, "mount"},
		{"a relay url with no host", func(o *Options) { o.RelayURL = "wss://" }, "host"},
		{"a relay url that cannot be dialled", func(o *Options) { o.RelayURL = "ftp://relay.example" }, "scheme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := good
			tt.tune(&opts)
			_, err := New(opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("New = %v, want an error mentioning %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "hunter2") {
				t.Error("the error repeats what was given as the key")
			}
		})
	}
	if _, err := New(good); err != nil {
		t.Errorf("New refused good options: %v", err)
	}
}

func TestAgentURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"wss://relay.example.com", "wss://relay.example.com/agent"},
		{"wss://relay.example.com/", "wss://relay.example.com/agent"},
		{"https://relay.example.com/v1", "https://relay.example.com/v1/agent"},
		{"http://127.0.0.1:8080/v1/?x=1#frag", "http://127.0.0.1:8080/v1/agent"},
		{"ws://localhost:8080", "ws://localhost:8080/agent"},
		{"ws://[::1]:8080", "ws://[::1]:8080/agent"},
	}
	for _, tt := range tests {
		if got, err := agentURL(tt.in); err != nil || got != tt.want {
			t.Errorf("agentURL(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

// TestAgentURLRefusesClearTextOffTheMachine: the venue's key goes out on the
// dial as a bearer token, so a relay anywhere but this machine must be
// reached over TLS.
func TestAgentURLRefusesClearTextOffTheMachine(t *testing.T) {
	for _, in := range []string{"ws://relay.example.com", "http://relay.example.com:8080", "ws://192.168.1.5", "ftp://relay.example.com", "relay.example.com"} {
		if got, err := agentURL(in); err == nil {
			t.Errorf("agentURL(%q) = %q, want a refusal", in, got)
		}
	}
}

func TestBackoff(t *testing.T) {
	const low, high = time.Second, time.Minute
	tests := []struct {
		name     string
		failures int
		jitter   float64
		want     time.Duration
	}{
		{"first failure, least", 0, 0, 500 * time.Millisecond},
		{"first failure, most", 0, 0.999999, 999999500 * time.Nanosecond},
		{"it doubles", 1, 0, time.Second},
		{"and doubles", 3, 0, 4 * time.Second},
		{"up to the ceiling", 6, 0, 30 * time.Second},
		{"and stays there", 7, 0, 30 * time.Second},
		{"a week of failures does not overflow", 5000, 0, 30 * time.Second},
		{"never past the ceiling", 5000, 0.999999, 59999970 * time.Microsecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backoff(tt.failures, low, high, tt.jitter); got != tt.want {
				t.Errorf("backoff = %v, want %v", got, tt.want)
			}
		})
	}
	for failures := range 100 {
		if got := backoff(failures, low, high, 0.5); got < low/2 || got > high {
			t.Fatalf("backoff(%d) = %v, outside %v to %v", failures, got, low/2, high)
		}
	}
}

func TestGuestRequestStaysUnderTheMount(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantPath string // empty: refused
		wantURI  string
	}{
		{"a table", "/t/abc", "/order/t/abc", "/order/t/abc"},
		{"the root", "/", "/order/", "/order/"},
		{"a query is kept", "/t/abc?seat=2", "/order/t/abc", "/order/t/abc?seat=2"},
		{"an asset", "/assets/app-1a2b.js", "/order/assets/app-1a2b.js", "/order/assets/app-1a2b.js"},
		{"the staff API's path is just a path under the mount", "/api/v1/orders", "/order/api/v1/orders", "/order/api/v1/orders"},
		{"an encoded space survives", "/t/a%20b", "/order/t/a b", "/order/t/a%20b"},
		{"already under the mount, as the app names its files", "/order/assets/app-1a2b.js", "/order/assets/app-1a2b.js", "/order/assets/app-1a2b.js"},
		{"already under the mount, the guest API", "/order/api/t/abc/menu?x=1", "/order/api/t/abc/menu", "/order/api/t/abc/menu?x=1"},
		{"already under the mount, encoded", "/order/t/a%20b", "/order/t/a b", "/order/t/a%20b"},
		{"the mount's name without its slash is a path like any other", "/orders", "/order/orders", "/order/orders"},
		{"the mount itself with no slash", "/order", "/order/order", "/order/order"},
		{"under the mount and climbing out of it", "/order/../api/v1/orders", "", ""},
		{"climbing out", "/../api/v1/orders", "", ""},
		{"climbing out further in", "/t/../../api/v1/orders", "", ""},
		{"climbing out, encoded", "/%2e%2e/api/v1/orders", "", ""},
		{"climbing out behind an encoded slash", "/t%2f..%2f..%2fapi/v1/orders", "", ""},
		{"an encoded slash that would route past the mount", "/order%2Fx", "", ""},
		{"an encoded slash, lower case", "/order%2fapi/v1/orders", "", ""},
		{"an encoded backslash", "/order%5Cx", "", ""},
		{"a dot segment", "/./t/abc", "", ""},
		{"an empty segment", "//api/v1/orders", "", ""},
		{"a backslash", "/t\\..\\api", "", ""},
		{"a full url", "http://evil.example/t/abc", "", ""},
		{"not rooted", "t/abc", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := guestRequest(context.Background(), wire.RequestHead{Method: "GET", Path: tt.path}, nil, "/order/", false)
			if tt.wantPath == "" {
				if err == nil {
					t.Fatalf("accepted, as %q", req.URL.Path)
				}
				// A path that is refused is refused with the back office on
				// too: the checks come before the prefixes (ADR-306).
				if req, err := guestRequest(context.Background(), wire.RequestHead{Method: "GET", Path: tt.path}, nil, "/order/", true); err == nil {
					t.Fatalf("accepted with the back office on, as %q", req.URL.Path)
				}
				return
			}
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if req.URL.Path != tt.wantPath || req.RequestURI != tt.wantURI {
				t.Errorf("path %q, uri %q; want %q, %q", req.URL.Path, req.RequestURI, tt.wantPath, tt.wantURI)
			}
		})
	}
}

func TestGuestsReachTheGuestAppAndNothingElse(t *testing.T) {
	r := startRelay(t)
	// On a remote key, so that the relay forwards the staff paths and it is
	// the tunnel's own mount rule that is proven here. On a plain key the
	// relay answers 404 before the tunnel sees them (ADR-306).
	c := r.venue(server(), func(o *Options) { o.Key = r.remoteKey("cafe") })
	waitState(t, c, StateConnected)

	resp, err := r.guest("POST", "/t/abc?seat=2", strings.NewReader("one dosa"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "guest app: POST /order/t/abc?seat=2 one dosa" {
		t.Errorf("status %d, body %q", resp.StatusCode, body)
	}
	if got := resp.Header.Get("X-Saw-Prefix"); got != "/v/cafe" {
		t.Errorf("the server saw prefix %q", got)
	}
	if got := resp.Header.Get("X-Saw-Remote"); got != "127.0.0.1:0" {
		t.Errorf("the server saw remote %q, want the guest's address as the relay reported it", got)
	}

	// Everything a guest can spell lands under the mount or nowhere.
	for _, path := range []string{"/api/v1/orders", "/api/v1/sessions", "/backoffice/", "/"} {
		if _, body := r.mustGuest("GET", path, nil); !strings.HasPrefix(body, "guest app: GET /order") {
			t.Errorf("guest asking for %s got %q", path, body)
		}
	}
	// Raw, because a well-behaved client tidies these before sending them.
	for _, path := range []string{"/../api/v1/orders", "/%2e%2e/api/v1/orders", "/t/..%2f..%2fapi/v1/orders", "//api/v1/orders", "/order%2Fx", "/order%2fapi/v1/orders"} {
		status, body := rawGuest(t, r.addr, "/v/cafe"+path)
		if strings.Contains(body, "STAFF") || strings.Contains(body, "TERMINAL") {
			t.Errorf("guest asking for %s got out from under the mount: %d %q", path, status, body)
		}
	}
}

// office is a stand-in for mez that says where each request landed and
// whether it was marked as relayed: the guest app, the back office's bundle
// and API, and the terminal at the root, which nothing relayed may reach.
func office() http.Handler {
	say := func(where string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Saw-Via", strings.Join(r.Header.Values(HeaderVia), ","))
			fmt.Fprintf(w, "%s: %s", where, r.URL.RequestURI())
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/order/", say("guest app"))
	mux.HandleFunc("/backoffice/", say("back office"))
	mux.HandleFunc("/api/v1/", say("staff api"))
	mux.HandleFunc("/kitchen", say("THE KITCHEN"))
	mux.HandleFunc("/", say("THE TERMINAL"))
	return mux
}

// TestTheBackOfficeCrossesOnlyWhenTheVenueSaysSo: with remote_backoffice on,
// the bundle and the API are passed as they are and everything else is still
// put under the guest app's mount; with it off nothing is, on the same key
// (ADR-306). Either way every relayed request carries the tunnel's mark.
func TestTheBackOfficeCrossesOnlyWhenTheVenueSaysSo(t *testing.T) {
	paths := []struct {
		path    string
		on, off string // where it lands
	}{
		{"/backoffice/", "back office: /backoffice/", "guest app: /order/backoffice/"},
		{"/backoffice/menu?tab=items", "back office: /backoffice/menu?tab=items", "guest app: /order/backoffice/menu?tab=items"},
		{"/backoffice/assets/app.js", "back office: /backoffice/assets/app.js", "guest app: /order/backoffice/assets/app.js"},
		{"/api/v1/menu", "staff api: /api/v1/menu", "guest app: /order/api/v1/menu"},
		{"/api/v1/devices/pair", "staff api: /api/v1/devices/pair", "guest app: /order/api/v1/devices/pair"},
		// The guest app is where it always was.
		{"/t/abc", "guest app: /order/t/abc", "guest app: /order/t/abc"},
		{"/order/api/v1/menu", "guest app: /order/api/v1/menu", "guest app: /order/api/v1/menu"},
		// And the rest of the server is still nowhere a relayed path leads.
		{"/", "guest app: /order/", "guest app: /order/"},
		{"/kitchen", "guest app: /order/kitchen", "guest app: /order/kitchen"},
		{"/pos", "guest app: /order/pos", "guest app: /order/pos"},
		{"/backofficer/", "guest app: /order/backofficer/", "guest app: /order/backofficer/"},
		{"/Backoffice/", "guest app: /order/Backoffice/", "guest app: /order/Backoffice/"},
		{"/api/v10/menu", "guest app: /order/api/v10/menu", "guest app: /order/api/v10/menu"},
		{"/api/health", "guest app: /order/api/health", "guest app: /order/api/health"},
	}
	for _, mode := range []struct {
		name string
		on   bool
	}{{"on", true}, {"off", false}} {
		t.Run(mode.name, func(t *testing.T) {
			r := startRelay(t)
			c := r.venue(office(), func(o *Options) {
				o.Key = r.remoteKey("cafe")
				o.BackOffice = mode.on
			})
			waitState(t, c, StateConnected)
			if c.BackOffice() != mode.on {
				t.Errorf("BackOffice() = %v", c.BackOffice())
			}

			for _, tt := range paths {
				want := tt.off
				if mode.on {
					want = tt.on
				}
				resp, err := r.guest("GET", tt.path, nil)
				if err != nil {
					t.Fatalf("GET %s: %v", tt.path, err)
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if string(body) != want {
					t.Errorf("GET %s landed at %q, want %q", tt.path, body, want)
				}
				if via := resp.Header.Get("X-Saw-Via"); via != ViaRelay {
					t.Errorf("GET %s: the server saw %s %q, want exactly %q", tt.path, HeaderVia, via, ViaRelay)
				}
			}

			// Raw, because a well-behaved client tidies these before sending
			// them. None may come out anywhere but under the mount or nowhere.
			for _, path := range []string{
				"/backoffice/../", "/backoffice/../kitchen", "/api/v1/../../kitchen", "/backoffice/%2e%2e/kitchen",
				"/backoffice%2F..%2Fkitchen", "/api/v1/..%2f..%2fkitchen", "/backoffice//kitchen", "//kitchen", "/api%2Fv1%2Fmenu",
			} {
				status, body := rawGuest(t, r.addr, "/v/cafe"+path)
				if strings.Contains(body, "TERMINAL") || strings.Contains(body, "KITCHEN") {
					t.Errorf("%s reached the room: %d %q", path, status, body)
				}
			}
		})
	}
}

// TestTheMarkIsTheTunnelsOwn: whatever arrives claiming to be, or not to be,
// from the relay is replaced. A guest cannot unmark a request, and cannot
// mark it as something else.
func TestTheMarkIsTheTunnelsOwn(t *testing.T) {
	for _, sent := range []http.Header{
		nil,
		{"X-Mez-Via": {"lan"}},
		{"x-mez-via": {"lan"}, "X-MEZ-VIA": {"", "relay, lan"}},
		{"X-Mez-Via": {}},
	} {
		req, err := guestRequest(context.Background(), wire.RequestHead{Method: "GET", Path: "/api/v1/menu", Header: sent}, nil, "/order/", true)
		if err != nil {
			t.Fatalf("guestRequest: %v", err)
		}
		var marks []string
		for name, values := range req.Header {
			if strings.EqualFold(name, HeaderVia) {
				marks = append(marks, values...)
			}
		}
		if len(marks) != 1 || marks[0] != ViaRelay || req.Header.Get(HeaderVia) != ViaRelay {
			t.Errorf("sent %v: the request carries %q", sent, marks)
		}
	}
}

func TestKeyAllowsBackOffice(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"mzr_cafe.0123456789ab.remote.sig", true},
		{"mzr_cafe.0123456789ab.sig", false},
		{"mzr_cafe.0123456789ab.admin.sig", false},
		{"mzr_cafe.remote.0123456789ab.sig", false},
		{"mzr_remote.0123456789ab.sig", false},
		{"mzr_cafe.0123456789ab.remote.sig.more", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := keyAllowsBackOffice(tt.key); got != tt.want {
			t.Errorf("keyAllowsBackOffice(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

// rawGuest sends a request line exactly as written.
func rawGuest(t *testing.T, addr, target string) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(testTimeout))
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: relay\r\nConnection: close\r\n\r\n", target); err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(conn)
	var status int
	_, _ = fmt.Sscanf(string(raw), "HTTP/1.1 %d", &status)
	return status, string(raw)
}

func TestLargeAndStreamedResponses(t *testing.T) {
	r := startRelay(t)
	bundle := bytes.Repeat([]byte("bundle"), 50001) // several frames, not a whole number of them
	firstRead := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/order/bundle.js", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bundle) })
	mux.HandleFunc("/order/events", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "accepted\n")
		w.(http.Flusher).Flush()
		// The second event waits for the guest to have the first.
		<-firstRead
		_, _ = io.WriteString(w, "ready\n")
	})
	mux.HandleFunc("/order/empty", func(http.ResponseWriter, *http.Request) {})
	c := r.venue(mux, nil)
	waitState(t, c, StateConnected)

	if status, body := r.mustGuest("GET", "/bundle.js", nil); status != 200 || body != string(bundle) {
		t.Errorf("bundle: status %d, %d bytes, want %d intact", status, len(body), len(bundle))
	}
	if status, body := r.mustGuest("GET", "/empty", nil); status != 200 || body != "" {
		t.Errorf("a handler that writes nothing: status %d, body %q", status, body)
	}

	resp, err := r.guest("GET", "/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	first := make([]byte, len("accepted\n"))
	if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "accepted\n" {
		t.Fatalf("first event = %q, %v", first, err)
	}
	close(firstRead)
	if rest, err := io.ReadAll(resp.Body); err != nil || string(rest) != "ready\n" {
		t.Errorf("rest = %q, %v", rest, err)
	}
}

func TestAGuestLeavingCancelsTheRequest(t *testing.T) {
	r := startRelay(t)
	started, cancelled := make(chan struct{}), make(chan struct{})
	c := r.venue(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		close(started)
		<-req.Context().Done()
		close(cancelled)
	}), nil)
	waitState(t, c, StateConnected)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+r.addr+"/v/cafe/slow", nil)
	go func() { _, _ = http.DefaultClient.Do(req) }()
	<-started
	cancel()
	select {
	case <-cancelled:
	case <-time.After(testTimeout):
		t.Fatal("the server went on working for a guest who had left")
	}
}

func TestAPanicInAGuestRequestIsOneGuestsProblem(t *testing.T) {
	r := startRelay(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/order/boom", func(http.ResponseWriter, *http.Request) { panic("a nil map, somewhere") })
	mux.HandleFunc("/order/", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "fine") })
	c := r.venue(mux, nil)
	waitState(t, c, StateConnected)

	if status, _ := r.mustGuest("GET", "/boom", nil); status != http.StatusBadGateway {
		t.Errorf("the guest whose request panicked got %d, want 502", status)
	}
	if status, body := r.mustGuest("GET", "/menu", nil); status != 200 || body != "fine" {
		t.Errorf("the next guest got %d %q", status, body)
	}
	if got := c.Status().State; got != StateConnected {
		t.Errorf("the tunnel is %q after a panic, want still connected", got)
	}
}

// TestTheRelayDiesAndComesBack is the tunnel's own half of the phase's
// requirement: a relay killed mid-service is noticed, waited for with
// backoff, and rejoined without anyone touching the venue's machine.
func TestTheRelayDiesAndComesBack(t *testing.T) {
	r := startRelay(t)
	c := r.venue(server(), nil)
	waitState(t, c, StateConnected)
	if status, _ := r.mustGuest("GET", "/t/abc", nil); status != 200 {
		t.Fatalf("before: status %d", status)
	}

	r.kill()
	s := waitState(t, c, StateWaiting)
	if s.LastError == "" || s.RetryAt.IsZero() {
		t.Errorf("waiting, but with no reason or no next attempt: %+v", s)
	}
	// It keeps trying, and keeps failing, without giving up or spinning.
	time.Sleep(100 * time.Millisecond)
	if got := c.Status().State; got != StateWaiting && got != StateConnecting {
		t.Errorf("with the relay down the tunnel is %q", got)
	}

	r.restart()
	waitState(t, c, StateConnected)
	if status, body := r.mustGuest("GET", "/t/abc", nil); status != 200 || !strings.HasPrefix(body, "guest app") {
		t.Errorf("after the relay came back: %d %q", status, body)
	}
}

func TestStartDoesNotWaitForARelayThatIsNotThere(t *testing.T) {
	// A port nothing is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	c, err := New(Options{RelayURL: "http://" + addr, Key: "mzr_cafe.0123456789ab.sig", Handler: server(), Mount: "/order/", Log: quiet(), MinBackoff: 5 * time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	c.Start()
	if took := time.Since(began); took > time.Second {
		t.Errorf("Start took %v; the counter would have opened that much later", took)
	}
	waitState(t, c, StateWaiting)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if err := c.Close(ctx); err != nil {
		t.Errorf("Close with nothing connected: %v", err)
	}
	if got := c.Status().State; got != StateStopped {
		t.Errorf("after Close the tunnel is %q", got)
	}
	// Closed is closed.
	c.Start()
	time.Sleep(20 * time.Millisecond)
	if got := c.Status().State; got != StateStopped {
		t.Errorf("Start after Close made it %q", got)
	}
}

func TestARefusedKeyIsNotHammered(t *testing.T) {
	r := startRelay(t)
	other, err := relay.NewKeys(strings.Repeat("z", relay.MinSecretLen), nil)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := other.Issue(rand.Reader, "cafe", false)
	if err != nil {
		t.Fatal(err)
	}
	c := r.venue(server(), func(o *Options) { o.Key = wrong })

	s := waitState(t, c, StateRefused)
	if wait := s.RetryAt.Sub(s.Since); wait < 30*time.Minute {
		t.Errorf("a refused key is retried in %v, want the long wait", wait)
	}
	if strings.Contains(s.LastError, wrong) {
		t.Error("the status repeats the key")
	}
}

func TestAnotherMachineWithTheSameKey(t *testing.T) {
	r := startRelay(t)
	first := r.venue(server(), nil)
	waitState(t, first, StateConnected)

	second := r.venue(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "the second machine")
	}), nil)
	waitState(t, second, StateConnected)

	// The first stands down for a long time and does not take the place back.
	s := waitState(t, first, StateReplaced)
	if wait := s.RetryAt.Sub(s.Since); wait < 30*time.Minute {
		t.Errorf("a replaced venue comes back in %v, want the long wait", wait)
	}
	time.Sleep(50 * time.Millisecond)
	if got := second.Status().State; got != StateConnected {
		t.Errorf("the second machine is %q; the two are fighting", got)
	}
	if _, body := r.mustGuest("GET", "/", nil); body != "the second machine" {
		t.Errorf("guests reach %q", body)
	}
}

func TestCloseSaysGoodbyeAndStopsServing(t *testing.T) {
	r := startRelay(t)
	c := r.venue(server(), nil)
	waitState(t, c, StateConnected)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if err := c.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := c.Status().State; got != StateStopped {
		t.Errorf("after Close the tunnel is %q", got)
	}
	// The relay knows at once, and tells guests so.
	deadline := time.Now().Add(testTimeout)
	for {
		status, _ := r.mustGuest("GET", "/t/abc", nil)
		if status == http.StatusServiceUnavailable {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after Close guests still get %d", status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHeartbeatKeepsAQuietTunnelOpen(t *testing.T) {
	r := startRelay(t)
	c := r.venue(server(), func(o *Options) { o.Heartbeat = 5 * time.Millisecond })
	first := waitState(t, c, StateConnected)

	time.Sleep(100 * time.Millisecond) // twenty heartbeats
	now := c.Status()
	if now.State != StateConnected || !now.Since.Equal(first.Since) {
		t.Errorf("a quiet tunnel reconnected or dropped: %+v", now)
	}
	if status, _ := r.mustGuest("GET", "/t/abc", nil); status != 200 {
		t.Errorf("after the quiet: status %d", status)
	}
}
