package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/service/menus"
)

// MenuEditor is what the API needs from the back office's menu editor.
// *menus.Service is the implementation.
type MenuEditor interface {
	Editor(ctx context.Context, venueID int64) (menus.Editor, error)
	CreateCategory(ctx context.Context, venueID int64, in menus.CategoryInput) (int64, error)
	RenameCategory(ctx context.Context, venueID, id int64, in menus.CategoryInput) error
	SetCategoryActive(ctx context.Context, venueID, id int64, active bool) error
	OrderCategories(ctx context.Context, venueID int64, ids []int64) error
	CreateItem(ctx context.Context, venueID int64, in menus.ItemInput) (int64, error)
	UpdateItem(ctx context.Context, venueID, id int64, in menus.ItemInput) error
	SetItemActive(ctx context.Context, venueID, id int64, active bool) error
	OrderItems(ctx context.Context, venueID, categoryID int64, ids []int64) error
	CreateGroup(ctx context.Context, venueID int64, in menus.GroupInput) (int64, error)
	UpdateGroup(ctx context.Context, venueID, id int64, in menus.GroupInput) error
	SetGroupActive(ctx context.Context, venueID, id int64, active bool) error
	SetItemGroups(ctx context.Context, venueID, itemID int64, ids []int64) error
	SetItemSoldOut(ctx context.Context, venueID, id int64, soldOut bool) (*time.Time, error)
}

// WithMenuEditor mounts the menu editor (phase 10, Task 2a). It needs
// WithDevices and WithStaff, and menu.edit.
func WithMenuEditor(m MenuEditor) Option {
	return func(s *Server) { s.menuEditor = m }
}

// The menu editor's error codes.
const (
	CodeCategoryNotFound ErrorCode = "category_not_found"
	CodeItemNotFound     ErrorCode = "item_not_found"
	CodeInvalidMenu      ErrorCode = "invalid_menu"
	CodeMenuOrderStale   ErrorCode = "menu_order_stale"
	CodeGroupNotFound    ErrorCode = "modifier_group_not_found"
)

type (
	categoryRequest = menus.CategoryInput
	itemRequest     = menus.ItemInput
	groupRequest    = menus.GroupInput
)

// soldOutResponse answers a sold-out change: when the item is back on
// sale, or nothing when it already is.
type soldOutResponse struct {
	ID           int64      `json:"id"`
	SoldOutUntil *time.Time `json:"sold_out_until,omitempty"`
}

// menuOrderRequest is a list of ids in the order they should show.
type menuOrderRequest struct {
	IDs []int64 `json:"ids"`
}

// menuEditResponse answers an edit with the row it made or changed; none
// for an ordering of categories. The editor reads the menu again after it.
type menuEditResponse struct {
	ID int64 `json:"id,omitempty"`
}

// mountMenuEditor routes the editor under /api/v1/menu/, beside the
// terminal's GET /api/v1/menu. Every route needs menu.edit.
func (s *Server) mountMenuEditor(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.menuEditor == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermMenuEdit, h))))
	}
	handle("GET /api/v1/menu/editor", s.readMenuEditor)

	handle("POST /api/v1/menu/categories", s.createCategory)
	// The literal path wins over {id}: "order" is never a category's id.
	handle("PUT /api/v1/menu/categories/order", s.orderCategories)
	handle("PUT /api/v1/menu/categories/{id}", s.renameCategory)
	handle("POST /api/v1/menu/categories/{id}/retire", s.activateCategory(false))
	handle("POST /api/v1/menu/categories/{id}/restore", s.activateCategory(true))
	handle("PUT /api/v1/menu/categories/{id}/items/order", s.orderItems)

	handle("POST /api/v1/menu/items", s.createItem)
	handle("PUT /api/v1/menu/items/{id}", s.updateItem)
	handle("POST /api/v1/menu/items/{id}/retire", s.activateItem(false))
	handle("POST /api/v1/menu/items/{id}/restore", s.activateItem(true))
	handle("PUT /api/v1/menu/items/{id}/groups", s.setItemGroups)
	handle("POST /api/v1/menu/items/{id}/sold-out", s.soldOut(true))
	handle("POST /api/v1/menu/items/{id}/on-sale", s.soldOut(false))

	handle("POST /api/v1/menu/groups", s.createGroup)
	handle("PUT /api/v1/menu/groups/{id}", s.updateGroup)
	handle("POST /api/v1/menu/groups/{id}/retire", s.activateGroup(false))
	handle("POST /api/v1/menu/groups/{id}/restore", s.activateGroup(true))
}

func (s *Server) soldOut(soldOut bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		until, err := s.menuEditor.SetItemSoldOut(r.Context(), Meta(r).VenueID, id, soldOut)
		if err != nil {
			s.writeMenuEditError(w, r, err)
			return
		}
		writeJSON(w, s.log, http.StatusOK, soldOutResponse{ID: id, SoldOutUntil: until})
	}
}

func (s *Server) setItemGroups(w http.ResponseWriter, r *http.Request) {
	var req menuOrderRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.SetItemGroups(r.Context(), Meta(r).VenueID, id, req.IDs))
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var req groupRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.menuEditor.CreateGroup(r.Context(), Meta(r).VenueID, req)
	s.menuEdited(w, r, http.StatusCreated, id, err)
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	var req groupRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.UpdateGroup(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) activateGroup(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.SetGroupActive(r.Context(), Meta(r).VenueID, id, active))
	}
}

func (s *Server) readMenuEditor(w http.ResponseWriter, r *http.Request) {
	e, err := s.menuEditor.Editor(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, e)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.menuEditor.CreateCategory(r.Context(), Meta(r).VenueID, req)
	s.menuEdited(w, r, http.StatusCreated, id, err)
}

func (s *Server) renameCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.RenameCategory(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) activateCategory(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.SetCategoryActive(r.Context(), Meta(r).VenueID, id, active))
	}
}

func (s *Server) orderCategories(w http.ResponseWriter, r *http.Request) {
	var req menuOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, 0, s.menuEditor.OrderCategories(r.Context(), Meta(r).VenueID, req.IDs))
}

func (s *Server) orderItems(w http.ResponseWriter, r *http.Request) {
	var req menuOrderRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.OrderItems(r.Context(), Meta(r).VenueID, id, req.IDs))
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var req itemRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.menuEditor.CreateItem(r.Context(), Meta(r).VenueID, req)
	s.menuEdited(w, r, http.StatusCreated, id, err)
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	var req itemRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.UpdateItem(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) activateItem(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.SetItemActive(r.Context(), Meta(r).VenueID, id, active))
	}
}

// menuEdited answers an edit: its error, or the id it was about.
func (s *Server) menuEdited(w http.ResponseWriter, r *http.Request, status int, id int64, err error) {
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	writeJSON(w, s.log, status, menuEditResponse{ID: id})
}

func (s *Server) writeMenuEditError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *menus.Invalid
	switch {
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidMenu, invalid.Message, invalid.Field)
	case errors.Is(err, menus.ErrCategoryNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeCategoryNotFound, "No such category.")
	case errors.Is(err, menus.ErrItemNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeItemNotFound, "No such item.")
	case errors.Is(err, menus.ErrGroupNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeGroupNotFound, "No such group of choices.")
	case errors.Is(err, menus.ErrStaleOrder):
		writeError(w, s.log, http.StatusConflict, CodeMenuOrderStale,
			"The menu changed since you opened it. Reload and put it in order again.")
	default:
		s.log.Error("a menu editor request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
