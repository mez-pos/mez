package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/byjoellobo/mez/internal/relay"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	"github.com/byjoellobo/mez/internal/tunnel"
)

// The relay and the counter, on the real stack (phase 11, Task 2, ADR-202):
// the database, the order service and the API behind a tunnel client, and a
// real relay on a real port that a guest's request goes through. The relay
// is then killed while the venue is trading.

// liveRelay is a relay that can be killed and brought back on its address.
type liveRelay struct {
	t    *testing.T
	addr string
	keys *relay.Keys
	srv  *relay.Server
	http *http.Server
}

func startLiveRelay(t *testing.T) *liveRelay {
	t.Helper()
	keys, err := relay.NewKeys("0123456789abcdef0123456789abcdef", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := &liveRelay{t: t, keys: keys}
	r.listen("127.0.0.1:0")
	t.Cleanup(r.kill)
	return r
}

func (r *liveRelay) listen(addr string) {
	r.t.Helper()
	var ln net.Listener
	var err error
	// On a restart the port was this relay's a moment ago, and the system
	// may need a beat to hand it back.
	for deadline := time.Now().Add(10 * time.Second); ; {
		if ln, err = net.Listen("tcp", addr); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		r.t.Fatalf("relay could not listen on %s: %v", addr, err)
	}
	srv, err := relay.New(relay.Options{Keys: r.keys, Log: quietLogger()})
	if err != nil {
		r.t.Fatal(err)
	}
	r.addr, r.srv, r.http = ln.Addr().String(), srv, &http.Server{Handler: srv.Handler()}
	go func(s *http.Server) { _ = s.Serve(ln) }(r.http)
}

// kill stops the relay the way a crash does: every socket cut, no goodbye.
func (r *liveRelay) kill() {
	if r.http == nil {
		return
	}
	_ = r.http.Close()
	r.srv.Close()
	r.srv, r.http = nil, nil
}

// guest asks for a table's page as a phone on mobile data would.
func (r *liveRelay) guest() (int, string) {
	r.t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+r.addr+"/v/cafe/t/4", nil)
	if err != nil {
		r.t.Fatal(err)
	}
	req.Header.Set("Accept", "text/html")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		// The relay is not there at all.
		return 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// sell rings up one coffee, takes cash for it, closes the order and returns
// its invoice number: a whole sale, as the counter makes one.
func (r *register) sell(t *testing.T) string {
	t.Helper()
	id := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":2,"name":"Filter coffee","qty":1,"unit_price":{"minor":4000,"currency":"INR"},"station":"bar","tax_rate":5000,"hsn":"996331"}`)
	paid := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments",
		`{"method":"cash","amount":{"minor":4200,"currency":"INR"}}`)
	if !paid.Order.Totals.BalanceDue.IsZero() || len(paid.Order.Invoices) != 1 {
		t.Fatalf("the sale did not settle: %+v, invoices %+v", paid.Order.Totals, paid.Order.Invoices)
	}
	closed := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/close", `{}`)
	if closed.Order.Status != "closed" {
		t.Fatalf("the order did not close: %+v", closed.Order)
	}
	return closed.Order.Invoices[0].InvoiceNo
}

func (r *register) tunnelHealth(t *testing.T) string {
	t.Helper()
	rec := send(r.handler, http.MethodGet, "/api/v1/health", "", nil)
	var body healthResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("health: %d %s", rec.Code, rec.Body)
	}
	return body.Checks.Tunnel
}

func (r *register) awaitTunnelHealth(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for r.tunnelHealth(t) != want {
		if time.Now().After(deadline) {
			t.Fatalf("health says the tunnel is %q, want %q", r.tunnelHealth(t), want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestTunnelHealthIsUnknownWithNoRelay(t *testing.T) {
	// Off is not failing: most venues never set a relay, and their health
	// must not carry a permanent fault.
	r := newRegister(t)
	if got := r.tunnelHealth(t); got != CheckUnknown {
		t.Errorf("tunnel = %q, want %q", got, CheckUnknown)
	}
}

// TestTheCounterSellsWhileTheRelayIsDown is the task's proof. A guest
// reaches the venue through the relay; the relay is killed; the counter
// goes on selling, on the same invoice series, as if nothing had happened;
// the relay comes back and guests are served again with nobody having
// touched the venue's machine.
func TestTheCounterSellsWhileTheRelayIsDown(t *testing.T) {
	r := newRegister(t)
	orders, err := svcorder.New(svcorder.Options{DB: r.db})
	if err != nil {
		t.Fatal(err)
	}

	// The server reads the tunnel through this, as cmd/mez wires it.
	var guests *tunnel.Client
	guestApp := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	r.rebuild(t,
		WithOrders(orders),
		WithStatic(map[string]fs.FS{"/order/": guestApp}),
		WithTunnel(func() TunnelStatus {
			st := guests.Status()
			state := TunnelWaiting
			if st.State == tunnel.StateConnected {
				state = TunnelConnected
			}
			return TunnelStatus{State: state, Since: st.Since, RetryAt: st.RetryAt, Detail: st.LastError}
		}),
	)

	rl := startLiveRelay(t)
	key, err := rl.keys.Issue(rand.Reader, "cafe")
	if err != nil {
		t.Fatal(err)
	}
	guests, err = tunnel.New(tunnel.Options{
		RelayURL:   "http://" + rl.addr,
		Key:        key,
		Handler:    r.handler,
		Mount:      "/order/",
		Log:        quietLogger(),
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	guests.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = guests.Close(ctx)
	})

	// Trading, with the relay up.
	r.awaitTunnelHealth(t, CheckOK)
	if status, body := rl.guest(); status != http.StatusOK || !strings.Contains(body, "the guest app") {
		t.Fatalf("a guest through the relay: %d %q", status, body)
	}
	if inv := r.sell(t); inv != "INV/26-27/000001" {
		t.Fatalf("first sale: invoice %q", inv)
	}

	// The relay dies.
	rl.kill()
	r.awaitTunnelHealth(t, CheckFailing)
	if status, _ := rl.guest(); status == http.StatusOK {
		t.Fatal("a guest was served with the relay dead")
	}

	// The counter does not notice. Three sales, each prompt, each the next
	// number in the same series, while the tunnel is busy failing to dial.
	for i, want := range []string{"INV/26-27/000002", "INV/26-27/000003", "INV/26-27/000004"} {
		began := time.Now()
		inv := r.sell(t)
		if inv != want {
			t.Fatalf("sale %d with the relay down: invoice %q, want %q", i+1, inv, want)
		}
		if took := time.Since(began); took > 2*time.Second {
			t.Errorf("sale %d with the relay down took %v; something at the counter waited on the tunnel", i+1, took)
		}
	}
	if got := r.tunnelHealth(t); got != CheckFailing {
		t.Errorf("health says the tunnel is %q while the relay is dead", got)
	}
	// Health as a whole still says the process is serving.
	if rec := send(r.handler, http.MethodGet, "/api/v1/health", "", nil); !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("health with the relay down: %s", rec.Body)
	}

	// The relay comes back, and the venue rejoins it unprompted.
	rl.listen(rl.addr)
	r.awaitTunnelHealth(t, CheckOK)
	if status, body := rl.guest(); status != http.StatusOK || !strings.Contains(body, "the guest app") {
		t.Errorf("a guest after the relay came back: %d %q", status, body)
	}
	if inv := r.sell(t); inv != "INV/26-27/000005" {
		t.Errorf("a sale after the relay came back: invoice %q", inv)
	}

	// And through all of it a guest never reached the staff API.
	req, _ := http.NewRequest(http.MethodGet, "http://"+rl.addr+"/v/cafe/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+r.deviceToken)
	req.Header.Set(HeaderStaffToken, r.staffToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `"orders"`) {
		t.Errorf("a guest holding staff credentials read the open orders through the relay: %s", body)
	}
}

func (r *register) tunnel(t *testing.T, owner map[string]string) (int, tunnelResponse) {
	t.Helper()
	rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
	var body tunnelResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("GET /tunnel: %v in %s", err, rec.Body)
		}
	}
	return rec.Code, body
}

// TestTunnelForTheBackOffice: what the Devices screen reads (phase 11,
// Task 2b). More than health's one word: the state, since when, when it
// tries next, and why it failed.
func TestTunnelForTheBackOffice(t *testing.T) {
	since := time.Date(2026, 10, 5, 12, 0, 0, 0, time.FixedZone("IST", 19800))
	retry := since.Add(40 * time.Second)

	tests := []struct {
		name       string
		status     *TunnelStatus // nil: no relay configured
		want       tunnelResponse
		wantHealth string
	}{
		{"no relay configured is off, and nothing more", nil,
			tunnelResponse{State: TunnelOff}, CheckUnknown},
		{"off as reported carries nothing either", &TunnelStatus{State: TunnelOff, Since: since, Detail: "x"},
			tunnelResponse{State: TunnelOff}, CheckUnknown},
		{"connected says since when, and no old error", &TunnelStatus{State: TunnelConnected, Since: since, Detail: "the failure before"},
			tunnelResponse{State: TunnelConnected, Since: utc(since)}, CheckOK},
		{"waiting says why and when next", &TunnelStatus{State: TunnelWaiting, Since: since, RetryAt: retry, Detail: "dialling the relay: no such host"},
			tunnelResponse{State: TunnelWaiting, Since: utc(since), RetryAt: utc(retry), Detail: "dialling the relay: no such host"}, CheckFailing},
		{"refused", &TunnelStatus{State: TunnelRefused, Since: since, RetryAt: retry, Detail: "the relay refused this venue's key"},
			tunnelResponse{State: TunnelRefused, Since: utc(since), RetryAt: utc(retry), Detail: "the relay refused this venue's key"}, CheckFailing},
		{"replaced", &TunnelStatus{State: TunnelReplaced, Since: since},
			tunnelResponse{State: TunnelReplaced, Since: utc(since)}, CheckFailing},
		{"connecting", &TunnelStatus{State: TunnelConnecting, Since: since},
			tunnelResponse{State: TunnelConnecting, Since: utc(since)}, CheckFailing},
		{"a word outside the vocabulary is stopped", &TunnelStatus{State: "exploded", Since: since},
			tunnelResponse{State: TunnelStopped, Since: utc(since)}, CheckFailing},
	}
	r := newRegister(t)
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.status != nil {
				opts = append(opts, WithTunnel(func() TunnelStatus { return *tt.status }))
			}
			r.rebuild(t, opts...)
			status, got := r.tunnel(t, owner)
			if status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}
			if got.State != tt.want.State || got.Detail != tt.want.Detail ||
				!sameTime(got.Since, tt.want.Since) || !sameTime(got.RetryAt, tt.want.RetryAt) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if h := r.tunnelHealth(t); h != tt.wantHealth {
				t.Errorf("health says %q, want %q", h, tt.wantHealth)
			}
		})
	}

	t.Run("times are UTC on the wire", func(t *testing.T) {
		r.rebuild(t, WithTunnel(func() TunnelStatus { return TunnelStatus{State: TunnelConnected, Since: since} }))
		rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
		if !strings.Contains(rec.Body.String(), `"since":"2026-10-05T06:30:00Z"`) {
			t.Errorf("body = %s", rec.Body)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Error("the tunnel's state may be cached")
		}
	})
}

func utc(at time.Time) *time.Time {
	u := at.UTC()
	return &u
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// TestTunnelIsForWhoeverManagesDevices: it says how the venue reaches the
// outside, so it is not for a device with nobody signed in, nor for a role
// that cannot manage devices.
func TestTunnelIsForWhoeverManagesDevices(t *testing.T) {
	r := newRegister(t)
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
	tests := []struct {
		name   string
		method string
		header map[string]string
		want   int
	}{
		{"no device", http.MethodGet, nil, http.StatusUnauthorized},
		{"a device with nobody signed in", http.MethodGet, r.asDevice(), http.StatusUnauthorized},
		{"a cashier, who does not manage devices", http.MethodGet, r.asStaff(r.staffToken), http.StatusForbidden},
		{"the owner", http.MethodGet, owner, http.StatusOK},
		{"it is read, not written", http.MethodPost, owner, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(r.handler, tt.method, "/api/v1/tunnel", "", tt.header); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}
