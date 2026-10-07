package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/service/stock"
)

// Stock is what the API needs from the venue's stock (phase 10, Task 3a).
// *stock.Service is the implementation.
type Stock interface {
	Items(ctx context.Context, venueID int64) ([]stock.Item, error)
	CreateItem(ctx context.Context, venueID int64, in stock.ItemInput) (int64, error)
	UpdateItem(ctx context.Context, venueID, id int64, in stock.ItemInput) error
	SetItemActive(ctx context.Context, venueID, id int64, active bool) error
	Currency(ctx context.Context, venueID int64) (string, error)
	Record(ctx context.Context, venueID, staffID, itemID int64, in stock.MovementInput) (stock.Recorded, error)
	Movements(ctx context.Context, venueID, itemID, before int64, limit int) ([]stock.Movement, int64, error)
	Recipes(ctx context.Context, venueID int64) (stock.RecipeBook, error)
	SetItemRecipe(ctx context.Context, venueID, itemID int64, parts []stock.RecipePart) error
	SetModifierRecipe(ctx context.Context, venueID, modifierID int64, parts []stock.RecipePart) error
	ClosingCount(ctx context.Context, venueID, shiftID int64) (stock.ClosingSheet, error)
	RecordClosingCount(ctx context.Context, venueID, staffID, shiftID int64, counts []stock.ClosingCountInput) (stock.ClosingSheet, error)
	ClosingReport(ctx context.Context, venueID int64, from, to string) (stock.ClosingReport, error)
	ClosingTable(ctx context.Context, venueID int64, from, to string) (export.Table, error)
	WasteReport(ctx context.Context, venueID int64, from, to string) (stock.WasteReport, error)
	PreviewItems(ctx context.Context, venueID int64, data []byte) (stock.ItemsImport, error)
	ImportItems(ctx context.Context, venueID int64, data []byte) (stock.ItemsImport, error)
	ItemsTemplate(ctx context.Context, venueID int64) ([][]string, error)
	PreviewRecipes(ctx context.Context, venueID int64, data []byte) (stock.RecipesImport, error)
	ImportRecipes(ctx context.Context, venueID int64, data []byte) (stock.RecipesImport, error)
	RecipesTemplate(ctx context.Context, venueID int64) ([][]string, error)
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
	CodeModifierNotFound  ErrorCode = "modifier_not_found"
	CodeStockCounted      ErrorCode = "stock_already_counted"
	CodeStockCountStale   ErrorCode = "stock_count_stale"
)

type (
	stockItemRequest = stock.ItemInput
	movementRequest  = stock.MovementInput
)

type stockItemsResponse struct {
	Items []stock.Item `json:"items"`
	// Currency is what the venue's stock is costed in.
	Currency string `json:"currency"`
}

// recipeRequest is a recipe, whole: what one sold takes from stock. An
// empty list clears it.
type recipeRequest struct {
	Parts []stock.RecipePart `json:"parts"`
}

// closingCountRequest is a shift's closing count: every item on the list.
type closingCountRequest struct {
	Counts []stock.ClosingCountInput `json:"counts"`
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
	handle("GET /api/v1/stock/recipes", s.listRecipes)
	handle("PUT /api/v1/stock/recipes/items/{id}", s.setRecipe(s.stock.SetItemRecipe))
	handle("PUT /api/v1/stock/recipes/modifiers/{id}", s.setRecipe(s.stock.SetModifierRecipe))
	s.mountStockImport(handle)
	s.mountRecipeImport(handle)

	// The closing count is part of closing the shift, so it needs what the
	// close needs (ADR-114).
	closing := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermShiftManage, h))))
	}
	closing("GET /api/v1/shift/{id}/stock-count", s.readClosingCount)
	closing("POST /api/v1/shift/{id}/stock-count", s.recordClosingCount)

	// The closing stock over a range is a report, read by whoever reads the
	// Z-report: it needs reports.view, not stock.manage.
	report := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, h))))
	}
	report("GET /api/v1/reports/closing-stock", s.readClosingReport)
	report("GET /api/v1/reports/closing-stock/csv", s.readClosingReportCSV)
	report("GET /api/v1/reports/wastage", s.readWasteReport)
}

// readClosingReport answers ?from=2026-10-01&to=2026-10-07, both trading
// days and required, with the closing stock counts of the shifts in them:
// expected, counted, and the variance in units and at cost (phase 10, Task
// 5f2, ADR-125). Bounded by the shifts and the items counted, so whole.
func (s *Server) readClosingReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rep, err := s.stock.ClosingReport(r.Context(), Meta(r).VenueID, q.Get("from"), q.Get("to"))
	if errors.Is(err, stock.ErrBadRange) {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
		return
	}
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, rep)
}

// readWasteReport answers ?from=2026-10-01&to=2026-10-07, both trading days
// and required, with what was written off in them, by item and by reason, at
// the cost each entry had (rebuild Task 16d1, ADR-186). Bounded by the stock
// items and the five reasons, so whole.
func (s *Server) readWasteReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rep, err := s.stock.WasteReport(r.Context(), Meta(r).VenueID, q.Get("from"), q.Get("to"))
	if errors.Is(err, stock.ErrBadRange) {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
		return
	}
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, rep)
}

// readClosingReportCSV answers the same range as a CSV file, a row per
// item per shift.
func (s *Server) readClosingReportCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	t, err := s.stock.ClosingTable(r.Context(), Meta(r).VenueID, q.Get("from"), q.Get("to"))
	if errors.Is(err, stock.ErrBadRange) {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
		return
	}
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	// The range was read, so from and to are days and safe in a name.
	s.writeCSV(w, r, "closing-stock-"+q.Get("from")+"-to-"+q.Get("to")+".csv", t)
}

func (s *Server) readClosingCount(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	sheet, err := s.stock.ClosingCount(r.Context(), Meta(r).VenueID, id)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, sheet)
}

func (s *Server) recordClosingCount(w http.ResponseWriter, r *http.Request) {
	var req closingCountRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	session, _ := SessionFrom(r.Context())
	sheet, err := s.stock.RecordClosingCount(r.Context(), Meta(r).VenueID, session.Staff.ID, id, req.Counts)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, sheet)
}

func (s *Server) listRecipes(w http.ResponseWriter, r *http.Request) {
	book, err := s.stock.Recipes(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, book)
}

// setRecipe replaces a menu item's or a modifier's recipe with set.
func (s *Server) setRecipe(set func(ctx context.Context, venueID, id int64, parts []stock.RecipePart) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req recipeRequest
		id, ok := s.pathID(w, r, "id")
		if !ok || !s.decode(w, r, &req) {
			return
		}
		if err := set(r.Context(), Meta(r).VenueID, id, req.Parts); err != nil {
			s.writeStockError(w, r, err)
			return
		}
		writeJSON(w, s.log, http.StatusOK, stockEditResponse{ID: id})
	}
}

func (s *Server) listStock(w http.ResponseWriter, r *http.Request) {
	items, err := s.stock.Items(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	currency, err := s.stock.Currency(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, stockItemsResponse{Items: items, Currency: currency})
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
	case errors.Is(err, stock.ErrMenuItemNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeItemNotFound, "No such item on the menu.")
	case errors.Is(err, stock.ErrModifierNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeModifierNotFound, "No such choice on the menu.")
	case errors.Is(err, stock.ErrShiftNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeShiftUnknown, "No such shift.")
	case errors.Is(err, stock.ErrShiftClosed):
		writeError(w, s.log, http.StatusConflict, CodeShiftClosed, "That shift is closed.")
	case errors.Is(err, stock.ErrAlreadyCounted):
		writeError(w, s.log, http.StatusConflict, CodeStockCounted, "The closing stock is already counted. It can't be counted again.")
	case errors.Is(err, stock.ErrStaleCount):
		writeError(w, s.log, http.StatusConflict, CodeStockCountStale, "The list of items to count changed. Reload and count again.")
	case errors.Is(err, stock.ErrRetired):
		writeError(w, s.log, http.StatusConflict, CodeStockItemRetired, "That stock item is retired. Bring it back first.")
	default:
		s.internalError(w, r, "a stock request failed", err)
	}
}
