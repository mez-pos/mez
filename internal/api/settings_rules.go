package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// rulesRequest is how the venue runs its shifts and tips. day_start is a 24
// hour time like "04:00"; variance_threshold is minor units; tip_role_weights
// is keyed by role name.
type rulesRequest struct {
	DayStart          string           `json:"day_start"`
	VarianceThreshold int64            `json:"variance_threshold"`
	TipRule           string           `json:"tip_rule"`
	TipRoleWeights    map[string]int64 `json:"tip_role_weights"`
	TicketLateMinutes int              `json:"ticket_late_minutes"`
}

// rulesResponse is the rules as they stand. roles are the names a weight may
// use; open_shift_day is the trading day of the shift now open, which keeps
// it when day_start changes, or "" when none is open.
type rulesResponse struct {
	rulesRequest
	Currency     string   `json:"currency"`
	Roles        []string `json:"roles"`
	OpenShiftDay string   `json:"open_shift_day"`
}

func (s *Server) mountRulesSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/rules", s.readRules)
	handle("PUT /api/v1/settings/rules", s.updateRules)
	mux.HandleFunc("/api/v1/settings/rules", s.methodNotAllowed)
}

func (s *Server) readRules(w http.ResponseWriter, r *http.Request) {
	s.writeRules(w, r)
}

func (s *Server) updateRules(w http.ResponseWriter, r *http.Request) {
	var req rulesRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.settings.UpdateRules(r.Context(), Meta(r).VenueID, settings.RulesInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeRules(w, r)
}

// writeRules answers with the rules as they now stand.
func (s *Server) writeRules(w http.ResponseWriter, r *http.Request) {
	n, err := s.settings.Rules(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, rulesResponse{
		rulesRequest: rulesRequest(n.RulesInput),
		Currency:     n.Currency, Roles: n.Roles, OpenShiftDay: n.OpenShiftDay,
	})
}
