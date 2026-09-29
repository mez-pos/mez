package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tips"
	svcshift "github.com/byjoellobo/nofiltr-pos/internal/service/shift"
)

// Tips is what the API needs of a shift's tip pool (ADR-101).
// *shift.Service is the implementation.
type Tips interface {
	Tips(ctx context.Context, venueID, shiftID int64) (svcshift.TipPool, error)
	PoolTips(ctx context.Context, meta svcshift.Meta, shiftID int64, members []svcshift.PoolMember) (svcshift.TipPool, error)
}

// The tip pool's error codes.
const (
	CodeAlreadyPooled  ErrorCode = "already_pooled"
	CodeNotStaff       ErrorCode = "not_staff"
	CodeInvalidTipPool ErrorCode = "invalid_tip_pool"
)

type poolRequest struct {
	Members []svcshift.PoolMember `json:"members"`
}

type tipMemberResponse struct {
	StaffID string       `json:"staff_id"`
	Name    string       `json:"name"`
	Role    string       `json:"role"`
	InPool  bool         `json:"in_pool"`
	Minutes int64        `json:"minutes"`
	Weight  int64        `json:"weight"`
	Taken   money.Amount `json:"taken"`
	Share   money.Amount `json:"share"`
}

// tipPoolResponse is a shift's tips, pooled or a preview: every staff
// member's report line.
type tipPoolResponse struct {
	ShiftID  int64               `json:"shift_id"`
	Pooled   bool                `json:"pooled"`
	Rule     tips.Rule           `json:"rule"`
	Total    money.Amount        `json:"total"`
	Members  []tipMemberResponse `json:"members"`
	PooledBy int64               `json:"pooled_by,omitempty"`
	PooledAt *time.Time          `json:"pooled_at,omitempty"`
	Problem  string              `json:"problem,omitempty"`
}

func (s *Server) mountTips(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.tips == nil {
		return
	}
	handle := func(pattern string, p auth.Permission, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(p, h))))
	}
	handle("GET /api/v1/shift/{id}/tips", auth.PermReportsView, s.readTips)
	handle("POST /api/v1/shift/{id}/tips", auth.PermShiftManage, s.poolTips)
}

func (s *Server) readTips(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := s.tips.Tips(r.Context(), Meta(r).VenueID, id)
	s.writeTips(w, r, p, err)
}

func (s *Server) poolTips(w http.ResponseWriter, r *http.Request) {
	var req poolRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	if req.Members == nil {
		req.Members = []svcshift.PoolMember{}
	}
	p, err := s.tips.PoolTips(r.Context(), shiftMeta(r), id, req.Members)
	s.writeTips(w, r, p, err)
}

func (s *Server) writeTips(w http.ResponseWriter, r *http.Request, p svcshift.TipPool, err error) {
	switch {
	case errors.Is(err, svcshift.ErrAlreadyPooled):
		writeError(w, s.log, http.StatusConflict, CodeAlreadyPooled, "This shift's tips are already pooled.")
		return
	case errors.Is(err, svcshift.ErrNotStaff):
		writeError(w, s.log, http.StatusUnprocessableEntity, CodeNotStaff, err.Error())
		return
	case errors.Is(err, tips.ErrInvalidPool):
		writeError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidTipPool, err.Error())
		return
	case err != nil:
		s.writeZError(w, r, err)
		return
	}
	out := tipPoolResponse{
		ShiftID: p.ShiftID, Pooled: p.Pooled, Rule: p.Rule, Total: p.Total,
		Members: make([]tipMemberResponse, len(p.Members)), PooledBy: p.PooledBy, Problem: p.Problem,
	}
	if !p.PooledAt.IsZero() {
		at := p.PooledAt.UTC()
		out.PooledAt = &at
	}
	for i, m := range p.Members {
		out.Members[i] = tipMemberResponse{
			StaffID: m.PublicID, Name: m.Name, Role: m.Role, InPool: m.InPool, Minutes: m.Minutes,
			Weight: m.Weight, Taken: m.Taken, Share: m.Share,
		}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}
