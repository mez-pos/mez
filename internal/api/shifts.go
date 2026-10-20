package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/domain/fsm"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/shift"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// Shifts is what the API needs of the venue's shifts (phase 09, Task 1b).
// *shift.Service is the implementation.
type Shifts interface {
	Current(ctx context.Context, venueID int64) (svcshift.Shift, error)
	Open(ctx context.Context, meta svcshift.Meta, float shift.Count) (svcshift.Shift, error)
	MoveCash(ctx context.Context, meta svcshift.Meta, shiftID int64, m shift.Movement) (svcshift.Shift, error)
	Close(ctx context.Context, meta svcshift.Meta, shiftID int64) (svcshift.Shift, error)
	Count(ctx context.Context, meta svcshift.Meta, shiftID int64, counted shift.Count) (svcshift.Shift, error)
	ExplainVariance(ctx context.Context, meta svcshift.Meta, shiftID int64, reason string) (svcshift.Shift, error)
}

// MyShifts says what one person did in the open shift (T08).
type MyShifts interface {
	Mine(ctx context.Context, venueID, staffID int64) (svcshift.MyShift, error)
}

// WithShifts mounts the shift endpoints. They need WithDevices and
// WithStaff. A Shifts that also keeps Z-reports, as *shift.Service does,
// mounts those too.
func WithShifts(sh Shifts) Option {
	return func(s *Server) {
		s.shifts = sh
		if z, ok := sh.(ZReports); ok {
			s.zreports = z
		}
		if t, ok := sh.(Tips); ok {
			s.tips = t
		}
		if m, ok := sh.(MyShifts); ok {
			s.myShifts = m
		}
		if e, ok := sh.(Exports); ok {
			s.exports = e
		}
	}
}

// The shift error codes.
const (
	CodeNoOpenShift  ErrorCode = "no_open_shift"
	CodeShiftOpen    ErrorCode = "shift_open"
	CodeShiftClosed  ErrorCode = "shift_closed"
	CodeShiftUnknown ErrorCode = "shift_not_found"
	CodeInvalidCount ErrorCode = "invalid_count"
	CodeInvalidCash  ErrorCode = "invalid_cash_movement"

	// The blind count (ADR-095).
	CodeAlreadyCounted      ErrorCode = "already_counted"
	CodeNotCounted          ErrorCode = "not_counted"
	CodeDrawerCounted       ErrorCode = "drawer_counted"
	CodeVarianceUnexplained ErrorCode = "variance_unexplained"
	CodeAlreadyExplained    ErrorCode = "already_explained"
)

// countRequest is cash counted in the drawer: the float at open, the
// blind count at close. Total is what was counted; the notes and coins,
// when given, must add up to it.
type countRequest struct {
	Total         amount               `json:"total"`
	Denominations []shift.Denomination `json:"denominations,omitempty"`
}

// openShiftRequest opens the shift with the float counted into the drawer.
type openShiftRequest = countRequest

// reasonRequest is why the drawer was over or short.
type reasonRequest struct {
	Reason string `json:"reason"`
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

// reconciliationResponse is the count set against what was expected.
type reconciliationResponse struct {
	Expected       money.Amount `json:"expected"`
	Counted        money.Amount `json:"counted"`
	Variance       money.Amount `json:"variance"` // over is positive, short negative
	Threshold      money.Amount `json:"threshold"`
	Over           bool         `json:"over"`
	Short          bool         `json:"short"`
	ReasonRequired bool         `json:"reason_required"`
	CountedBy      int64        `json:"counted_by"`
	CountedAt      time.Time    `json:"counted_at"`
}

type varianceReasonResponse struct {
	Reason      string    `json:"reason"`
	ExplainedBy int64     `json:"explained_by"`
	ExplainedAt time.Time `json:"explained_at"`
}

// shiftResponse is a shift as a till may see it. Before the blind count it
// carries no expected figure and no cash sales, on purpose (ADR-095): the
// count is entered before anyone is shown what the drawer should hold.
// After it, Reconciliation reveals it.
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
	// Counted is whether the drawer has had its blind count.
	Counted bool `json:"counted"`
	// Reconciliation is absent until the drawer is counted.
	Reconciliation *reconciliationResponse `json:"reconciliation,omitempty"`
	Reason         *varianceReasonResponse `json:"reason,omitempty"`
	// ZReport is the number of the shift's Z-report as it now stands;
	// absent while the shift is open.
	ZReport int64 `json:"z_report,omitempty"`
}

// currentShiftResponse answers "is the till open?": the open shift, or
// none. No shift is not an error; it is how every venue starts the day.
type currentShiftResponse struct {
	Shift *shiftResponse `json:"shift"`
	// Openers is who can open a shift, named only while none is open, for a
	// till whose person cannot (ADR-150).
	Openers []staffResponse `json:"openers,omitempty"`
}

// myShiftResponse is what the person asking did in the open shift: the
// orders they opened, what their payments settled, and the tips on them.
type myShiftResponse struct {
	ShiftID  int64        `json:"shift_id"`
	OpenedAt time.Time    `json:"opened_at"`
	Orders   int64        `json:"orders"`
	Tables   int64        `json:"tables"`
	Sales    money.Amount `json:"sales"`
	Tips     money.Amount `json:"tips"`
	// TipsByTable is largest first. Table 0 is the orders with no table.
	TipsByTable []tableTipsResponse `json:"tips_by_table"`
	// FirstAt is when they first touched an order in the shift, if they have.
	FirstAt *time.Time `json:"first_at,omitempty"`
}

type tableTipsResponse struct {
	TableID int64        `json:"table_id"`
	Tips    money.Amount `json:"tips"`
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
	// Opening is its own permission, so a cashier can start the day without
	// being able to close it (ADR-152).
	handle("POST /api/v1/shift/open", auth.PermShiftOpen, s.openShift)
	// Whoever takes payments handles the drawer; the movement itself is
	// authorised by a manager's override for cash.move.
	handle("POST /api/v1/shift/{id}/cash", auth.PermPaymentTake, s.moveCash)
	handle("POST /api/v1/shift/{id}/close", auth.PermShiftManage, s.closeShift)
	// Whoever runs the till counts it, blind, and explains the variance.
	// That is the cashier's permission (shift.open) and not payment.take,
	// which every waiter holds: the count is taken once and the reason is
	// written once, and a counted shift takes no more orders, so the pair
	// is a cash control and a way to stop service, not a sale.
	handle("POST /api/v1/shift/{id}/count", auth.PermShiftOpen, s.countDrawer)
	handle("POST /api/v1/shift/{id}/reason", auth.PermShiftOpen, s.explainVariance)
	mux.HandleFunc("/api/v1/shift", s.methodNotAllowed)
	if s.myShifts != nil {
		// Anyone who sells may read their own shift, and only their own.
		handle("GET /api/v1/shift/mine", auth.PermOrderOpen, s.myShift)
		mux.HandleFunc("/api/v1/shift/mine", s.methodNotAllowed)
	}
}

func (s *Server) myShift(w http.ResponseWriter, r *http.Request) {
	m := Meta(r)
	mine, err := s.myShifts.Mine(r.Context(), m.VenueID, m.ActorID)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	out := myShiftResponse{
		ShiftID: mine.ShiftID, OpenedAt: mine.OpenedAt.UTC(), Orders: mine.Orders, Tables: mine.Tables,
		Sales: mine.Sales, Tips: mine.Tips, TipsByTable: make([]tableTipsResponse, len(mine.ByTable)),
	}
	for i, t := range mine.ByTable {
		out.TipsByTable[i] = tableTipsResponse{TableID: t.TableID, Tips: t.Tips}
	}
	if mine.First != nil {
		at := mine.First.UTC()
		out.FirstAt = &at
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) currentShift(w http.ResponseWriter, r *http.Request) {
	sh, err := s.shifts.Current(r.Context(), Meta(r).VenueID)
	if errors.Is(err, svcshift.ErrNoOpenShift) {
		openers, err := s.staff.WithPermission(r.Context(), Meta(r).VenueID, auth.PermShiftOpen)
		if err != nil {
			// The till is closed whether or not we can say who to ask: say
			// the first, log the second.
			s.log.Error("listing who can open the shift failed", "request_id", RequestID(r.Context()), "error", err)
		}
		out := make([]staffResponse, len(openers))
		for i, member := range openers {
			out[i] = toStaffResponse(member)
		}
		writeJSON(w, s.log, http.StatusOK, currentShiftResponse{Openers: out})
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
	float, err := req.count(shift.CountOpening)
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

// count reads a count of kind: the total, checked against its notes and
// coins when they are given.
func (req countRequest) count(kind shift.CountKind) (shift.Count, error) {
	total := toMoney(req.Total)
	if len(req.Denominations) == 0 {
		return shift.LumpSum(kind, total)
	}
	c := shift.Count{Kind: kind, Denominations: req.Denominations, Total: total}
	if err := c.Validate(total.Currency); err != nil {
		return shift.Count{}, err
	}
	return shift.NewCount(kind, total.Currency, req.Denominations)
}

func (s *Server) countDrawer(w http.ResponseWriter, r *http.Request) {
	var req countRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	counted, err := req.count(shift.CountClosing)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	sh, err := s.shifts.Count(r.Context(), shiftMeta(r), id, counted)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toShiftResponse(sh))
}

func (s *Server) explainVariance(w http.ResponseWriter, r *http.Request) {
	var req reasonRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	sh, err := s.shifts.ExplainVariance(r.Context(), shiftMeta(r), id, req.Reason)
	if err != nil {
		s.writeShiftError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toShiftResponse(sh))
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
		OpenedBy: sh.OpenedBy, OpenedAt: sh.OpenedAt.UTC(), ClosedBy: sh.ClosedBy, ZReport: sh.ZReport,
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
	// The reveal: only from the closing count, never before it.
	if c := sh.Closing; c != nil {
		rec := c.Reconciliation
		out.Counted = true
		out.Reconciliation = &reconciliationResponse{
			Expected: rec.Expected, Counted: rec.Counted, Variance: rec.Variance, Threshold: rec.Threshold,
			Over: rec.Over(), Short: rec.Short(), ReasonRequired: rec.NeedsReason(),
			CountedBy: c.CountedBy, CountedAt: c.CountedAt.UTC(),
		}
	}
	if rs := sh.Reason; rs != nil {
		out.Reason = &varianceReasonResponse{Reason: rs.Reason, ExplainedBy: rs.ExplainedBy, ExplainedAt: rs.ExplainedAt.UTC()}
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
	{svcshift.ErrAlreadyCounted, http.StatusConflict, CodeAlreadyCounted},
	{svcshift.ErrNotCounted, http.StatusConflict, CodeNotCounted},
	{svcshift.ErrDrawerCounted, http.StatusConflict, CodeDrawerCounted},
	{svcshift.ErrVarianceUnexplained, http.StatusConflict, CodeVarianceUnexplained},
	{svcshift.ErrAlreadyExplained, http.StatusConflict, CodeAlreadyExplained},
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
	s.internalError(w, r, "a shift request failed", err)
}
