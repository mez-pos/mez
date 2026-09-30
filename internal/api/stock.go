package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/service/stock"
)

// Stock is what the API needs from the venue's stock (phase 10, Task 3a).
// *stock.Service is the implementation.
type Stock interface {
	Items(ctx context.Context, venueID int64) ([]stock.Item, error)
	CreateItem(ctx context.Context, venueID int64, in stock.ItemInput) (int64, error)
	UpdateItem(ctx context.Context, venueID, id int64, in stock.ItemInput) error
	SetItemActive(ctx context.Context, venueID, id int64, active bool) error
	Record(ctx context.Context, venueID, staffID, itemID int64, in stock.MovementInput) (stock.Recorded, error)
	Movements(ctx context.Context, venueID, itemID, before int64, limit int) ([]stock.Movement, int64, error)
}

// WithStock mounts the stock routes. They need WithDevices and WithStaff,
// and stock.manage.
func WithStock(s Stock) Option {
	return func(srv *Server) { srv.stock = s }
}

// The stock error codes.
const (
	CodeStockItemNotFound ErrorCode = "stock_item_not_found"
	CodeStockItemRetired  ErrorCode = "stock_item_retired"
	CodeStockNameTaken    ErrorCode = "stock_name_taken"
	CodeInvalidStock      ErrorCode = "invalid_stock"
)

type (
	stockItemRequest = stock.ItemInput
	movementRequest  = stock.MovementInput
)

type stockItemsResponse struct {
	Items []stock.Item `json:"items"`
}

type stockEditResponse struct {
	ID int64 `json:"id"`
}

// movementsResponse is a page of an item's history, newest first. Pass
// next_before as ?before= for the page after; it is absent on the last.
type movementsResponse struct {
	Movements  []stock.Movement `json:"movements"`
	NextBefore int64            `json:"next_before,omitempty"`
}

func (s *Server) mountStock(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.stock == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermStockManage, h))))
	}
	handle("GET /api/v1/stock", s.listStock)
	handle("POST /api/v1/stock/items", s.createStockItem)
	handle("PUT /api/v1/stock/items/{id}", s.updateStockItem)
	handle("POST /api/v1/stock/items/{id}/retire", s.activateStockItem(false))
	handle("POST /api/v1/stock/items/{id}/restore", s.activateStockItem(true))
	handle("POST /api/v1/stock/items/{id}/movements", s.recordMovement)
	handle("GET /api/v1/stock/items/{id}/movements", s.listMovements)
}

func (s *Server) listStock(w http.ResponseWriter, r *http.Request) {
	items, err := s.stock.Items(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, stockItemsResponse{Items: items})
}

func (s *Server) createStockItem(w http.ResponseWriter, r *http.Request) {
	var req stockItemRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.stock.CreateItem(r.Context(), Meta(r).VenueID, req)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, stockEditResponse{ID: id})
}

func (s *Server) updateStockItem(w http.ResponseWriter, r *http.Request) {
	var req stockItemRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	if err := s.stock.UpdateItem(r.Context(), Meta(r).VenueID, id, req); err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, stockEditResponse{ID: id})
}

func (s *Server) activateStockItem(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		if err := s.stock.SetItemActive(r.Context(), Meta(r).VenueID, id, active); err != nil {
			s.writeStockError(w, r, err)
			return
		}
		writeJSON(w, s.log, http.StatusOK, stockEditResponse{ID: id})
	}
}

// recordMovement records a movement by the staff member signed in: who
// received, wasted or counted it is part of the record.
func (s *Server) recordMovement(w http.ResponseWriter, r *http.Request) {
	var req movementRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	session, _ := SessionFrom(r.Context())
	rec, err := s.stock.Record(r.Context(), Meta(r).VenueID, session.Staff.ID, id, req)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, rec)
}

func (s *Server) listMovements(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	before, limit, ok := s.pageParams(w, r)
	if !ok {
		return
	}
	page, next, err := s.stock.Movements(r.Context(), Meta(r).VenueID, id, before, limit)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, movementsResponse{Movements: page, NextBefore: next})
}

// pageParams reads ?before= and ?limit=, both optional whole numbers.
func (s *Server) pageParams(w http.ResponseWriter, r *http.Request) (int64, int, bool) {
	read := func(name string) (int64, bool) {
		v := r.URL.Query().Get(name)
		if v == "" {
			return 0, true
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, name+" must be a whole number, 0 or more", name)
			return 0, false
		}
		return n, true
	}
	before, ok := read("before")
	if !ok {
		return 0, 0, false
	}
	limit, ok := read("limit")
	if !ok {
		return 0, 0, false
	}
	return before, int(min(limit, stock.MaxPage)), true
}

func (s *Server) writeStockError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *stock.Invalid
	switch {
	case errors.Is(err, stock.ErrNameTaken) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodeStockNameTaken, invalid.Message, invalid.Field)
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidStock, invalid.Message, invalid.Field)
	case errors.Is(err, stock.ErrNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeStockItemNotFound, "No such stock item.")
	case errors.Is(err, stock.ErrRetired):
		writeError(w, s.log, http.StatusConflict, CodeStockItemRetired, "That stock item is retired. Bring it back first.")
	default:
		s.log.Error("a stock request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
