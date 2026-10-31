package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/device/discover"
	"github.com/mez-pos/mez/internal/domain/qr"
)

func addressesOf(t *testing.T, w *till, headers map[string]string) (int, addressesResponse, string) {
	t.Helper()
	rec := send(w.handler, http.MethodGet, "/api/v1/devices/addresses", "", headers)
	var out addressesResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("addresses: %v: %s", err, rec.Body)
		}
	}
	return rec.Code, out, rec.Body.String()
}

// TestServerAddresses: each address comes with its adapter, and with what
// each kind of device opens there, as an address and as a QR code of
// exactly that address.
func TestServerAddresses(t *testing.T) {
	w := newTill(t)
	w.rebuild(t, WithAddresses(func() ([]discover.Address, error) {
		return []discover.Address{
			{URL: "http://192.168.1.10:7777", Adapter: "Wi-Fi"},
			{URL: "http://172.25.224.1:7777/", Adapter: "vEthernet (WSL)", Virtual: true},
		}, nil
	}))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	status, out, body := addressesOf(t, w, owner)
	if status != http.StatusOK || len(out.Addresses) != 2 {
		t.Fatalf("status %d: %s", status, body)
	}
	wifi, wsl := out.Addresses[0], out.Addresses[1]
	if wifi.Adapter != "Wi-Fi" || wifi.Virtual || wsl.Adapter != "vEthernet (WSL)" || !wsl.Virtual {
		t.Errorf("adapters = %q (virtual %v), %q (virtual %v)", wifi.Adapter, wifi.Virtual, wsl.Adapter, wsl.Virtual)
	}
	want := map[auth.DeviceKind]string{
		auth.KindTerminal:   "http://192.168.1.10:7777",
		auth.KindKitchen:    "http://192.168.1.10:7777/kitchen",
		auth.KindBackOffice: "http://192.168.1.10:7777/backoffice/",
	}
	if len(wifi.Apps) != len(want) {
		t.Fatalf("apps = %+v", wifi.Apps)
	}
	for _, app := range wifi.Apps {
		if app.URL != want[app.Kind] {
			t.Errorf("a %s opens %q, want %q", app.Kind, app.URL, want[app.Kind])
		}
		code, err := qr.Encode([]byte(app.URL))
		if err != nil {
			t.Fatal(err)
		}
		if app.SVG != code.SVG(qrQuiet) || !strings.HasPrefix(app.SVG, "<svg") {
			t.Errorf("the QR for a %s is not the code of %q", app.Kind, app.URL)
		}
	}
	// A trailing slash on the address does not double up.
	if got := wsl.Apps[1].URL; got != "http://172.25.224.1:7777/kitchen" {
		t.Errorf("kitchen at the second address = %q", got)
	}
}

// TestServerAddressesByName: where the server answers for mez.local
// (ADR-379), each kind's address by that name comes with the numbered ones,
// with a QR code of exactly it, so the wizard can offer the name first
// (SET-1). It comes even when the numbered addresses could not be read, and
// it is an empty list, not a missing one, when there is no name.
func TestServerAddressesByName(t *testing.T) {
	want := map[auth.DeviceKind]string{
		auth.KindTerminal:   "http://mez.local:7777",
		auth.KindKitchen:    "http://mez.local:7777/kitchen",
		auth.KindBackOffice: "http://mez.local:7777/backoffice/",
	}
	for name, addresses := range map[string]func() ([]discover.Address, error){
		"beside the numbered addresses":            func() ([]discover.Address, error) { return []discover.Address{{URL: "http://192.168.1.10:7777"}}, nil },
		"when the numbered ones could not be read": func() ([]discover.Address, error) { return nil, errors.New("no adapters") },
	} {
		t.Run(name, func(t *testing.T) {
			w := newTill(t)
			w.rebuild(t, WithAddresses(addresses), WithNamedAddress(func() string { return "http://mez.local:7777/" }))
			owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
			status, out, body := addressesOf(t, w, owner)
			if status != http.StatusOK || len(out.Named) != len(want) {
				t.Fatalf("status %d: %s", status, body)
			}
			for _, app := range out.Named {
				if app.URL != want[app.Kind] {
					t.Errorf("a %s opens %q by name, want %q", app.Kind, app.URL, want[app.Kind])
				}
				code, err := qr.Encode([]byte(app.URL))
				if err != nil {
					t.Fatal(err)
				}
				if app.SVG != code.SVG(qrQuiet) {
					t.Errorf("the QR for a %s by name is not the code of %q", app.Kind, app.URL)
				}
			}
		})
	}

	for name, opts := range map[string][]Option{
		"a server announcing no name":     nil,
		"an announcer that did not start": {WithNamedAddress(func() string { return "" })},
	} {
		t.Run(name, func(t *testing.T) {
			w := newTill(t)
			w.rebuild(t, append([]Option{WithAddresses(func() ([]discover.Address, error) {
				return []discover.Address{{URL: "http://192.168.1.10:7777"}}, nil
			})}, opts...)...)
			owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
			status, out, body := addressesOf(t, w, owner)
			if status != http.StatusOK || len(out.Named) != 0 || !strings.Contains(body, `"named":[]`) || len(out.Addresses) != 1 {
				t.Errorf("status %d, body %s", status, body)
			}
		})
	}
}

func TestServerAddressesWithNone(t *testing.T) {
	for name, addresses := range map[string]func() ([]discover.Address, error){
		"a server on this computer alone":     func() ([]discover.Address, error) { return nil, nil },
		"addresses that could not be read":    func() ([]discover.Address, error) { return nil, errors.New("netlink: permission denied") },
		"an empty list, said as an empty one": func() ([]discover.Address, error) { return []discover.Address{}, nil },
	} {
		t.Run(name, func(t *testing.T) {
			w := newTill(t)
			w.rebuild(t, WithAddresses(addresses))
			owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
			status, out, body := addressesOf(t, w, owner)
			// A list, not null, and not a failure: the code still pairs a device.
			if status != http.StatusOK || len(out.Addresses) != 0 || !strings.Contains(body, `"addresses":[]`) || strings.Contains(body, "netlink") {
				t.Errorf("status %d, body %s", status, body)
			}
		})
	}
}

func TestServerAddressesRefusals(t *testing.T) {
	asked := 0
	addresses := func() ([]discover.Address, error) {
		asked++
		return []discover.Address{{URL: "http://192.168.1.10:7777"}}, nil
	}

	w := newTill(t)
	w.rebuild(t, WithAddresses(addresses))
	// A cashier pairs nothing, and is not told where the server is by this.
	cashier := w.asStaff(w.signInAs(t, "stf_ASHA0001", "2468").Token)
	for name, headers := range map[string]map[string]string{"nobody": nil, "a device with nobody signed in": w.asDevice(), "a cashier": cashier} {
		status, _, _ := addressesOf(t, w, headers)
		if status != http.StatusUnauthorized && status != http.StatusForbidden {
			t.Errorf("%s: status %d, want 401 or 403", name, status)
		}
	}
	if asked != 0 {
		t.Errorf("the addresses were read %d times for callers who may not have them", asked)
	}

	// A server built without them has no such endpoint.
	bare := newTill(t)
	owner := bare.asStaff(bare.signInAs(t, "stf_PRIYA001", "2468").Token)
	if status, _, _ := addressesOf(t, bare, owner); status == http.StatusOK {
		t.Errorf("a server with no addresses answered %d", status)
	}
}

// TestAPairingCodeSaysWhereToOpenMez: the code comes with each address a
// device of its kind opens mez at, real adapters first, and a QR code of
// that address with the code in its fragment, so a camera pairs it in one
// go (ADR-367). The address to type carries no code.
func TestAPairingCodeSaysWhereToOpenMez(t *testing.T) {
	w := newTill(t)
	w.rebuild(t, WithAddresses(func() ([]discover.Address, error) {
		return []discover.Address{
			{URL: "http://192.168.1.10:7777", Adapter: "Wi-Fi"},
			{URL: "http://172.25.224.1:7777/", Adapter: "vEthernet (WSL)", Virtual: true},
		}, nil
	}))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	tests := []struct {
		kind    string
		wantURL string
		wantQR  string
	}{
		{"terminal", "http://192.168.1.10:7777", "http://192.168.1.10:7777/#code="},
		{"kitchen", "http://192.168.1.10:7777/kitchen", "http://192.168.1.10:7777/kitchen#code="},
		{"back_office", "http://192.168.1.10:7777/backoffice/", "http://192.168.1.10:7777/backoffice/#code="},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", `{"name":"Second","kind":"`+tt.kind+`"}`, owner)
			if rec.Code != http.StatusCreated {
				t.Fatalf("a code: %d %s", rec.Code, rec.Body)
			}
			var out pairingCodeResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Open) != 2 || out.Open[0].Adapter != "Wi-Fi" || out.Open[0].Virtual || !out.Open[1].Virtual {
				t.Fatalf("open = %+v", out.Open)
			}
			if out.Open[0].URL != tt.wantURL {
				t.Errorf("url = %q, want %q", out.Open[0].URL, tt.wantURL)
			}
			drawn, err := qr.Encode([]byte(tt.wantQR + out.Code))
			if err != nil {
				t.Fatal(err)
			}
			if out.Open[0].SVG != drawn.SVG(qrQuiet) {
				t.Errorf("the QR is not of %q", tt.wantQR+out.Code)
			}
		})
	}

	// Without addresses to give, the code still comes, and says nothing of
	// where to open it.
	w.rebuild(t, WithAddresses(func() ([]discover.Address, error) { return nil, errors.New("no adapters") }))
	rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", `{"name":"Third","kind":"terminal"}`, owner)
	var out pairingCodeResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Code == "" || len(out.Open) != 0 {
		t.Fatalf("without addresses: %d %s", rec.Code, rec.Body)
	}
}
