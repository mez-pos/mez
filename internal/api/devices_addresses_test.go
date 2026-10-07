package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/device/discover"
	"github.com/byjoellobo/mez/internal/domain/qr"
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
