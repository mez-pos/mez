package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
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

// WithAddresses tells the API where on the venue's network this server is
// (ADR-269): each address a second device could open, most likely first.
// Without it the endpoint is not mounted.
func WithAddresses(urls func() ([]string, error)) Option {
	return func(s *Server) { s.addresses = urls }
}

// addressesResponse is where to point a device that is being paired: the
// addresses this server can be opened at from the venue's network, most
// likely first. Empty when it listens on this computer alone.
type addressesResponse struct {
	URLs []string `json:"urls"`
}

// serverAddresses answers whoever is pairing a device with where to open
// it. It is behind device.manage with the code it goes beside.
func (s *Server) serverAddresses(w http.ResponseWriter, r *http.Request) {
	urls, err := s.addresses()
	if err != nil {
		// The code still pairs a device; only the address must be read off
		// the computer instead. An empty list says that.
		s.log.Warn("reading this server's addresses failed", "request_id", RequestID(r.Context()), "error", err)
		urls = nil
	}
	if urls == nil {
		urls = []string{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, addressesResponse{URLs: urls})
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
			This: d.ID == dev.ID,
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
		s.log.Error("a device request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", s.loggedPath(r), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
