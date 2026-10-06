package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/device/discover"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/service/printers"
)

// fixedFinder answers every search with the same result.
type fixedFinder struct {
	res   discover.Result
	err   error
	calls int
}

func (f *fixedFinder) Find(context.Context) (discover.Result, error) {
	f.calls++
	return f.res, f.err
}

// discoverTill is a till whose printer settings search with f, or with
// nothing when f is nil.
func discoverTill(t *testing.T, f *fixedFinder) *till {
	t.Helper()
	w := newTill(t)
	q, err := printq.New(printq.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	var opts []printers.Option
	if f != nil {
		opts = append(opts, printers.WithFinder(f))
	}
	svc, err := printers.New(w.db, q, nil, "1.2.3", opts...)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithPrinters(svc), WithPrints(q))
	return w
}

// TestDiscoverPrintersOverHTTP: someone who may edit settings searches, sees
// what answered and which of it the venue already has, and can add a find
// exactly as it was given.
func TestDiscoverPrintersOverHTTP(t *testing.T) {
	f := &fixedFinder{res: discover.Result{
		Network:  []string{"192.168.1.50", "192.168.1.60"},
		Local:    []string{"EPSON TM-T82"},
		Searched: []string{"192.168.1.0/24"},
		LocalErr: errors.New("half the spooler answered, from C:\\Windows"),
	}}
	w := discoverTill(t, f)
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	if rec := send(w.handler, http.MethodPost, "/api/v1/printers", `{"name":"Counter","transport":"tcp","address":"192.168.1.50","paper_mm":80}`, owner); rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("discover: %d %s", rec.Code, rec.Body)
	}
	var out discoverResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("discover: %v: %s", err, rec.Body)
	}
	if len(out.Network) != 2 || out.Network[0] != (printers.Candidate{Address: "192.168.1.50", Known: true}) ||
		out.Network[1] != (printers.Candidate{Address: "192.168.1.60"}) ||
		len(out.USB) != 1 || out.USB[0].Address != "EPSON TM-T82" || out.USB[0].Known ||
		len(out.Searched) != 1 || out.Searched[0] != "192.168.1.0/24" {
		t.Errorf("discover = %+v", out)
	}
	// Why the installed list is short is the log's, not the caller's.
	if strings.Contains(rec.Body.String(), "spooler") {
		t.Errorf("the response says what went wrong inside: %s", rec.Body)
	}

	// A find goes back in as it came out.
	body := `{"name":"Bar","transport":"windows","address":"` + out.USB[0].Address + `","paper_mm":80}`
	if rec := send(w.handler, http.MethodPost, "/api/v1/printers", body, owner); rec.Code != http.StatusCreated {
		t.Errorf("adding a found printer: %d %s", rec.Code, rec.Body)
	}
}

func TestDiscoverPrintersRefusals(t *testing.T) {
	t.Run("a cashier may not make the server search", func(t *testing.T) {
		f := &fixedFinder{}
		w := discoverTill(t, f)
		cashier := w.asStaff(w.signInAs(t, "stf_ASHA0001", "2468").Token)
		rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", cashier)
		if rec.Code != http.StatusForbidden || f.calls != 0 {
			t.Errorf("status %d after %d searches, want 403 and none: %s", rec.Code, f.calls, rec.Body)
		}
	})
	t.Run("nor may a device with nobody signed in, or no device at all", func(t *testing.T) {
		f := &fixedFinder{}
		w := discoverTill(t, f)
		for _, header := range []map[string]string{nil, w.asDevice()} {
			rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", header)
			if rec.Code != http.StatusUnauthorized || f.calls != 0 {
				t.Errorf("status %d after %d searches, want 401 and none", rec.Code, f.calls)
			}
		}
	})
	t.Run("it is a POST", func(t *testing.T) {
		w := discoverTill(t, &fixedFinder{})
		owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
		rec := send(w.handler, http.MethodGet, "/api/v1/printers/discover", "", owner)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET: status %d, want 405", rec.Code)
		}
	})
	t.Run("a server with no way to search says so", func(t *testing.T) {
		w := discoverTill(t, nil)
		owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
		rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", owner)
		if rec.Code != http.StatusNotImplemented || errorOf(t, rec).Code != CodeNoPrinterSearch {
			t.Errorf("status %d, body %s", rec.Code, rec.Body)
		}
	})
	t.Run("a search that fails is an internal error, and says no more", func(t *testing.T) {
		w := discoverTill(t, &fixedFinder{err: errors.New("netlink: permission denied")})
		owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
		rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", owner)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "netlink") {
			t.Errorf("status %d, body %s", rec.Code, rec.Body)
		}
	})
	t.Run("searching is limited", func(t *testing.T) {
		f := &fixedFinder{}
		w := discoverTill(t, f)
		owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
		for i := range discoverLimit.requests {
			if rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", owner); rec.Code != http.StatusOK {
				t.Fatalf("search %d: status %d", i+1, rec.Code)
			}
		}
		rec := send(w.handler, http.MethodPost, "/api/v1/printers/discover", "", owner)
		if rec.Code != http.StatusTooManyRequests || f.calls != discoverLimit.requests {
			t.Errorf("status %d after %d searches, want 429 after %d", rec.Code, f.calls, discoverLimit.requests)
		}
	})
}
