package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/menus"
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
	CreateStation(ctx context.Context, venueID int64, in menus.StationInput) (int64, error)
	RenameStation(ctx context.Context, venueID, id int64, in menus.StationInput) error
	OrderStations(ctx context.Context, venueID int64, ids []int64) error
	RetireStation(ctx context.Context, venueID, id int64, in menus.RetireInput) (menus.StationRetired, error)
	RestoreStation(ctx context.Context, venueID, id int64) error
	CreateCourse(ctx context.Context, venueID int64, in menus.CourseInput) (int64, error)
	RenameCourse(ctx context.Context, venueID, id int64, in menus.CourseInput) error
	OrderCourses(ctx context.Context, venueID int64, ids []int64) error
	RetireCourse(ctx context.Context, venueID, id int64, in menus.RetireInput) (menus.CourseRetired, error)
	RestoreCourse(ctx context.Context, venueID, id int64) error
	SetItemPhoto(ctx context.Context, venueID, id int64, data []byte) (string, error)
	ClearItemPhoto(ctx context.Context, venueID, id int64) error
	OpenPhoto(name string) (*os.File, error)
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
	CodePhotoNotFound    ErrorCode = "photo_not_found"
	CodeCourseNotFound   ErrorCode = "course_not_found"
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

// photoResponse answers a photo upload with the stored photo's name.
type photoResponse struct {
	ID    int64  `json:"id"`
	Photo string `json:"photo"`
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
	handle("PUT /api/v1/menu/items/{id}/photo", s.putItemPhoto)
	handle("DELETE /api/v1/menu/items/{id}/photo", s.deleteItemPhoto)
	// Public: an <img> cannot send the device and staff tokens, and nothing
	// on a menu is private. The name is a hash, checked before any path is
	// made from it (ADR-110).
	mux.HandleFunc("GET /api/v1/menu/photos/{name}", s.servePhoto)

	s.mountStationsAndCourses(handle)

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

func (s *Server) putItemPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "image/jpeg" {
		writeFieldError(w, s.log, http.StatusUnsupportedMediaType, CodeInvalidMenu, "Send the photo as image/jpeg.", "photo")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, menus.MaxPhotoBytes+1))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		writeFieldError(w, s.log, http.StatusRequestEntityTooLarge, CodeInvalidMenu,
			fmt.Sprintf("That photo is over %d KB.", menus.MaxPhotoBytes>>10), "photo")
		return
	case err != nil:
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "the photo could not be read")
		return
	}
	name, err := s.menuEditor.SetItemPhoto(r.Context(), Meta(r).VenueID, id, data)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, photoResponse{ID: id, Photo: name})
}

func (s *Server) deleteItemPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.ClearItemPhoto(r.Context(), Meta(r).VenueID, id))
}

// servePhoto answers a stored photo. Its name is the hash of its bytes, so
// it never changes and may be kept for good; a new photo is a new name.
func (s *Server) servePhoto(w http.ResponseWriter, r *http.Request) {
	f, err := s.menuEditor.OpenPhoto(r.PathValue("name"))
	if errors.Is(err, menus.ErrPhotoNotFound) {
		writeError(w, s.log, http.StatusNotFound, CodePhotoNotFound, "No such photo.")
		return
	}
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), f)
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
	case errors.Is(err, menus.ErrStationNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeStationNotFound, "No such station.")
	case errors.Is(err, menus.ErrCourseNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeCourseNotFound, "No such course.")
	case errors.Is(err, menus.ErrTicketsOpen):
		writeError(w, s.log, http.StatusConflict, CodeTicketsOpen,
			"Orders are still open. Settle or void them before changing the order of the courses.")
	case errors.Is(err, menus.ErrStaleOrder):
		writeError(w, s.log, http.StatusConflict, CodeMenuOrderStale,
			"The menu changed since you opened it. Reload and put it in order again.")
	default:
		s.internalError(w, r, "a menu editor request failed", err)
	}
}
