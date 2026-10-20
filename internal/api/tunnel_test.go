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

	"github.com/mez-pos/mez/internal/relay"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/tunnel"
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
	key, err := rl.keys.Issue(rand.Reader, "cafe", false)
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
		{"not entitled says why and when next", &TunnelStatus{State: TunnelNotEntitled, Since: since, RetryAt: retry, Detail: "the relay is not carrying this venue: payment_failed"},
			tunnelResponse{State: TunnelNotEntitled, Since: utc(since), RetryAt: utc(retry), Detail: "the relay is not carrying this venue: payment_failed"}, CheckFailing},
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

	// Whether the back office crosses the relay is the venue's setting, said
	// in every state but off, where there is no relay for it to cross
	// (ADR-306). Health says nothing of it either way: health is public.
	t.Run("back_office follows the setting", func(t *testing.T) {
		for _, tt := range []struct {
			name   string
			status *TunnelStatus
			want   string
		}{
			{"no relay", nil, `"back_office":false`},
			{"off, whatever was reported", &TunnelStatus{State: TunnelOff, BackOffice: true}, `"back_office":false`},
			{"connected, setting off", &TunnelStatus{State: TunnelConnected, Since: since}, `"back_office":false`},
			{"connected, setting on", &TunnelStatus{State: TunnelConnected, Since: since, BackOffice: true}, `"back_office":true`},
			{"waiting, setting on", &TunnelStatus{State: TunnelWaiting, Since: since, BackOffice: true}, `"back_office":true`},
		} {
			var opts []Option
			if tt.status != nil {
				opts = append(opts, WithTunnel(func() TunnelStatus { return *tt.status }))
			}
			r.rebuild(t, opts...)
			rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
			if !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("%s: body = %s, want %s", tt.name, rec.Body, tt.want)
			}
			health := send(r.handler, http.MethodGet, "/api/v1/health", "", nil)
			if strings.Contains(health.Body.String(), "back_office") {
				t.Errorf("%s: health says whether the back office is open: %s", tt.name, health.Body)
			}
		}
	})

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

// TestTheAddressToOpenTheBackOfficeAt: what the Devices screen shows an
// owner to open on their phone. Only with remote_backoffice on, and only an
// address that would open: the venue's own hostname, with nothing after it
// (ADR-309, phase 13 Task 4).
func TestTheAddressToOpenTheBackOfficeAt(t *testing.T) {
	r := newRegister(t)
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
	on := TunnelStatus{State: TunnelConnected, BackOffice: true}
	off := TunnelStatus{State: TunnelConnected}

	tests := []struct {
		name      string
		status    TunnelStatus
		publicURL string
		want      string // back_office_url, or empty for absent
	}{
		{"on, at the venue's own hostname", on, "https://cafe.mezpos.com", "https://cafe.mezpos.com/backoffice/"},
		{"a trailing slash is the same address", on, "https://cafe.mezpos.com/", "https://cafe.mezpos.com/backoffice/"},
		{"with a port", on, "http://cafe.mez.localhost:7811", "http://cafe.mez.localhost:7811/backoffice/"},
		{"on, but still reconnecting: the address is the same", TunnelStatus{State: TunnelWaiting, BackOffice: true}, "https://cafe.mezpos.com", "https://cafe.mezpos.com/backoffice/"},
		{"off: no address, whatever public_url says", off, "https://cafe.mezpos.com", ""},
		{"on, and public_url not set", on, "", ""},
		{"on, reached under a path, where it would not open", on, "https://relay.example.com/v/cafe", ""},
		{"on, and public_url is not an address", on, "cafe", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.rebuild(t, WithTunnel(func() TunnelStatus { return tt.status }), WithTableCodes(nil, tt.publicURL))
			rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, body %s", rec.Code, rec.Body)
			}
			var got tunnelResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.BackOfficeURL != tt.want || got.BackOffice != tt.status.BackOffice {
				t.Errorf("back_office %v, back_office_url %q; want %q", got.BackOffice, got.BackOfficeURL, tt.want)
			}
			if tt.want == "" && strings.Contains(rec.Body.String(), "back_office_url") {
				t.Errorf("an empty address is on the wire: %s", rec.Body)
			}
		})
	}
}

// TestTunnelSaysWhereTheVenueStands: what the relay told the venue is passed
// on as it was told, with a date that is not one left out, and nothing at
// all when the relay said nothing (ADR-316).
func TestTunnelSaysWhereTheVenueStands(t *testing.T) {
	paid := time.Date(2026, 11, 12, 0, 0, 0, 0, time.FixedZone("IST", 19800))
	until := paid.AddDate(0, 0, 7)
	r := newRegister(t)
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
	read := func(st TunnelStatus) tunnelResponse {
		t.Helper()
		r.rebuild(t, WithTunnel(func() TunnelStatus { return st }))
		status, got := r.tunnel(t, owner)
		if status != http.StatusOK {
			t.Fatalf("status = %d", status)
		}
		return got
	}

	if got := read(TunnelStatus{State: TunnelConnected}); got.Cloud != nil {
		t.Errorf("a relay that said nothing: cloud = %+v, want none", got.Cloud)
	}

	got := read(TunnelStatus{State: TunnelConnected, BackOffice: true,
		Cloud: &TunnelCloud{State: "grace", PaidUntil: paid, Until: until, BackOffice: true, Reason: "payment_failed"}})
	c := got.Cloud
	if c == nil || c.State != "grace" || c.Reason != "payment_failed" || !c.BackOffice ||
		!sameTime(c.PaidUntil, utc(paid)) || !sameTime(c.Until, utc(until)) {
		t.Errorf("cloud = %+v", c)
	}

	// A relay that keeps a file and set no date: only whether the back
	// office is carried.
	got = read(TunnelStatus{State: TunnelConnected, Cloud: &TunnelCloud{}})
	if c := got.Cloud; c == nil || c.State != "" || c.PaidUntil != nil || c.Until != nil || c.BackOffice {
		t.Errorf("cloud with nothing known = %+v, want present and empty", c)
	}
	rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
	if body := rec.Body.String(); !strings.Contains(body, `"cloud":{"back_office":false}`) {
		t.Errorf("on the wire: %s", body)
	}

	// Turned away: still said, since it is the reason the owner is looking.
	got = read(TunnelStatus{State: TunnelNotEntitled, Cloud: &TunnelCloud{State: "suspended", PaidUntil: paid, Until: until, Reason: "payment_failed"}})
	if got.State != TunnelNotEntitled || got.Cloud == nil || got.Cloud.State != "suspended" {
		t.Errorf("not entitled: %+v, cloud %+v", got, got.Cloud)
	}

	if got := read(TunnelStatus{State: TunnelOff, Cloud: &TunnelCloud{State: "active"}}); got.Cloud != nil {
		t.Errorf("no relay: cloud = %+v, want none", got.Cloud)
	}
}

// TestTunnelSaysWhoTheVenueIsPairedTo: the owner's name, the hostname and
// when, for a venue that is paired, and nothing at all for one that is not
// (ADR-318, ADR-319). Neither key is anywhere in the answer.
func TestTunnelSaysWhoTheVenueIsPairedTo(t *testing.T) {
	at := time.Date(2026, 10, 12, 14, 30, 0, 0, time.FixedZone("IST", 19800))
	r := newRegister(t)
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)

	r.rebuild(t, WithTunnel(func() TunnelStatus { return TunnelStatus{State: TunnelConnected} }))
	if _, got := r.tunnel(t, owner); got.PairedTo != nil {
		t.Errorf("a venue that is not paired: paired_to = %+v", got.PairedTo)
	}
	r.rebuild(t,
		WithTunnel(func() TunnelStatus { return TunnelStatus{State: TunnelConnected} }),
		WithPairing(func() (TunnelPairing, bool) { return TunnelPairing{}, false }))
	if _, got := r.tunnel(t, owner); got.PairedTo != nil {
		t.Errorf("a venue whose pairing is not in use: paired_to = %+v", got.PairedTo)
	}

	r.rebuild(t,
		WithTunnel(func() TunnelStatus { return TunnelStatus{State: TunnelWaiting} }),
		WithPairing(func() (TunnelPairing, bool) {
			return TunnelPairing{Owner: "Priya Menon", Hostname: "corner-cafe.mezpos.com", PairedAt: at}, true
		}))
	_, got := r.tunnel(t, owner)
	p := got.PairedTo
	if p == nil || p.Owner != "Priya Menon" || p.Hostname != "corner-cafe.mezpos.com" || !p.PairedAt.Equal(at) {
		t.Fatalf("paired_to = %+v", p)
	}
	rec := send(r.handler, http.MethodGet, "/api/v1/tunnel", "", owner)
	if body := rec.Body.String(); !strings.Contains(body, `"paired_at":"2026-10-12T09:00:00Z"`) || strings.Contains(body, "mzr_") {
		t.Errorf("on the wire: %s", body)
	}
}
