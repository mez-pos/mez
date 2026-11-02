package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/device/discover"
	"github.com/mez-pos/mez/internal/device/tabletapp"
	"github.com/mez-pos/mez/internal/domain/qr"
)

// testApp is a tablet app as a release would carry it (ADR-421).
func testApp(t *testing.T) *tabletapp.App {
	t.Helper()
	app, err := tabletapp.Read([]byte("PK\x03\x04"+strings.Repeat("an apk ", 100)), []byte(`{"version":"v0.2.0-beta.3","version_code":20036}`))
	if err != nil {
		t.Fatal(err)
	}
	return &app
}

// TestTheTabletAppIsServed: the package itself, to a tablet that has never
// seen mez and so has no token, as the package it is, named with its
// version; its digest the ETag, and ranges served, so a download broken by
// poor wifi resumes rather than starting again.
func TestTheTabletAppIsServed(t *testing.T) {
	app := testApp(t)
	w := newTill(t)
	w.rebuild(t, WithTabletApp(app))

	rec := send(w.handler, http.MethodGet, "/app/mez.apk", "", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != string(app.APK) {
		t.Fatalf("the package: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type":        "application/vnd.android.package-archive",
		"Content-Disposition": `attachment; filename="mez-v0.2.0-beta.3.apk"`,
		"ETag":                `"` + app.SHA256 + `"`,
		"Accept-Ranges":       "bytes",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	part := send(w.handler, http.MethodGet, "/app/mez.apk", "", map[string]string{"Range": "bytes=4-9"})
	if part.Code != http.StatusPartialContent || part.Body.String() != string(app.APK[4:10]) {
		t.Errorf("a range: %d %q", part.Code, part.Body)
	}
	past := send(w.handler, http.MethodGet, "/app/mez.apk", "", map[string]string{"Range": "bytes=100000-"})
	if past.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("a range past the end: %d, want 416", past.Code)
	}
	head := send(w.handler, http.MethodHead, "/app/mez.apk", "", nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != apkType {
		t.Errorf("HEAD: %d, %d bytes, %q", head.Code, head.Body.Len(), head.Header().Get("Content-Type"))
	}
	same := send(w.handler, http.MethodGet, "/app/mez.apk", "", map[string]string{"If-None-Match": `"` + app.SHA256 + `"`})
	if same.Code != http.StatusNotModified {
		t.Errorf("the package a tablet has: %d, want 304", same.Code)
	}
	if rec := send(w.handler, http.MethodPost, "/app/mez.apk", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", rec.Code)
	}
}

// TestTheTabletAppsVersion: what the app is, for a tablet to know whether
// the PC carries a newer one (Task 8), with no token.
func TestTheTabletAppsVersion(t *testing.T) {
	app := testApp(t)
	w := newTill(t)
	w.rebuild(t, WithTabletApp(app))

	rec := send(w.handler, http.MethodGet, "/api/v1/app", "", nil)
	var out tabletAppResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("the version: %d %s", rec.Code, rec.Body)
	}
	want := tabletAppResponse{Version: "v0.2.0-beta.3", VersionCode: 20036, Size: len(app.APK), SHA256: app.SHA256, Path: "/app/mez.apk"}
	if out != want {
		t.Errorf("version = %+v, want %+v", out, want)
	}
	if rec := send(w.handler, http.MethodPost, "/api/v1/app", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", rec.Code)
	}
}

// TestABuildWithoutTheTabletAppSaysSo: no package and no version, each
// answered in words, not a till's page or an empty file.
func TestABuildWithoutTheTabletAppSaysSo(t *testing.T) {
	w := newTill(t)
	for _, path := range []string{"/app/mez.apk", "/api/v1/app"} {
		rec := send(w.handler, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), string(CodeAppNotBuilt)) ||
			!strings.Contains(rec.Body.String(), "built without the tablet app") {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
}

// TestTheTabletAppStaysAtTheVenue: a tablet installs on the venue's
// network; through the relay neither the package nor its version is given.
func TestTheTabletAppStaysAtTheVenue(t *testing.T) {
	w := newTill(t)
	w.rebuild(t, WithTabletApp(testApp(t)))
	for _, path := range []string{"/app/mez.apk", "/api/v1/app"} {
		rec := send(w.handler, http.MethodGet, path, "", map[string]string{HeaderVia: ViaRelay})
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), string(CodeRemoteNotAllowed)) {
			t.Errorf("%s through the relay: %d %s", path, rec.Code, rec.Body)
		}
	}
}

// TestAPairingCodeSaysWhereTheAppIs: pairing a till or a kitchen display,
// each address comes with the app's address there and its QR, and the code
// says which app this build carries. A back office is a browser: no app. A
// build without the app says so with an empty version, and gives no
// address for it.
func TestAPairingCodeSaysWhereTheAppIs(t *testing.T) {
	addresses := WithAddresses(func() ([]discover.Address, error) {
		return []discover.Address{
			{URL: "http://192.168.1.10:7777", Adapter: "Wi-Fi"},
			{URL: "http://172.25.224.1:7777/", Adapter: "vEthernet (WSL)", Virtual: true},
		}, nil
	})
	code := func(t *testing.T, w *till, owner map[string]string, kind string) pairingCodeResponse {
		t.Helper()
		rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", `{"name":"Tablet `+kind+`","kind":"`+kind+`"}`, owner)
		var out pairingCodeResponse
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Open) != 2 {
			t.Fatalf("a code for a %s: %d %s", kind, rec.Code, rec.Body)
		}
		return out
	}

	w := newTill(t)
	w.rebuild(t, addresses, WithTabletApp(testApp(t)))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	for _, kind := range []string{"terminal", "kitchen"} {
		t.Run(kind, func(t *testing.T) {
			out := code(t, w, owner, kind)
			if out.AppVersion != "v0.2.0-beta.3" {
				t.Errorf("app_version = %q", out.AppVersion)
			}
			for i, want := range []string{"http://192.168.1.10:7777/app/mez.apk", "http://172.25.224.1:7777/app/mez.apk"} {
				place := out.Open[i]
				drawn, err := qr.Encode([]byte(want))
				if err != nil {
					t.Fatal(err)
				}
				if place.AppURL != want || place.AppSVG != drawn.SVG(qrQuiet) {
					t.Errorf("place %d: app at %q, its QR drawn: %v; want %q", i, place.AppURL, place.AppSVG != "", want)
				}
			}
		})
	}
	t.Run("back office", func(t *testing.T) {
		out := code(t, w, owner, "back_office")
		if out.Open[0].AppURL != "" || out.Open[0].AppSVG != "" {
			t.Errorf("a back office is offered the app: %+v", out.Open[0])
		}
	})

	bare := newTill(t)
	bare.rebuild(t, addresses)
	bareOwner := bare.asStaff(bare.signInAs(t, "stf_PRIYA001", "2468").Token)
	rec := send(bare.handler, http.MethodPost, "/api/v1/devices/codes", `{"name":"Counter","kind":"terminal"}`, bareOwner)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"app_version":""`) || strings.Contains(rec.Body.String(), "app_url") {
		t.Errorf("a build without the app: %d %s", rec.Code, rec.Body)
	}
}
