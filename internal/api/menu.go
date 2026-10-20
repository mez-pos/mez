package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// Menus is what the API needs to read a venue's menu. *db.DB implements it.
type Menus interface {
	Menu(ctx context.Context, venueID int64) (menu.Menu, error)
	MenuVersion(ctx context.Context, venueID int64) (int64, error)
}

// WithMenus mounts the menu endpoint. It needs WithDevices: the menu is
// read by a paired device, and without one the route is not mounted.
func WithMenus(m Menus) Option {
	return func(s *Server) { s.menus = m }
}

// mountMenu routes /api/v1/menu.
//
// Any paired device of the venue may read it, with or without a staff
// member signed in: a kitchen display needs the stations before anyone
// has touched it, and nothing on a menu is private.
func (s *Server) mountMenu(mux *http.ServeMux) {
	if s.devices == nil || s.menus == nil {
		return
	}
	mux.Handle("GET /api/v1/menu", s.requireDevice(http.HandlerFunc(s.readMenu)))
	mux.HandleFunc("/api/v1/menu", s.methodNotAllowed)
}

// readMenu answers with the whole menu, or 304 when the caller already has
// this version. The ETag is the version, so asking "has it changed" costs
// one indexed read rather than the menu.
func (s *Server) readMenu(w http.ResponseWriter, r *http.Request) {
	venueID := Meta(r).VenueID
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		version, err := s.menus.MenuVersion(r.Context(), venueID)
		if err != nil {
			s.menuFailed(w, r, err)
			return
		}
		if inm == menuETag(version) {
			w.Header().Set("ETag", menuETag(version))
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	m, err := s.menus.Menu(r.Context(), venueID)
	if err != nil {
		s.menuFailed(w, r, err)
		return
	}
	w.Header().Set("ETag", menuETag(m.Version))
	// The browser may keep it, but must ask before using it: the realtime
	// stream is what says a menu changed, and a stale one sells at old prices.
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, s.log, http.StatusOK, toMenuResponse(m))
}

func (s *Server) menuFailed(w http.ResponseWriter, r *http.Request, err error) {
	s.internalError(w, r, "reading the menu failed", err, "venue_id", Meta(r).VenueID)
}

func menuETag(version int64) string {
	return `"menu-` + strconv.FormatInt(version, 10) + `"`
}

// menuResponse is the venue's menu, whole. A terminal caches it by
// version and rings items up from it.
type menuResponse struct {
	Version  int64  `json:"version"`
	Currency string `json:"currency"`

	Stations       []stationResponse       `json:"stations"`
	Courses        []courseResponse        `json:"courses"`
	Categories     []categoryResponse      `json:"categories"`
	Items          []menuItemResponse      `json:"items"`
	ModifierGroups []modifierGroupResponse `json:"modifier_groups"`
}

type stationResponse struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type courseResponse struct {
	ID     int64  `json:"id"`
	Number int64  `json:"number"`
	Name   string `json:"name"`
}

type categoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// menuItemResponse is one item, carrying what a line snapshots when it is
// rung up: station is a station's code, course a course's number.
type menuItemResponse struct {
	ID             int64        `json:"id"`
	CategoryID     int64        `json:"category_id"`
	Name           string       `json:"name"`
	Price          money.Amount `json:"price"`
	Station        string       `json:"station"`
	Course         int64        `json:"course"`
	TaxRate        tax.Rate     `json:"tax_rate"`
	HSN            string       `json:"hsn,omitempty"`
	TaxExempt      bool         `json:"tax_exempt,omitempty"`
	ModifierGroups []int64      `json:"modifier_groups"`
	// SoldOutUntil is set while the item is sold out: until then the
	// register shows it sold out and will not ring it up (ADR-109).
	SoldOutUntil *time.Time `json:"sold_out_until,omitempty"`
	// Photo is a stored photo's name, served at /api/v1/menu/photos/{name}.
	Photo string `json:"photo,omitempty"`
	// What the card shows (ADR-147). Left out of the JSON when there is
	// nothing to show, so an item that says none reads as before.
	Description string    `json:"description,omitempty"`
	Diet        menu.Diet `json:"diet,omitempty"`
	GlutenFree  bool      `json:"gluten_free,omitempty"`
	Popular     bool      `json:"popular,omitempty"`
}

// modifierGroupResponse is a group of choices. A group with min_select
// above zero is required.
type modifierGroupResponse struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	MinSelect int64              `json:"min_select"`
	MaxSelect int64              `json:"max_select"`
	Modifiers []modifierResponse `json:"modifiers"`
}

// modifierResponse is one choice. price is per unit of the line and may be
// negative.
type modifierResponse struct {
	ID        int64        `json:"id"`
	Name      string       `json:"name"`
	Price     money.Amount `json:"price"`
	IsDefault bool         `json:"is_default"`
}

func toMenuResponse(m menu.Menu) menuResponse {
	out := menuResponse{
		Version: m.Version, Currency: m.Currency,
		Stations:       make([]stationResponse, len(m.Stations)),
		Courses:        make([]courseResponse, len(m.Courses)),
		Categories:     make([]categoryResponse, len(m.Categories)),
		Items:          make([]menuItemResponse, len(m.Items)),
		ModifierGroups: make([]modifierGroupResponse, len(m.Groups)),
	}
	for i, st := range m.Stations {
		out.Stations[i] = stationResponse{ID: st.ID, Code: st.Code, Name: st.Name}
	}
	for i, c := range m.Courses {
		out.Courses[i] = courseResponse{ID: c.ID, Number: c.Number, Name: c.Name}
	}
	for i, c := range m.Categories {
		out.Categories[i] = categoryResponse{ID: c.ID, Name: c.Name}
	}
	for i, it := range m.Items {
		groups := it.Groups
		if groups == nil {
			groups = []int64{}
		}
		out.Items[i] = menuItemResponse{
			ID: it.ID, CategoryID: it.CategoryID, Name: it.Name, Price: it.Price,
			Station: it.Station, Course: it.Course, TaxRate: it.TaxRate, HSN: it.HSN,
			TaxExempt: it.TaxExempt, ModifierGroups: groups, SoldOutUntil: it.SoldOutUntil, Photo: it.Photo,
			Description: it.Description, Diet: it.Diet, GlutenFree: it.GlutenFree, Popular: it.Popular,
		}
	}
	for i, g := range m.Groups {
		mods := make([]modifierResponse, len(g.Modifiers))
		for j, md := range g.Modifiers {
			mods[j] = modifierResponse{ID: md.ID, Name: md.Name, Price: md.Price, IsDefault: md.IsDefault}
		}
		out.ModifierGroups[i] = modifierGroupResponse{
			ID: g.ID, Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Modifiers: mods,
		}
	}
	return out
}
