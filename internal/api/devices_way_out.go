package api

import (
	"net/http"

	"github.com/mez-pos/mez/internal/auth"
)

// wayOutResponse is who opened a tablet's way out.
type wayOutResponse struct {
	OpenedBy staffResponse `json:"opened_by"`
}

// mountWayOut serves a tablet's way out of mez (ADR-419): who may open it,
// and opening it with their PIN, which is checked here as the till's
// overrides are. Both are the venue's: neither is among the relay's routes.
func (s *Server) mountWayOut(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil {
		return
	}
	mux.Handle("GET /api/v1/devices/me/way-out", s.requireDevice(http.HandlerFunc(s.wayOutStaff)))
	mux.Handle("POST /api/v1/devices/me/way-out", s.requireDevice(http.HandlerFunc(s.openWayOut)))
	mux.HandleFunc("/api/v1/devices/me/way-out", s.methodNotAllowed)
}

// wayOutStaff is who may let this tablet out: the staff whose role may
// manage devices.
func (s *Server) wayOutStaff(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	staff, err := s.staff.WithPermission(r.Context(), dev.VenueID, auth.PermDeviceManage)
	if err != nil {
		s.internalError(w, r, "listing who may let a tablet out failed", err)
		return
	}
	out := make([]staffResponse, len(staff))
	for i, member := range staff {
		out[i] = toStaffResponse(member)
	}
	writeJSON(w, s.log, http.StatusOK, staffListResponse{Staff: out})
}

// openWayOut checks the PIN of the staff member named, for the permission
// to manage devices, through the same gauntlet as every override: lockout
// included, and no session opened. The app lets the tablet out on a 200.
func (s *Server) openWayOut(w http.ResponseWriter, r *http.Request) {
	var o Override
	if err := decodeJSON(w, r, &o); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	by, ok := s.authorise(w, r, o, auth.PermDeviceManage)
	if !ok {
		return
	}
	dev, _ := DeviceFrom(r.Context())
	s.log.Info("a tablet's way out was opened",
		"request_id", RequestID(r.Context()), "venue_id", dev.VenueID, "device_id", dev.ID, "actor_id", by.ID)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, wayOutResponse{OpenedBy: toStaffResponse(by)})
}
