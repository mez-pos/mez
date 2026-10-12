package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
)

// fakePairing answers a pairing as it is told to, and remembers the asking.
type fakePairing struct {
	res   cloudpair.Result
	err   error
	codes []string
	venue int64
}

func (f *fakePairing) Pair(_ context.Context, venueID int64, code string) (cloudpair.Result, error) {
	f.codes = append(f.codes, code)
	f.venue = venueID
	return f.res, f.err
}

// TestPairingToCloudOverHTTP: the owner types a code in the back office.
// What comes back says who and where, and never a key; each way it does not
// work is said in words an owner can act on, with whether the code is still
// good (ADR-320).
func TestPairingToCloudOverHTTP(t *testing.T) {
	at := time.Date(2026, 10, 12, 14, 30, 0, 0, time.FixedZone("IST", 19800))
	r := newRegister(t)
	fake := &fakePairing{res: cloudpair.Result{
		Owner: "Priya Menon", Hostname: "corner-cafe.mezpos.com", PairedAt: at, Backup: cloudpair.BackupSet,
	}}
	r.rebuild(t, WithCloudPairing(fake))
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
	const path = "/api/v1/cloud/pair"

	rec := send(r.handler, http.MethodPost, path, `{"code":"abcd-2345"}`, owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("pairing: %d %s", rec.Code, rec.Body)
	}
	var got cloudPairResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Owner != "Priya Menon" || got.Hostname != "corner-cafe.mezpos.com" || !got.PairedAt.Equal(at) ||
		got.Backup != cloudpair.BackupSet || got.RestartNeeded {
		t.Errorf("answer %+v", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"paired_at":"2026-10-12T09:00:00Z"`) || strings.Contains(body, "mzr_") || strings.Contains(body, "secret") {
		t.Errorf("on the wire: %s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the answer may be cached")
	}
	// As typed: cleaning it is the service's, which is where it is tested.
	if len(fake.codes) != 1 || fake.codes[0] != "abcd-2345" || fake.venue == 0 {
		t.Errorf("the service was asked %v for venue %d", fake.codes, fake.venue)
	}

	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   ErrorCode
		says   string
	}{
		{"a code that could not be one", cloud.ErrCodeShape, http.StatusUnprocessableEntity, CodePairingCodeInvalid, "8 letters and digits"},
		{"a code the cloud refuses", cloud.ErrCodeRefused, http.StatusConflict, CodePairingCodeRefused, "Make a new one"},
		{"a busy cloud", cloud.ErrBusy, http.StatusTooManyRequests, CodeRateLimited, "the code is still good"},
		{"no internet", fmt.Errorf("%w: dial tcp 203.0.113.9:443: no route", cloud.ErrUnreachable), http.StatusBadGateway, CodeCloudUnreachable, "the code is still good"},
		{"an answer that is not one", fmt.Errorf("%w: the relay key is for another venue", cloud.ErrInvalid), http.StatusBadGateway, CodeCloudUnreachable, "Update mez"},
		{"a relay in the settings file", cloudpair.ErrRelayInSettings, http.StatusConflict, CodeRelayInSettings, "The code was not used"},
		{"it could not be written down", cloudpair.ErrNotKept, http.StatusInternalServerError, CodePairingNotKept, "Make a new code"},
		{"something nobody expected", errors.New("the disk has gone"), http.StatusInternalServerError, CodeInternal, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A server built again starts its count of tries again: there
			// are more cases here than one address may try in a minute.
			r.rebuild(t, WithCloudPairing(fake))
			fake.err = tt.err
			rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, owner)
			e := errorOf(t, rec)
			if rec.Code != tt.status || e.Code != tt.code {
				t.Fatalf("status %d code %s, want %d %s: %s", rec.Code, e.Code, tt.status, tt.code, rec.Body)
			}
			if !strings.Contains(e.Message, tt.says) {
				t.Errorf("message %q does not say %q", e.Message, tt.says)
			}
			// What went wrong underneath is the log's, not the screen's.
			for _, leak := range []string{"203.0.113.9", "another venue", "the disk has gone"} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("the answer repeats %q: %s", leak, rec.Body)
				}
			}
		})
	}
	fake.err = nil

	// The tries above are most of what one address may make in a minute. A
	// server built again starts its count again.
	t.Run("an address that keeps trying is told to wait", func(t *testing.T) {
		limited := false
		for range cloudPairLimit.requests + 1 {
			rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, owner)
			limited = limited || rec.Code == http.StatusTooManyRequests
		}
		if !limited {
			t.Errorf("more than %d tries in a minute were all let through", cloudPairLimit.requests)
		}
	})
	r.rebuild(t, WithCloudPairing(fake))

	t.Run("a body that is not a code", func(t *testing.T) {
		before := len(fake.codes)
		if rec := send(r.handler, http.MethodPost, path, `{"cod`, owner); rec.Code != http.StatusBadRequest {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
		if len(fake.codes) != before {
			t.Error("the service was asked with no code")
		}
	})
	t.Run("only by someone who may change settings, signed in", func(t *testing.T) {
		before := len(fake.codes)
		cashier := r.asStaff(r.signInAs(t, "stf_ASHA0001", "2468").Token)
		if rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, cashier); rec.Code != http.StatusForbidden {
			t.Errorf("a cashier: %d %s", rec.Code, rec.Body)
		}
		if rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, r.asDevice()); rec.Code != http.StatusUnauthorized {
			t.Errorf("nobody signed in: %d %s", rec.Code, rec.Body)
		}
		if rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("no device: %d %s", rec.Code, rec.Body)
		}
		if len(fake.codes) != before {
			t.Error("a code was tried for someone who may not pair the venue")
		}
	})
	t.Run("not from away", func(t *testing.T) {
		r.rebuild(t, WithCloudPairing(fake))
		before := len(fake.codes)
		office := r.backOffice(t)
		rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, viaRelay(office))
		if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed || len(fake.codes) != before {
			t.Errorf("through the relay: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("only by POST", func(t *testing.T) {
		if rec := send(r.handler, http.MethodGet, path, "", owner); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET: %d", rec.Code)
		}
	})
	t.Run("a server that cannot pair says so", func(t *testing.T) {
		r.rebuild(t)
		owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)
		rec := send(r.handler, http.MethodPost, path, `{"code":"ABCD2345"}`, owner)
		if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodePairingUnavailable {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})
}

// TestTheVenuesAddressCanChangeWhileRunning: a venue paired after it started
// has an address from that moment, and the table codes that are drawn from
// it stop saying there is none (ADR-320).
func TestTheVenuesAddressCanChangeWhileRunning(t *testing.T) {
	r := newRegister(t)
	address := ""
	r.rebuild(t,
		WithTunnel(func() TunnelStatus { return TunnelStatus{State: TunnelConnected, BackOffice: true} }),
		WithPublicURL(func() string { return address }))
	owner := r.asStaff(r.signInAs(t, "stf_PRIYA001", "2468").Token)

	if _, got := r.tunnel(t, owner); got.BackOfficeURL != "" {
		t.Errorf("with no address: back_office_url = %q", got.BackOfficeURL)
	}
	address = "https://corner-cafe.mezpos.com/"
	if _, got := r.tunnel(t, owner); got.BackOfficeURL != "https://corner-cafe.mezpos.com/backoffice/" {
		t.Errorf("once it has one: back_office_url = %q", got.BackOfficeURL)
	}
}
