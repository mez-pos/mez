package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func addressesOf(t *testing.T, w *till, headers map[string]string) (int, []string, string) {
	t.Helper()
	rec := send(w.handler, http.MethodGet, "/api/v1/devices/addresses", "", headers)
	if rec.Code != http.StatusOK {
		return rec.Code, nil, rec.Body.String()
	}
	var out addressesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("addresses: %v: %s", err, rec.Body)
	}
	return rec.Code, out.URLs, rec.Body.String()
}

func TestServerAddresses(t *testing.T) {
	tests := []struct {
		name string
		urls func() ([]string, error)
		want []string
	}{
		{"the server's addresses, in its order", func() ([]string, error) { return []string{"http://192.168.1.10:7777", "http://10.0.75.1:7777"}, nil },
			[]string{"http://192.168.1.10:7777", "http://10.0.75.1:7777"}},
		{"a server on this computer alone has none, as a list", func() ([]string, error) { return nil, nil }, []string{}},
		{"addresses that cannot be read are none, and not a failure", func() ([]string, error) { return nil, errors.New("netlink: permission denied") }, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTill(t)
			w.rebuild(t, WithAddresses(tt.urls))
			owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
			status, urls, body := addressesOf(t, w, owner)
			if status != http.StatusOK || urls == nil || !slices.Equal(urls, tt.want) {
				t.Errorf("status %d, urls %v, want %v: %s", status, urls, tt.want, body)
			}
			if !strings.Contains(body, `"urls":[`) || strings.Contains(body, "netlink") {
				t.Errorf("body = %s", body)
			}
		})
	}
}

func TestServerAddressesRefusals(t *testing.T) {
	asked := 0
	urls := func() ([]string, error) { asked++; return []string{"http://192.168.1.10:7777"}, nil }

	w := newTill(t)
	w.rebuild(t, WithAddresses(urls))
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
