package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/table"
	"github.com/byjoellobo/nofiltr-pos/internal/service/floor"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// Floor is what the API needs from the floor plan. *floor.Service is the
// implementation.
type Floor interface {
	Plan(ctx context.Context, venueID int64) (floor.Plan, error)
	CreateArea(ctx context.Context, venueID int64, in floor.AreaSettings) (int64, error)
	RenameArea(ctx context.Context, venueID, id int64, in floor.AreaSettings) error
	SetAreaActive(ctx context.Context, venueID, id int64, active bool) error
	CreateTable(ctx context.Context, venueID int64, in floor.TableSettings) (int64, error)
	UpdateTable(ctx context.Context, venueID, id int64, in floor.TableSettings) error
	SetTableActive(ctx context.Context, venueID, id int64, active bool) error
	Live(ctx context.Context, venueID int64) (floor.Live, error)
	Clear(ctx context.Context, venueID, id int64) error
	Release(ctx context.Context, meta svcorder.Meta, id int64) error
}

// WithFloor mounts the floor plan editor (phase 08, Task 1), which needs
// settings.edit, and the live floor (Task 2a), which needs order.open. Both
// need WithDevices and WithStaff.
func WithFloor(f Floor) Option {
	return func(s *Server) { s.floor = f }
}

// The floor plan error codes.
const (
	CodeAreaNotFound   ErrorCode = "area_not_found"
	CodeAreaRetired    ErrorCode = "area_retired"
	CodeAreaInUse      ErrorCode = "area_in_use"
	CodeTableNotFound  ErrorCode = "table_not_found"
	CodeCellTaken      ErrorCode = "cell_taken"
	CodeFloorNameTaken ErrorCode = "floor_name_taken"
	CodeInvalidFloor   ErrorCode = "invalid_floor"
	CodeTableOccupied  ErrorCode = "table_occupied"
	CodeTableNotEmpty  ErrorCode = "table_not_empty"
)

type areaRequest = floor.AreaSettings

type tableRequest = floor.TableSettings

type gridResponse struct {
	Columns int64 `json:"columns"`
	Rows    int64 `json:"rows"`
}

type areaResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type tableResponse struct {
	ID     int64       `json:"id"`
	AreaID int64       `json:"area_id"`
	Name   string      `json:"name"`
	Seats  int64       `json:"seats"`
	Shape  floor.Shape `json:"shape"`
	X      int64       `json:"x"` // the column, from 0
	Y      int64       `json:"y"` // the row, from 0
	Active bool        `json:"active"`
}

type floorResponse struct {
	Grid   gridResponse    `json:"grid"`
	Areas  []areaResponse  `json:"areas"`
	Tables []tableResponse `json:"tables"`
}

// liveTableResponse is a table in service and where it is in its turn.
type liveTableResponse struct {
	ID     int64        `json:"id"`
	AreaID int64        `json:"area_id"`
	Name   string       `json:"name"`
	Seats  int64        `json:"seats"`
	Shape  floor.Shape  `json:"shape"`
	X      int64        `json:"x"`
	Y      int64        `json:"y"`
	Status table.Status `json:"status"`
	Covers int64        `json:"covers"`
	// Since is when the table was seated, or when it went dirty; absent
	// while it is free.
	Since *time.Time `json:"since,omitempty"`
	// Balance is still owed over its open orders; absent with none open.
	Balance *money.Amount `json:"balance,omitempty"`
	// Orders are the public ids of the orders open at it, oldest first.
	Orders []string `json:"orders"`
	// JoinedTo is the table whose party this one is joined to (ADR-088).
	JoinedTo int64 `json:"joined_to,omitempty"`
}

type liveFloorResponse struct {
	Grid   gridResponse        `json:"grid"`
	Areas  []areaResponse      `json:"areas"`
	Tables []liveTableResponse `json:"tables"`
}

func (s *Server) mountFloor(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.floor == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/floor", s.writeFloorOK)
	mux.HandleFunc("/api/v1/floor", s.methodNotAllowed)
	handle("POST /api/v1/floor/areas", s.createArea)
	handle("PUT /api/v1/floor/areas/{id}", s.renameArea)
	handle("POST /api/v1/floor/areas/{id}/retire", s.activateArea(false))
	handle("POST /api/v1/floor/areas/{id}/restore", s.activateArea(true))
	handle("POST /api/v1/floor/tables", s.createTable)
	handle("PUT /api/v1/floor/tables/{id}", s.updateTable)
	handle("POST /api/v1/floor/tables/{id}/retire", s.activateTable(false))
	handle("POST /api/v1/floor/tables/{id}/restore", s.activateTable(true))

	// The floor in service: any staff member who can open an order.
	service := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermOrderOpen, h))))
	}
	service("GET /api/v1/floor/live", s.writeLiveFloor)
	service("POST /api/v1/floor/tables/{id}/clear", s.clearTable)
	service("POST /api/v1/floor/tables/{id}/release", s.releaseTable)
}

// releaseTable lets guests go who sat and ordered nothing: their empty
// tickets close and the table is free (ADR-087).
func (s *Server) releaseTable(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.floor.Release(r.Context(), Meta(r), id); err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	s.writeLiveFloor(w, r)
}

func (s *Server) clearTable(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.floor.Clear(r.Context(), Meta(r).VenueID, id); err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	s.writeLiveFloor(w, r)
}

// writeLiveFloor answers with the floor in service as it now stands.
func (s *Server) writeLiveFloor(w http.ResponseWriter, r *http.Request) {
	live, err := s.floor.Live(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	out := liveFloorResponse{
		Grid:   gridResponse{Columns: floor.Columns, Rows: floor.Rows},
		Areas:  make([]areaResponse, len(live.Areas)),
		Tables: make([]liveTableResponse, len(live.Tables)),
	}
	for i, a := range live.Areas {
		out.Areas[i] = areaResponse{ID: a.ID, Name: a.Name, Active: a.Active}
	}
	for i, lt := range live.Tables {
		t, v := lt.Table, lt.View
		out.Tables[i] = liveTableResponse{
			ID: t.ID, AreaID: t.AreaID, Name: t.Name, Seats: t.Seats, Shape: floor.Shape(t.Shape), X: t.X, Y: t.Y,
			Status: v.Status, Covers: v.Covers, Orders: lt.Orders, JoinedTo: lt.JoinedTo,
		}
		if !v.Since.IsZero() {
			since := v.Since.UTC()
			out.Tables[i].Since = &since
		}
		if v.BalanceValid {
			balance := v.Balance
			out.Tables[i].Balance = &balance
		}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) writeFloorOK(w http.ResponseWriter, r *http.Request) {
	s.writeFloor(w, r, http.StatusOK)
}

func (s *Server) createArea(w http.ResponseWriter, r *http.Request) {
	var req areaRequest
	if !s.decode(w, r, &req) {
		return
	}
	if _, err := s.floor.CreateArea(r.Context(), Meta(r).VenueID, req); err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	s.writeFloor(w, r, http.StatusCreated)
}

func (s *Server) renameArea(w http.ResponseWriter, r *http.Request) {
	var req areaRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.floorChange(w, r, s.floor.RenameArea(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) activateArea(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if id, ok := s.pathID(w, r, "id"); ok {
			s.floorChange(w, r, s.floor.SetAreaActive(r.Context(), Meta(r).VenueID, id, active))
		}
	}
}

func (s *Server) createTable(w http.ResponseWriter, r *http.Request) {
	var req tableRequest
	if !s.decode(w, r, &req) {
		return
	}
	if _, err := s.floor.CreateTable(r.Context(), Meta(r).VenueID, req); err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	s.writeFloor(w, r, http.StatusCreated)
}

func (s *Server) updateTable(w http.ResponseWriter, r *http.Request) {
	var req tableRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.floorChange(w, r, s.floor.UpdateTable(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) activateTable(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if id, ok := s.pathID(w, r, "id"); ok {
			s.floorChange(w, r, s.floor.SetTableActive(r.Context(), Meta(r).VenueID, id, active))
		}
	}
}

// floorChange answers a change: its refusal, or the plan as it now stands.
func (s *Server) floorChange(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	s.writeFloor(w, r, http.StatusOK)
}

// writeFloor answers with the whole plan, so the editor redraws from the
// server after every change.
func (s *Server) writeFloor(w http.ResponseWriter, r *http.Request, status int) {
	p, err := s.floor.Plan(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeFloorError(w, r, err)
		return
	}
	out := floorResponse{
		Grid:   gridResponse{Columns: floor.Columns, Rows: floor.Rows},
		Areas:  make([]areaResponse, len(p.Areas)),
		Tables: make([]tableResponse, len(p.Tables)),
	}
	for i, a := range p.Areas {
		out.Areas[i] = areaResponse{ID: a.ID, Name: a.Name, Active: a.Active}
	}
	for i, t := range p.Tables {
		out.Tables[i] = tableResponse{
			ID: t.ID, AreaID: t.AreaID, Name: t.Name, Seats: t.Seats, Shape: floor.Shape(t.Shape), X: t.X, Y: t.Y, Active: t.Active,
		}
	}
	writeJSON(w, s.log, status, out)
}

func (s *Server) writeFloorError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *floor.Invalid
	hasField := errors.As(err, &invalid)
	switch {
	case errors.Is(err, floor.ErrDuplicate) && hasField:
		writeFieldError(w, s.log, http.StatusConflict, CodeFloorNameTaken, invalid.Message, invalid.Field)
	case errors.Is(err, floor.ErrCellTaken) && hasField:
		writeFieldError(w, s.log, http.StatusConflict, CodeCellTaken, invalid.Message, invalid.Field)
	case errors.Is(err, floor.ErrAreaRetired) && hasField:
		writeFieldError(w, s.log, http.StatusConflict, CodeAreaRetired, invalid.Message, invalid.Field)
	case errors.Is(err, floor.ErrAreaNotFound) && hasField:
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeAreaNotFound, invalid.Message, invalid.Field)
	case hasField:
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidFloor, invalid.Message, invalid.Field)
	case errors.Is(err, floor.ErrAreaNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeAreaNotFound, "No such area.")
	case errors.Is(err, floor.ErrTableNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeTableNotFound, "No such table.")
	case errors.Is(err, table.ErrOccupied):
		writeError(w, s.log, http.StatusConflict, CodeTableOccupied, "Guests are still at this table: settle or move their ticket first.")
	case errors.Is(err, table.ErrNotEmpty):
		writeError(w, s.log, http.StatusConflict, CodeTableNotEmpty, "Something is on this table's ticket: void it or take the payment first.")
	case errors.Is(err, floor.ErrAreaInUse):
		writeError(w, s.log, http.StatusConflict, CodeAreaInUse, "Retire or move the tables in this area first.")
	default:
		s.log.Error("a floor plan request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
