package api

import (
	"bytes"
	"net/http"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/device/tabletapp"
	"github.com/mez-pos/mez/internal/domain/qr"
)

// The tablet app (ADR-421): the mez app for Android tablets, carried in this
// binary by a release and served from the venue's own network to a tablet
// that has never seen mez. Neither route needs a token, since such a tablet
// has none, and neither is given through the relay.
//
//	GET /app/mez.apk   the package itself
//	GET /api/v1/app    its version, size and digest
const tabletAppPath = "/app/mez.apk"

// CodeAppNotBuilt is either route on a build that carries no app.
const CodeAppNotBuilt ErrorCode = "app_not_built"

// appNotBuilt is what such a build says.
const appNotBuilt = "This mez was built without the tablet app."

// apkType is the package's media type, which Android's browsers download
// and offer to install.
const apkType = "application/vnd.android.package-archive"

// WithTabletApp gives the API the app this binary carries; nil, or no
// option, is a build without one.
func WithTabletApp(app *tabletapp.App) Option {
	return func(s *Server) { s.tabletApp = app }
}

// tabletAppResponse is GET /api/v1/app: what the app is, for a tablet to
// tell whether the PC carries a newer one than it has (Task 8).
type tabletAppResponse struct {
	// Version is the version of mez the app was built with.
	Version string `json:"version"`
	// VersionCode is Android's, higher for each release (ADR-421).
	VersionCode int64 `json:"version_code"`
	// Size is the package's, in bytes.
	Size int `json:"size"`
	// SHA256 is the package's digest, in hex.
	SHA256 string `json:"sha256"`
	// Path is where the package is, under this server's address.
	Path string `json:"path"`
}

func (s *Server) mountTabletApp(mux *http.ServeMux) {
	mux.HandleFunc("GET "+tabletAppPath, s.tabletAppPackage)
	mux.HandleFunc(tabletAppPath, s.methodNotAllowed)
	mux.HandleFunc("GET /api/v1/app", s.tabletAppVersion)
	mux.HandleFunc("/api/v1/app", s.methodNotAllowed)
}

// tabletAppPackage serves the package: named with its version, its digest
// the ETag, and ranges served, so a download broken by poor wifi resumes.
func (s *Server) tabletAppPackage(w http.ResponseWriter, r *http.Request) {
	app := s.tabletApp
	if app == nil {
		writeError(w, s.log, http.StatusNotFound, CodeAppNotBuilt, appNotBuilt)
		return
	}
	h := w.Header()
	h.Set("Content-Type", apkType)
	// The version is checked to need no quoting when the app is read.
	h.Set("Content-Disposition", `attachment; filename="mez-`+app.Version+`.apk"`)
	h.Set("ETag", `"`+app.SHA256+`"`)
	// Asked again each time, and answered 304 when it is the same package.
	h.Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(app.APK))
}

func (s *Server) tabletAppVersion(w http.ResponseWriter, r *http.Request) {
	app := s.tabletApp
	if app == nil {
		writeError(w, s.log, http.StatusNotFound, CodeAppNotBuilt, appNotBuilt)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, s.log, http.StatusOK, tabletAppResponse{
		Version: app.Version, VersionCode: app.Code, Size: len(app.APK), SHA256: app.SHA256, Path: tabletAppPath,
	})
}

// tabletAppVersionName is the version of the app this build carries, or ""
// for none, as a pairing code tells it.
func (s *Server) tabletAppVersionName() string {
	if s.tabletApp == nil {
		return ""
	}
	return s.tabletApp.Version
}

// tabletAppAt adds to a place a device is paired at the app's address there,
// and its QR, for a till or a kitchen display, which are the tablets. base is
// the place's address with no trailing slash.
func (s *Server) tabletAppAt(r *http.Request, place *pairingPlaceResponse, kind auth.DeviceKind, base string) {
	if s.tabletApp == nil || (kind != auth.KindTerminal && kind != auth.KindKitchen) {
		return
	}
	place.AppURL = base + tabletAppPath
	drawn, err := qr.Encode([]byte(place.AppURL))
	if err != nil {
		// The address is still typed, or the app is had another way.
		s.log.Warn("drawing the tablet app's address as a QR failed", "request_id", RequestID(r.Context()), "error", err)
		return
	}
	place.AppSVG = drawn.SVG(qrQuiet)
}
