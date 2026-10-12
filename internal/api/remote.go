package api

import (
	"net/http"
	"strings"

	"github.com/byjoellobo/mez/internal/auth"
)

// A request that came through the relay carries this header, set by the
// tunnel client, which also removes any copy that arrived (ADR-307). They are
// internal/tunnel's HeaderVia and ViaRelay, restated because this package
// must not import that one; cmd/mez has the test that the two agree.
const (
	HeaderVia = "X-Mez-Via"
	ViaRelay  = "relay"
)

// CodeRemoteNotAllowed is a request through the relay for something that is
// done at the venue and nowhere else: taking a payment, closing the day,
// using a till. The same request from the venue's own network is not refused.
const CodeRemoteNotAllowed ErrorCode = "remote_not_allowed"

// remoteGuest is the guest app's mount and remoteBundle is where the back
// office's own files are served. Between them they are everything a relayed
// request may reach that is not one of the API routes named below. They are
// web.MountGuest and web.MountBackoffice, restated because this package does
// not import that one; a test here holds them to it. They are constants and
// not read from what happens to be mounted, so that a bundle added later is
// not open to the outside by being served.
const (
	remoteGuest  = "/order/"
	remoteBundle = "/backoffice"
)

// relayed reports whether a request came through the relay. Any value
// counts, and so does more than one: the tunnel writes exactly "relay", and a
// request that says anything else under this name is not one to be given
// the benefit of the doubt.
func relayed(r *http.Request) bool {
	return len(r.Header.Values(HeaderVia)) > 0
}

// remoteAreas are the parts of the API a relayed request may use whole: every
// route beneath each, whatever its method. They are the owner's desk work,
// and each route in them is behind the permission that work needs. A route
// added to one of these later is reachable from outside without a line being
// changed here, which is what is wanted of a menu or a stock route and is
// why nothing that touches money or the room is filed under any of them.
var remoteAreas = []string{
	"/api/v1/menu",
	"/api/v1/stock",
	"/api/v1/people",
	"/api/v1/audit",
	"/api/v1/reports",
}

// remoteRefused are the routes inside remoteAreas that stay at the venue.
var remoteRefused = map[string]struct{}{
	// A PIN locked by wrong guesses is cleared by someone standing in the
	// room, or it is not much of a lock against a guesser outside it.
	"POST /api/v1/people/staff/{id}/unlock": {},
}

// remoteRoutes are the other routes a relayed request may use, one by one,
// exactly as they are registered. Anything not here and not in an area is
// refused: a new route is closed to the outside until someone decides it
// should not be (ADR-308).
var remoteRoutes = map[string]struct{}{
	// Becoming a device, and being one. A code is made at the venue; a lost
	// phone can be unpaired from anywhere.
	"POST /api/v1/devices/pair":        {},
	"GET /api/v1/devices/me":           {},
	"GET /api/v1/devices":              {},
	"POST /api/v1/devices/{id}/revoke": {},

	// Signing in and out.
	"GET /api/v1/staff":               {},
	"POST /api/v1/sessions":           {},
	"GET /api/v1/sessions/current":    {},
	"DELETE /api/v1/sessions/current": {},
	// The owner, vouched for by mez Cloud. It is only ever from away.
	"POST /api/v1/sessions/cloud": {},

	// What every screen reads to draw itself.
	"GET /api/v1/venue":  {},
	"GET /api/v1/health": {},
	"GET /api/v1/tunnel": {},
	"GET /api/v1/setup":  {},

	// The floor plan: areas and tables. Not clearing or releasing a table,
	// which is the till's.
	"GET /api/v1/floor":                      {},
	"GET /api/v1/floor/live":                 {},
	"POST /api/v1/floor/areas":               {},
	"PUT /api/v1/floor/areas/{id}":           {},
	"POST /api/v1/floor/areas/{id}/restore":  {},
	"POST /api/v1/floor/areas/{id}/retire":   {},
	"POST /api/v1/floor/tables":              {},
	"PUT /api/v1/floor/tables/{id}":          {},
	"POST /api/v1/floor/tables/{id}/restore": {},
	"POST /api/v1/floor/tables/{id}/retire":  {},

	// The codes guests scan, which go with the tables.
	"GET /api/v1/table-codes":              {},
	"POST /api/v1/table-codes":             {},
	"GET /api/v1/table-codes/sheet":        {},
	"GET /api/v1/tables/{id}/code/qr":      {},
	"POST /api/v1/tables/{id}/code/renew":  {},
	"POST /api/v1/tables/{id}/code/revoke": {},
	"GET /api/v1/pickup-code/qr":           {},
	"POST /api/v1/pickup-code/renew":       {},
	"POST /api/v1/pickup-code/revoke":      {},

	// Settings: all of them can be read. The receipt, the venue's name and
	// address, the rules and tax can be changed. Where the money goes
	// (payments), where the backup goes and how invoices are numbered are
	// changed at the venue.
	"GET /api/v1/settings":                           {},
	"GET /api/v1/settings/backup":                    {},
	"GET /api/v1/settings/numbering":                 {},
	"POST /api/v1/settings/numbering/preview":        {},
	"GET /api/v1/settings/payments":                  {},
	"GET /api/v1/settings/receipt":                   {},
	"PUT /api/v1/settings/receipt":                   {},
	"POST /api/v1/settings/receipt/preview":          {},
	"GET /api/v1/settings/rules":                     {},
	"PUT /api/v1/settings/rules":                     {},
	"GET /api/v1/settings/tax":                       {},
	"PUT /api/v1/settings/tax":                       {},
	"POST /api/v1/settings/tax-classes":              {},
	"PUT /api/v1/settings/tax-classes/{id}":          {},
	"POST /api/v1/settings/tax-classes/{id}/restore": {},
	"POST /api/v1/settings/tax-classes/{id}/retire":  {},
	"PUT /api/v1/settings/venue":                     {},

	// Reading what was sold: orders, the day's reports and their files.
	// Nothing here changes an order, and a report is not printed from home.
	"GET /api/v1/orders":                     {},
	"GET /api/v1/orders/{id}":                {},
	"GET /api/v1/orders/{id}/events":         {},
	"GET /api/v1/orders/{id}/invoices/{seq}": {},
	"GET /api/v1/z-reports":                  {},
	"GET /api/v1/z-reports/{number}":         {},
	"GET /api/v1/shift/{id}/tips":            {},
	"GET /api/v1/shift/{id}/exports":         {},
	"GET /api/v1/exports/{id}":               {},

	// Which printers there are, so the screen that lists them can open.
	// Adding, testing and searching the network for one are the venue's.
	"GET /api/v1/printers": {},
}

// remoteAllows reports whether a relayed request may use the route
// registered as pattern, "METHOD /path".
func remoteAllows(pattern string) bool {
	if _, refused := remoteRefused[pattern]; refused {
		return false
	}
	if _, ok := remoteRoutes[pattern]; ok {
		return true
	}
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || method == "" {
		// A pattern with no method is one of the catch-alls that answer
		// 405 and 404. From outside those are refused like the rest.
		return false
	}
	for _, area := range remoteAreas {
		if path == area || strings.HasPrefix(path, area+"/") {
			return true
		}
	}
	return false
}

// remoteGate is the last thing before the routes. A request from the
// venue's own network passes untouched. One that came through the relay
// reaches the guest app, the back office's files, and the API routes
// remoteAllows names; for anything else it is told, in the usual envelope,
// that this is done at the venue (ADR-308).
//
// It asks the mux which route the request would take and judges that, so
// what is judged is what would run: there is no second reading of the path
// here for the two to disagree about.
func (s *Server) remoteGate(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !relayed(r) {
			mux.ServeHTTP(w, r)
			return
		}
		if info := infoFrom(r.Context()); info != nil {
			info.via = ViaRelay
		}
		// Said back, so the back office can tell it is being used from
		// away and stop waiting for a stream the relay cannot carry.
		w.Header().Set(HeaderVia, ViaRelay)

		// The guest app first: it is nearly every relayed request there is,
		// and it was reachable before any of this existed (ADR-202).
		path := r.URL.Path
		if strings.HasPrefix(path, remoteGuest) || (s.guestMount != "" && strings.HasPrefix(path, s.guestMount)) {
			mux.ServeHTTP(w, r)
			return
		}
		if path == remoteBundle || strings.HasPrefix(path, remoteBundle+"/") {
			mux.ServeHTTP(w, r)
			return
		}
		if _, pattern := mux.Handler(r); remoteAllows(pattern) {
			mux.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, s.log, http.StatusForbidden, CodeRemoteNotAllowed,
			"This can only be done at the venue.")
	})
}

// remoteDevice refuses a relayed request from a device that is not a back
// office. A till's or a kitchen display's token opens nothing from outside
// the building, whoever is holding it. It answers the request itself and
// reports whether the request may go on.
func (s *Server) remoteDevice(w http.ResponseWriter, r *http.Request, dev auth.Device) bool {
	if !relayed(r) || dev.Kind == auth.KindBackOffice {
		return true
	}
	writeError(w, s.log, http.StatusForbidden, CodeRemoteNotAllowed,
		"This device can only be used at the venue.")
	return false
}

// remoteDesk are the permissions that make someone the back office's to
// use from outside: holding any one is enough. A cashier or a waiter holds
// none, so a right PIN in the wrong hands outside the building is refused.
var remoteDesk = []auth.Permission{
	auth.PermMenuEdit, auth.PermSettingsEdit, auth.PermReportsView, auth.PermStockManage,
}

// remoteStaff reports whether a staff member holding these permissions may
// use the back office through the relay.
func remoteStaff(held auth.PermissionSet) bool {
	for _, p := range remoteDesk {
		if held.Has(p) {
			return true
		}
	}
	return false
}

// refuseRemoteStaff answers a relayed request from someone who works the
// floor and not the desk.
func (s *Server) refuseRemoteStaff(w http.ResponseWriter) {
	writeError(w, s.log, http.StatusForbidden, CodeRemoteNotAllowed,
		"Only someone who manages the venue can sign in away from it.")
}
