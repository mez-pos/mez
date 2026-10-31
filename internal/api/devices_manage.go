package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/device/discover"
	"github.com/mez-pos/mez/internal/domain/qr"
)

// The device management error codes (phase 10, Task 6k).
const (
	CodeDeviceNotFound ErrorCode = "device_not_found"
	// CodeCannotRevokeSelf refuses revoking the device making the request.
	CodeCannotRevokeSelf ErrorCode = "cannot_revoke_self"
	// CodeDeviceUnpaired refuses renaming a device that has been revoked.
	CodeDeviceUnpaired ErrorCode = "device_unpaired"
)

// deviceRecordResponse is a paired device as the back office lists it. last_seen_at is
// absent for a device not heard from since it was paired; revoked_at for one in
// service. this is true for the device that made the request.
type deviceRecordResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Kind       auth.DeviceKind `json:"kind"`
	PairedAt   time.Time       `json:"paired_at"`
	LastSeenAt *time.Time      `json:"last_seen_at,omitempty"`
	RevokedAt  *time.Time      `json:"revoked_at,omitempty"`
	This       bool            `json:"this"`
	// Cloud is the device mez Cloud signs the owner in on (ADR-323).
	Cloud bool `json:"cloud"`
}

// waitingCodeResponse is a pairing code made and not yet used: a device on its
// way. The code itself is never shown again.
type waitingCodeResponse struct {
	Name      string          `json:"name"`
	Kind      auth.DeviceKind `json:"kind"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// deviceListResponse is the venue's devices, in service first, and the codes
// still waiting to be used.
type deviceListResponse struct {
	Devices []deviceRecordResponse `json:"devices"`
	Waiting []waitingCodeResponse  `json:"waiting"`
}

// renameDeviceRequest is the body of POST /api/v1/devices/{id}/rename.
type renameDeviceRequest struct {
	Name string `json:"name"`
}

// mountDeviceManagement adds the devices screen's routes. They are not the
// literal routes of /api/v1/devices/..., so the per-device actions are
// sub-paths: a method-less catch-all on /devices/pair would collide with a
// wildcard under /devices/{id}.
func (s *Server) mountDeviceManagement(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermDeviceManage, h))))
	}
	handle("GET /api/v1/devices", s.listDevices)
	mux.HandleFunc("/api/v1/devices", s.methodNotAllowed)
	handle("POST /api/v1/devices/{id}/rename", s.renameDevice)
	handle("POST /api/v1/devices/{id}/revoke", s.revokeDevice)
	if s.addresses != nil {
		handle("GET /api/v1/devices/addresses", s.serverAddresses)
	}
}

// WithNamedAddress tells the API the address the server answers for by
// name on the venue's network (ADR-379), "http://mez.local:7777", or "" when
// it is not announcing one. Read when asked, since the announcer may have
// failed to start.
func WithNamedAddress(named func() string) Option {
	return func(s *Server) { s.namedAddress = named }
}

// namedURLFor is the named address with the kind's path, or "".
func (s *Server) namedURLFor(kind auth.DeviceKind) string {
	if s.namedAddress == nil {
		return ""
	}
	base := strings.TrimSuffix(s.namedAddress(), "/")
	if base == "" {
		return ""
	}
	path, ok := appPath(kind)
	if !ok {
		return ""
	}
	return base + path
}

// WithAddresses tells the API where on the venue's network this server is
// (ADR-269, ADR-271): each address a second device could open, the real
// adapters' first. Without it the endpoint is not mounted.
func WithAddresses(addresses func() ([]discover.Address, error)) Option {
	return func(s *Server) { s.addresses = addresses }
}

// appPaths is where each kind of device opens its app, under the server's
// address. The kitchen display is the terminal's bundle at its own path
// (ADR-091); the back office is mounted at its own (web/web.go).
var appPaths = []struct {
	kind auth.DeviceKind
	path string
}{
	{auth.KindTerminal, ""},
	{auth.KindKitchen, "/kitchen"},
	{auth.KindBackOffice, "/backoffice/"},
}

// addressesResponse is where to point a device that is being paired: each
// address this server can be opened at from the venue's network, real
// adapters first. Empty when it listens on this computer alone.
type addressesResponse struct {
	// Named is what each kind of device opens by the name the server answers
	// for (ADR-379), "http://mez.local:7777", with its QR code: offered first,
	// since it outlives an address change (SET-1). Empty when no name is
	// announced.
	Named     []addressAppResponse `json:"named"`
	Addresses []addressResponse    `json:"addresses"`
}

// addressResponse is one address, by the adapter it is on, and what each
// kind of device opens there.
type addressResponse struct {
	// Adapter is the adapter's name as the operating system shows it, such
	// as "Wi-Fi"; empty when it is not known.
	Adapter string `json:"adapter"`
	// Virtual is true for an adapter that looks like a virtual machine's or
	// a VPN's, which a tablet is unlikely to be on.
	Virtual bool                 `json:"virtual"`
	Apps    []addressAppResponse `json:"apps"`
}

// addressAppResponse is the address one kind of device opens, and that
// address as a QR code for its camera: an SVG, as the table codes are.
type addressAppResponse struct {
	Kind auth.DeviceKind `json:"kind"`
	URL  string          `json:"url"`
	SVG  string          `json:"svg"`
}

// appPath is where a kind of device opens its app under the server's
// address, or "" for a kind that has none.
func appPath(kind auth.DeviceKind) (string, bool) {
	for _, app := range appPaths {
		if app.kind == kind {
			return app.path, true
		}
	}
	return "", false
}

// pairingPlaces is where a device of kind opens mez to redeem code: each
// address this server has on the venue's network, the real adapters' first,
// as a QR code that carries the code in the address's fragment (ADR-367).
// The fragment never reaches a server: the pairing screen reads it and
// types the code in. Nil when the server was built without its addresses,
// and empty when it knows none a second device can reach; either way the
// code is still typed by hand.
func (s *Server) pairingPlaces(r *http.Request, kind auth.DeviceKind, code string) []pairingPlaceResponse {
	if s.addresses == nil {
		return nil
	}
	path, ok := appPath(kind)
	if !ok {
		return nil
	}
	found, err := s.addresses()
	if err != nil {
		s.log.Warn("reading this server's addresses failed", "request_id", RequestID(r.Context()), "error", err)
		return []pairingPlaceResponse{}
	}
	out := make([]pairingPlaceResponse, 0, len(found))
	for _, a := range found {
		url := strings.TrimSuffix(a.URL, "/") + path
		scan := url
		if path == "" {
			scan += "/"
		}
		scan += "#code=" + code
		place := pairingPlaceResponse{Adapter: a.Adapter, Virtual: a.Virtual, URL: url}
		if drawn, err := qr.Encode([]byte(scan)); err != nil {
			s.log.Warn("drawing a pairing address as a QR failed", "request_id", RequestID(r.Context()), "error", err)
		} else {
			place.SVG = drawn.SVG(qrQuiet)
		}
		out = append(out, place)
	}
	return out
}

// serverAddresses answers whoever is pairing a device with where to open
// it. It is behind device.manage with the code it goes beside.
func (s *Server) serverAddresses(w http.ResponseWriter, r *http.Request) {
	found, err := s.addresses()
	if err != nil {
		// The code still pairs a device; only the address must be read off
		// the computer instead. An empty list says that.
		s.log.Warn("reading this server's addresses failed", "request_id", RequestID(r.Context()), "error", err)
		found = nil
	}
	out := addressesResponse{Named: []addressAppResponse{}, Addresses: make([]addressResponse, 0, len(found))}
	// The name does not depend on the adapters: it is offered even when they
	// could not be read.
	if s.namedAddress != nil {
		if base := strings.TrimSuffix(s.namedAddress(), "/"); base != "" {
			out.Named = s.appsAt(r, base)
		}
	}
	for _, a := range found {
		out.Addresses = append(out.Addresses, addressResponse{Adapter: a.Adapter, Virtual: a.Virtual, Apps: s.appsAt(r, strings.TrimSuffix(a.URL, "/"))})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, out)
}

// appsAt is what each kind of device opens under base, an address with no
// trailing slash, and that address as a QR code.
func (s *Server) appsAt(r *http.Request, base string) []addressAppResponse {
	apps := make([]addressAppResponse, 0, len(appPaths))
	for _, app := range appPaths {
		url := base + app.path
		code, err := qr.Encode([]byte(url))
		if err != nil {
			// An address is a few dozen bytes; a code holds two thousand.
			// Without a picture the address is still typed.
			s.log.Warn("drawing an address as a QR failed", "request_id", RequestID(r.Context()), "error", err)
			apps = append(apps, addressAppResponse{Kind: app.kind, URL: url})
			continue
		}
		apps = append(apps, addressAppResponse{Kind: app.kind, URL: url, SVG: code.SVG(qrQuiet)})
	}
	return apps
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	roster, err := s.devices.Roster(r.Context(), dev.VenueID)
	if err != nil {
		s.writeDeviceError(w, r, err)
		return
	}
	out := deviceListResponse{
		Devices: make([]deviceRecordResponse, len(roster.Devices)),
		Waiting: make([]waitingCodeResponse, len(roster.Waiting)),
	}
	for i, d := range roster.Devices {
		out.Devices[i] = deviceRecordResponse{
			ID: d.PublicID, Name: d.Name, Kind: d.Kind, PairedAt: d.PairedAt, LastSeenAt: d.LastSeenAt, RevokedAt: d.RevokedAt,
			This: d.ID == dev.ID, Cloud: d.Cloud,
		}
	}
	for i, c := range roster.Waiting {
		out.Waiting[i] = waitingCodeResponse(c)
	}
	// Who is in service and when each was last heard from: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) renameDevice(w http.ResponseWriter, r *http.Request) {
	var req renameDeviceRequest
	if !s.decode(w, r, &req) {
		return
	}
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	if err := s.devices.Rename(r.Context(), dev.VenueID, r.PathValue("id"), req.Name, session.Staff.ID); err != nil {
		s.writeDeviceError(w, r, err)
		return
	}
	s.listDevices(w, r)
}

func (s *Server) revokeDevice(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	if err := s.devices.RevokeByPublicID(r.Context(), dev.VenueID, r.PathValue("id"), dev.ID, session.Staff.ID); err != nil {
		s.writeDeviceError(w, r, err)
		return
	}
	s.listDevices(w, r)
}

func (s *Server) writeDeviceError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *auth.InvalidDevice
	switch {
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidRequest, invalid.Message, invalid.Field)
	case errors.Is(err, auth.ErrDeviceNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeDeviceNotFound, "No such device.")
	case errors.Is(err, auth.ErrRevokeSelf):
		writeError(w, s.log, http.StatusConflict, CodeCannotRevokeSelf,
			"You are using this device: revoke it from another back office, or leave it.")
	case errors.Is(err, auth.ErrDeviceRevoked):
		writeError(w, s.log, http.StatusConflict, CodeDeviceUnpaired, "That device has been revoked, so it cannot be renamed.")
	default:
		s.internalError(w, r, "a device request failed", err)
	}
}
