package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/service/tablecodes"
)

// The guest API (phase 11, Task 3a, ADR-206): what a guest's phone asks the
// venue. It lives under the guest app's mount, /order/api/, and not under
// /api/v1: the tunnel puts every guest request under that mount, so this is
// the one API a guest can reach through the relay, and the staff API is the
// one they cannot. On the venue's own wifi it is the same paths.
//
// A guest is nobody: no device, no session. What they hold is the code on
// their table, and every route here is under it.

// CodeTableCodeUnknown is a table code that leads nowhere.
const CodeTableCodeUnknown ErrorCode = "table_code_unknown"

// guestLimit is what one address may ask of the guest API. A phone opening
// the menu asks a handful of times; a table of six behind one address asks
// a few dozen. It is far under what guessing a code would need.
var guestLimit = limit{requests: 300, per: time.Minute}

// Guests is what the API needs to read a table's code.
type Guests interface {
	Resolve(ctx context.Context, code string) (tablecodes.Table, error)
}

// WithGuests mounts the guest API under the guest app's mount, which is
// web.MountGuest. It needs WithMenus for the menu.
func WithGuests(g Guests, mount string) Option {
	return func(s *Server) {
		s.guests = g
		s.guestMount = mount
	}
}

// mountGuests routes /order/api/.
func (s *Server) mountGuests(mux *http.ServeMux) {
	if s.guests == nil {
		return
	}
	base := s.guestMount + "api/"
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.rateLimit(guestLimit, h))
	}
	handle("GET "+base+"t/{code}", s.guestTable)
	if s.menus != nil {
		handle("GET "+base+"t/{code}/menu", s.guestMenu)
	}
	if s.guestOrders != nil {
		handle("POST "+base+"t/{code}/orders", s.guestPlaceOrder)
	}
	if s.menuEditor != nil {
		// A photo is on the menu the guest was shown, and named by its
		// content: there is nothing to guard, and a cache may keep it.
		handle("GET "+base+"photos/{name}", s.servePhoto)
	}
	// Anything else under the API is not a page of the app: without this
	// the app's own fallback would answer a mistyped call with index.html
	// and a 200.
	mux.Handle(base, s.rateLimit(guestLimit, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, s.log, http.StatusNotFound, CodeNotFound, "not found")
	})))
}

// guestVenueResponse is the venue as a guest sees it.
type guestVenueResponse struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

// guestTableNameResponse is the table the guest is sitting at.
type guestTableNameResponse struct {
	Name string `json:"name"`
}

// guestTableResponse is GET /order/api/t/{code}: where the code on the
// table leads.
type guestTableResponse struct {
	Venue guestVenueResponse     `json:"venue"`
	Table guestTableNameResponse `json:"table"`
}

// guestMenuResponse is the venue's menu as a guest reads it: what there is,
// what it costs and what can be chosen. It leaves out what only a till
// needs: stations, courses, tax rates and codes.
type guestMenuResponse struct {
	Version        int64                   `json:"version"`
	Currency       string                  `json:"currency"`
	Categories     []categoryResponse      `json:"categories"`
	Items          []guestItemResponse     `json:"items"`
	ModifierGroups []modifierGroupResponse `json:"modifier_groups"`
}

// guestItemResponse is one item on the guest's menu.
type guestItemResponse struct {
	ID             int64        `json:"id"`
	CategoryID     int64        `json:"category_id"`
	Name           string       `json:"name"`
	Price          money.Amount `json:"price"`
	ModifierGroups []int64      `json:"modifier_groups"`
	// SoldOut is true while the item cannot be ordered. The guest is not
	// told until when: that is the kitchen's guess, not a promise.
	SoldOut bool `json:"sold_out,omitempty"`
	// Photo is a stored photo's name, served at /order/api/photos/{name}.
	Photo       string    `json:"photo,omitempty"`
	Description string    `json:"description,omitempty"`
	Diet        menu.Diet `json:"diet,omitempty"`
	GlutenFree  bool      `json:"gluten_free,omitempty"`
	Popular     bool      `json:"popular,omitempty"`
}

// guestTableFor reads the code in the path, and answers for one that leads
// nowhere.
func (s *Server) guestTableFor(w http.ResponseWriter, r *http.Request) (tablecodes.Table, bool) {
	table, err := s.guests.Resolve(r.Context(), r.PathValue("code"))
	if errors.Is(err, tablecodes.ErrUnknown) {
		// One answer for a code never made, one turned off and one whose
		// table is gone, and words a guest can act on.
		writeError(w, s.log, http.StatusNotFound, CodeTableCodeUnknown,
			"This code does not lead to a table. Scan the code on your table again, or ask a member of staff.")
		return tablecodes.Table{}, false
	}
	if err != nil {
		s.log.Error("reading a table code failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return tablecodes.Table{}, false
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID = table.VenueID
	}
	return table, true
}

func (s *Server) guestTable(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	// A code can be turned off at any moment: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, guestTableResponse{
		Venue: guestVenueResponse{Name: table.VenueName, Currency: table.Currency},
		Table: guestTableNameResponse{Name: table.TableName},
	})
}

func (s *Server) guestMenu(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	m, err := s.menus.Menu(r.Context(), table.VenueID)
	if err != nil {
		s.log.Error("reading the menu for a guest failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	// Not the version alone: whether an item is sold out turns on the
	// clock, so the same version can read differently an hour apart. The
	// phone asks each time, and it is one small answer.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, toGuestMenuResponse(m, s.now()))
}

func toGuestMenuResponse(m menu.Menu, now time.Time) guestMenuResponse {
	whole := toMenuResponse(m)
	out := guestMenuResponse{
		Version: whole.Version, Currency: whole.Currency,
		Categories:     whole.Categories,
		ModifierGroups: whole.ModifierGroups,
		Items:          make([]guestItemResponse, len(whole.Items)),
	}
	for i, it := range whole.Items {
		out.Items[i] = guestItemResponse{
			ID: it.ID, CategoryID: it.CategoryID, Name: it.Name, Price: it.Price, ModifierGroups: it.ModifierGroups,
			SoldOut: it.SoldOutUntil != nil && now.Before(*it.SoldOutUntil),
			Photo:   it.Photo, Description: it.Description, Diet: it.Diet, GlutenFree: it.GlutenFree, Popular: it.Popular,
		}
	}
	return out
}
