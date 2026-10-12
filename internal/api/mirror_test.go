package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

var mirroredAt = time.Date(2026, 10, 11, 17, 40, 0, 0, time.UTC)

// mirrorOf is a till's world served as a mirror, with a back office paired
// to it and the owner signed in: the headers of a request from that browser.
// Nothing here says a request came from away. A mirror takes every one so.
func mirrorOf(t *testing.T, extra ...Option) (*till, map[string]string) {
	t.Helper()
	w := newTill(t)
	w.rebuild(t, append([]Option{WithMirror(func() MirrorInfo { return MirrorInfo{AsOf: mirroredAt} })}, extra...)...)

	code, err := w.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Mirror", DeviceKind: auth.KindBackOffice})
	if err != nil {
		t.Fatal(err)
	}
	browser := pairOK(t, w.handler, code.Code).Token
	rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"2468"}`, bearerOf(browser))
	if rec.Code != http.StatusCreated {
		t.Fatalf("signing in on a mirror: status %d, body %s", rec.Code, rec.Body)
	}
	var in sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil {
		t.Fatal(err)
	}
	return w, map[string]string{"Authorization": "Bearer " + browser, HeaderStaffToken: in.Token}
}

func TestAMirrorIsReadAndSaysHowOldItIs(t *testing.T) {
	w, owner := mirrorOf(t)

	for _, path := range []string{"/api/v1/sessions/current", "/api/v1/staff", "/api/v1/devices", "/api/v1/health"} {
		rec := send(w.handler, http.MethodGet, path, "", owner)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status %d, body %s", path, rec.Code, rec.Body)
		}
		if got := rec.Header().Get(HeaderMirror); got != "2026-10-11T17:40:00Z" {
			t.Errorf("GET %s: %s = %q, want when the copy was last written", path, HeaderMirror, got)
		}
		// Asked from away, whoever asks: the screens leave out what cannot
		// be done from there.
		if got := rec.Header().Get(HeaderVia); path != "/api/v1/health" && got != ViaRelay {
			t.Errorf("GET %s: %s = %q", path, HeaderVia, got)
		}
	}

	// The session is the one a PIN from away opens: twelve hours.
	rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", owner)
	var current sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if left := time.Until(current.ExpiresAt); left > auth.RemoteSessionTTL || left < auth.RemoteSessionTTL-time.Minute {
		t.Errorf("the session has %s left, want twelve hours", left)
	}

	// What stays at the venue is not on a mirror either, read or not.
	rec = send(w.handler, http.MethodGet, "/api/v1/orders/today", "", owner)
	if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
		t.Errorf("the till's own list on a mirror: status %d, body %s", rec.Code, rec.Body)
	}
	// A till's pairing opens nothing on one.
	rec = send(w.handler, http.MethodGet, "/api/v1/staff", "", w.asDevice())
	if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
		t.Errorf("a terminal on a mirror: status %d, body %s", rec.Code, rec.Body)
	}
	// Nobody orders from a copy.
	for _, path := range []string{"/order/", "/order/t/ABCD"} {
		if rec := send(w.handler, http.MethodGet, path, "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s on a mirror: status %d", path, rec.Code)
		}
	}

	// A path that decodes to one under the back office's and is routed
	// elsewhere reaches nothing: not the till's bundle, which serves "/".
	for _, path := range []string{"/backoffice%2f", "/backoffice%2f..%2fassets/x", "/order%2f", "/"} {
		if rec := send(w.handler, http.MethodGet, path, "", nil); rec.Code == http.StatusOK {
			t.Errorf("GET %s on a mirror: status %d, body %.80s", path, rec.Code, rec.Body)
		}
	}

	// Signing out is one of the few things a mirror lets happen.
	if rec := send(w.handler, http.MethodDelete, "/api/v1/sessions/current", "", owner); rec.Code != http.StatusNoContent {
		t.Errorf("signing out of a mirror: status %d, body %s", rec.Code, rec.Body)
	}
}

// TestAMirrorRefusesEveryWrite walks every route the API registers. Each
// one that is not a read, and not one of the few a mirror lets through by
// name, is answered 409 before its handler, whoever asks and whether or
// not it could be done from away.
func TestAMirrorRefusesEveryWrite(t *testing.T) {
	w, owner := mirrorOf(t)
	param := regexp.MustCompile(`\{[^}]+\}`)

	refused := 0
	for _, pattern := range registeredRoutes(t) {
		method, path, _ := strings.Cut(pattern, " ")
		if method == http.MethodGet {
			continue
		}
		path = param.ReplaceAllString(path, "1")
		_, let := mirrorWrites[pattern]
		for name, headers := range map[string]map[string]string{"signed in": owner, "nobody": nil} {
			rec := send(w.handler, method, path, `{}`, headers)
			is409 := rec.Code == http.StatusConflict && errorOf(t, rec).Code == CodeMirrorReadOnly
			switch {
			// The two previews are settings routes, which this world does
			// not mount: there they fall to the catch-all and are refused
			// with everything else. The four that let someone in are served.
			case let && is409 && !strings.Contains(pattern, "/settings/"):
				t.Errorf("%s (%s): refused as a write, and it is one a mirror takes", pattern, name)
			case !let && !is409:
				t.Errorf("%s (%s): status %d, body %s; want 409 %s", pattern, name, rec.Code, rec.Body, CodeMirrorReadOnly)
			case !let:
				refused++
				if rec.Header().Get(HeaderMirror) == "" {
					t.Errorf("%s: the refusal does not say it is a mirror's", pattern)
				}
			}
		}
	}
	if refused < 100 {
		t.Fatalf("only %d writes were tried; the walk is broken", refused)
	}

	// And a route nobody registered, sent as a write.
	rec := send(w.handler, http.MethodPost, "/api/v1/not-a-route", `{}`, owner)
	if rec.Code != http.StatusConflict {
		t.Errorf("an unknown route as a write: status %d", rec.Code)
	}
}

// Every write a mirror takes is a route that exists, so a rename cannot
// leave a name here that lets nothing through and nobody notices.
func TestTheMirrorsWritesAreRoutes(t *testing.T) {
	registered := map[string]bool{}
	for _, p := range registeredRoutes(t) {
		registered[p] = true
	}
	for pattern := range mirrorWrites {
		if !registered[pattern] {
			t.Errorf("mirrorWrites names %q, which is not a route", pattern)
		}
		// And each is one the relay's gate passes: a mirror is judged by
		// that gate next.
		if !remoteAllows(pattern) {
			t.Errorf("mirrorWrites names %q, which cannot be reached from away", pattern)
		}
	}
}

func TestAServerThatIsNotAMirrorSaysNothingOfOne(t *testing.T) {
	w := newTill(t)
	rec := send(w.handler, http.MethodGet, "/api/v1/health", "", nil)
	if _, present := rec.Header()[HeaderMirror]; present {
		t.Errorf("an ordinary server answers with %s", HeaderMirror)
	}
}
