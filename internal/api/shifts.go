package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/fsm"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
	svcshift "github.com/byjoellobo/nofiltr-pos/internal/service/shift"
)

// Shifts is what the API needs of the venue's shifts (phase 09, Task 1b).
// *shift.Service is the implementation.
type Shifts interface {
	Current(ctx context.Context, venueID int64) (svcshift.Shift, error)
	Open(ctx context.Context, meta svcshift.Meta, float shift.Count) (svcshift.Shift, error)
	MoveCash(ctx context.Context, meta svcshift.Meta, shiftID int64, m shift.Movement) (svcshift.Shift, error)
	Close(ctx context.Context, meta svcshift.Meta, shiftID int64) (svcshift.Shift, error)
}

// WithShifts mounts the shift endpoints. They need WithDevices and
// WithStaff.
func WithShifts(sh Shifts) Option {
	return func(s *Server) { s.shifts = sh }
}

// The shift error codes.
const (
	CodeNoOpenShift  ErrorCode = "no_open_shift"
	CodeShiftOpen    ErrorCode = "shift_open"
	CodeShiftClosed  ErrorCode = "shift_closed"
	CodeShiftUnknown ErrorCode = "shift_not_found"
	CodeInvalidCount ErrorCode = "invalid_count"
	CodeInvalidCash  ErrorCode = "invalid_cash_movement"
)

// openShiftRequest opens the shift with the float counted into the
// drawer. Total is the float; the notes and coins, when given, must add up
// to it.
type openShiftRequest struct {
	Total         amount               `json:"total"`
	Denominations []shift.Denomination `json:"denominations,omitempty"`
}

// cashRequest is cash put in or taken out, with a manager's override: the
// cashier at the drawer cannot authorise their own payout.
type cashRequest struct {
	Kind     shift.MovementKind `json:"kind"`
	Amount   amount             `json:"amount"`
	Reason   string             `json:"reason"`
	Override Override           `json:"override"`
}

type shiftEventResponse struct {
	ID           int64              `json:"id"`
	Type         svcshift.EventType `json:"type"`
	Amount       money.Amount       `json:"amount"`
	Reason       string             `json:"reason"`
	ActorID      int64              `json:"actor_id"`
	AuthorisedBy int64              `json:"authorised_by"`
	At           time.Time          `json:"at"`
}

// shiftResponse is a shift as a till may see it. It has no expected drawer
// figure, on purpose: Task 2's blind count is entered before anyone is
// shown what the drawer should hold, and a figure here would give it away.
type shiftResponse struct {
	ID         int64                `json:"id"`
	PublicID   string               `json:"public_id"`
	Status     shift.Status         `json:"status"`
	TradingDay string               `json:"trading_day"`
	OpenedBy   int64                `json:"opened_by"`
	OpenedAt   time.Time            `json:"opened_at"`
	ClosedBy   int64                `json:"closed_by,omitempty"`
	ClosedAt   *time.Time           `json:"closed_at,omitempty"`
	Float      money.Amount         `json:"float"`
	Events     []shiftEventResponse `json:"events"`
}

// currentShiftResponse answers "is the till open?": the open shift, or
// none. No shift is not an error; it is how every venue starts the day.
type currentShiftResponse struct {
	Shift *shiftResponse `json:"shift"`
}

func (s *Server) mountShifts(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.shifts == nil {
		return
	}
	handle := func(pattern string, p auth.Permission, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(p, h))))
	}
	// Anyone who sells needs to know whether the till is open.
	handle("GET /api/v1/shift", auth.PermOrderOpen, s.currentShift)
	handle("POST /api/v1/shift/open", auth.PermShiftManage, s.openShift)
	// Whoever takes payments handles the drawer; the movement itself is
	// authorised by a manager's override for cash.move.
	handle("POST /api/v1/shift/{id}/cash", auth.PermPaymentTake, s.moveCash)
	handle("POST /api/v1/shift/{id}/close", auth.PermShiftManage, s.closeShift)
	mux.HandleFunc("/api/v1/shift", s.methodNotAllowed)
}

func (s *Server) currentShift(w http.ResponseWriter, r *http.Request) {
	sh, err := s.shifts.Current(r.Context(), Meta(r).VenueID)
	if errors.Is(err, svcshift.ErrNoOpenShift) {
		writeJSON(w, s.log, http.StatusOK, currentShiftResponse{})
		return
	}
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	out := toShiftResponse(sh)
	writeJSON(w, s.log, http.StatusOK, currentShiftResponse{Shift: &out})
}

func (s *Server) openShift(w http.ResponseWriter, r *http.Request) {
	var req openShiftRequest
	if !s.decode(w, r, &req) {
		return
	}
	float, err := req.count()
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	sh, err := s.shifts.Open(r.Context(), shiftMeta(r), float)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, toShiftResponse(sh))
}

// count reads the float: the total, checked against its notes and coins
// when they are given.
func (req openShiftRequest) count() (shift.Count, error) {
	total := toMoney(req.Total)
	if len(req.Denominations) == 0 {
		return shift.LumpSum(shift.CountOpening, total)
	}
	c := shift.Count{Kind: shift.CountOpening, Denominations: req.Denominations, Total: total}
	if err := c.Validate(total.Currency); err != nil {
		return shift.Count{}, err
	}
	return shift.NewCount(shift.CountOpening, total.Currency, req.Denominations)
}

func (s *Server) moveCash(w http.ResponseWriter, r *http.Request) {
	var req cashRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	by, ok := s.authorise(w, r, req.Override, auth.PermCashMove)
	if !ok {
		return
	}
	sh, err := s.shifts.MoveCash(r.Context(), shiftMeta(r), id, shift.Movement{
		Kind: req.Kind, Amount: toMoney(req.Amount), Reason: req.Reason, AuthorisedBy: by.ID,
	})
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toShiftResponse(sh))
}

func (s *Server) closeShift(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	sh, err := s.shifts.Close(r.Context(), shiftMeta(r), id)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toShiftResponse(sh))
}

// shiftMeta is Meta for the shift service.
func shiftMeta(r *http.Request) svcshift.Meta {
	m := Meta(r)
	return svcshift.Meta{VenueID: m.VenueID, ActorID: m.ActorID, DeviceID: m.DeviceID, IdempotencyKey: m.IdempotencyKey}
}

func toShiftResponse(sh svcshift.Shift) shiftResponse {
	out := shiftResponse{
		ID: sh.ID, PublicID: sh.PublicID, Status: sh.Status, TradingDay: sh.TradingDay,
		OpenedBy: sh.OpenedBy, OpenedAt: sh.OpenedAt.UTC(), ClosedBy: sh.ClosedBy,
		Float: sh.Float.Total, Events: make([]shiftEventResponse, len(sh.Events)),
	}
	if !sh.ClosedAt.IsZero() {
		closed := sh.ClosedAt.UTC()
		out.ClosedAt = &closed
	}
	for i, e := range sh.Events {
		out.Events[i] = shiftEventResponse{
			ID: e.ID, Type: e.Type, Amount: e.Amount, Reason: e.Reason,
			ActorID: e.ActorID, AuthorisedBy: e.AuthorisedBy, At: e.At.UTC(),
		}
	}
	return out
}

// shiftErrors maps the shift's refusals to a status and a code; the first
// match wins. Anything else is a bug, and a 500 with the detail logged.
var shiftErrors = []struct {
	err    error
	status int
	code   ErrorCode
}{
	{svcshift.ErrShiftNotFound, http.StatusNotFound, CodeShiftUnknown},
	{svcshift.ErrShiftOpen, http.StatusConflict, CodeShiftOpen},
	{svcshift.ErrShiftClosed, http.StatusConflict, CodeShiftClosed},
	{svcshift.ErrNoOpenShift, http.StatusConflict, CodeNoOpenShift},
	{svcshift.ErrIdempotencyKeyReused, http.StatusConflict, CodeIdempotencyKeyReused},
	{svcshift.ErrVenueConfig, http.StatusConflict, CodeVenueNotConfigured},
	{shift.ErrInvalidCount, http.StatusUnprocessableEntity, CodeInvalidCount},
	{shift.ErrInvalidMovement, http.StatusUnprocessableEntity, CodeInvalidCash},
	{svcshift.ErrInvalidCommand, http.StatusBadRequest, CodeInvalidRequest},
	{fsm.ErrInvalidTransition, http.StatusConflict, CodeInvalidTransition},
}

func (s *Server) writeShiftError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range shiftErrors {
		if errors.Is(err, m.err) {
			writeError(w, s.log, m.status, m.code, err.Error())
			return
		}
	}
	s.log.Error("a shift request failed",
		"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
}
