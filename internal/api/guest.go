package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/money"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/tablecodes"
)

// The guest API (phase 11, Task 3a, ADR-206): what a guest's phone asks the
// venue. It lives under the guest app's mount, /order/api/, and not under
// /api/v1: the tunnel puts every guest request under that mount, so this is
// the one API a guest can reach through the relay, and the staff API is the
// one they cannot. On the venue's own wifi it is the same paths.
//
// A guest is nobody: no device and no staff session. What they hold is the
// code on their table, and every route here is under it. The code says which
// table and lasts for ever, so it opens only what any passer-by may see: the
// landing, the menu, and an order or a receipt they already hold the id of.
// The kitchen and the bill also need the phone's session for the sitting
// (ADR-262), sent as X-Guest-Session.

// CodeTableCodeUnknown is a table code that leads nowhere.
const CodeTableCodeUnknown ErrorCode = "table_code_unknown"

// CodeGuestSessionRequired is a phone without a session that is good for the
// table's sitting now (ADR-262).
const CodeGuestSessionRequired ErrorCode = "guest_session_required"

// HeaderGuestSession carries a guest phone's session for the sitting.
const HeaderGuestSession = "X-Guest-Session"

// What a guest may ask, and how often (phase 11, Task 6d, ADR-246). There
// are two things to count by, and neither is enough alone.
//
// An address is who is asking, as far as anyone can tell: a phone on the
// venue's own network has its own, and through the relay the guest's
// address is carried to here (internal/tunnel). But a cafe's wifi is one
// address to the relay, and every phone in the room is behind it, each
// asking about its order every ten seconds (ADR-239). So the address has a
// ceiling a full room does not reach, and the real limits are by table.
//
// A table is what a code can do harm to. What one table may read is a
// multiple of what its phones ask by themselves; what it may change is
// counted apart and held close; and orders closer still.
var (
	// guestLimit is the ceiling for one address over all of the guest API:
	// about fifty phones each tracking an order, reading a bill and
	// waiting on a payment at once. It is there for a flood, not a meal.
	guestLimit = limit{requests: 1500, per: time.Minute}

	// guestBadCodeLimit is how many codes that lead nowhere one address
	// may try. A guest mistypes none: the code comes from a sticker. Past
	// it the address is told to wait, whatever it sends, right or wrong.
	guestBadCodeLimit = limit{requests: 20, per: time.Minute}

	// guestReadLimit is what may be read by one table's code: a table of
	// eight with every screen asking at its fastest is about two hundred a
	// minute, and this is three times that.
	guestReadLimit = limit{requests: 600, per: time.Minute}

	// guestWriteLimit is what one table's code may change: start a
	// payment, give one up, say "I have paid", place an order. A table
	// paying in eight shares with a few second thoughts is under it.
	guestWriteLimit = limit{requests: 30, per: time.Minute}

	// guestOrderLimit is how many orders one table's code may place. Past
	// the first, a table's rounds go straight to the kitchen (ADR-209), so
	// this is what stands between a photographed sticker and forty tickets.
	guestOrderLimit = limit{requests: 20, per: 10 * time.Minute}

	// guestSessionLimit is how many sessions one table may be handed: each
	// is a row, and a phone that keeps its own asks for no other. A table
	// of eight with every phone in a private tab is well under it.
	guestSessionLimit = limit{requests: 60, per: 10 * time.Minute}
)

// Guests is what the API needs to read a table's code, and the venue's
// pickup code (ADR-247).
type Guests interface {
	Resolve(ctx context.Context, code string) (tablecodes.Table, error)
	ResolvePickup(ctx context.Context, code string) (tablecodes.Venue, error)
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
	s.mountPickup(handle, base)
	if s.menus != nil {
		handle("GET "+base+"t/{code}/menu", s.guestMenu)
	}
	if s.guestOrders != nil {
		handle("POST "+base+"t/{code}/orders", s.guestPlaceOrder)
		handle("GET "+base+"t/{code}/orders/{round}", s.guestTrackOrder)
		handle("GET "+base+"t/{code}/bill", s.guestBill)
		handle("GET "+base+"t/{code}/receipts/{bill}", s.guestReceipt)
		handle("GET "+base+"t/{code}/bill/upi.svg", s.guestBillQR)
		handle("POST "+base+"t/{code}/bill/paid", s.guestTellPaid)
		if s.charges != nil {
			handle("POST "+base+"t/{code}/bill/charge", s.guestStartCharge)
			handle("POST "+base+"t/{code}/orders/{round}/charge", s.guestStartRoundCharge)
			handle("GET "+base+"t/{code}/bill/charge/{id}", s.guestCharge)
			handle("POST "+base+"t/{code}/bill/charge/{id}/let-go", s.guestLetGoCharge)
		}
	}
	if s.charges != nil {
		handle("POST "+base+"hook/{gateway}", s.gatewayHook)
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
	// TaxAdded is whether the bill adds tax to the menu's prices (ADR-386):
	// a cart's total is "before tax" only where it is. False where the
	// prices hold the tax, and where the venue charges none.
	TaxAdded bool `json:"tax_added"`
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
	// Session is a new session for the sitting (ADR-262), handed out only
	// while the table has no ticket open and the phone presented none that
	// is still good. The phone sends it as X-Guest-Session from then on.
	Session string `json:"session,omitempty"`
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
	// An address that has tried too many codes that lead nowhere waits,
	// before any code it sends is looked at: a right one among the wrong
	// ones is not told apart.
	addr := remoteHost(r)
	if !s.limiter.within("guest-bad:"+addr, guestBadCodeLimit) {
		s.tooMany(w, "guest-bad:"+addr, guestBadCodeLimit, "Too many codes tried from here. Wait a minute, then scan the code on your table again.")
		return tablecodes.Table{}, false
	}
	table, err := s.guests.Resolve(r.Context(), r.PathValue("code"))
	if errors.Is(err, tablecodes.ErrUnknown) {
		s.limiter.allow("guest-bad:"+addr, guestBadCodeLimit)
		// One answer for a code never made, one turned off and one whose
		// table is gone, and words a guest can act on.
		writeError(w, s.log, http.StatusNotFound, CodeTableCodeUnknown,
			"This code does not lead to a table. Scan the code on your table again, or ask a member of staff.")
		return tablecodes.Table{}, false
	}
	if err != nil {
		s.internalError(w, r, "reading a table code failed", err)
		return tablecodes.Table{}, false
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID = table.VenueID
	}
	// Counted by the table and not by its code: a renewed code is the same
	// table, and starts no new count.
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if !s.guestAllow(w, table, "r", guestReadLimit, "This table is asking too often. Wait a moment.") {
			return tablecodes.Table{}, false
		}
		return table, true
	}
	if !s.guestAllow(w, table, "w", guestWriteLimit, "Too much is being done from this table at once. Wait a moment and try again, or ask a member of staff.") {
		return tablecodes.Table{}, false
	}
	return table, true
}

// guestAllow counts one thing done by a table's code against a limit, and
// has answered when it is over.
func (s *Server) guestAllow(w http.ResponseWriter, table tablecodes.Table, kind string, lim limit, message string) bool {
	key := "guest-" + kind + ":" + strconv.FormatInt(table.VenueID, 10) + ":" + strconv.FormatInt(table.TableID, 10)
	ok, retryAfter := s.limiter.allow(key, lim)
	if ok {
		return true
	}
	// Worth a line: a table at its limit is a guest being refused, or a
	// code that has got out and should be renewed.
	s.log.Warn("a table's code is over its limit", "venue_id", table.VenueID, "table_id", table.TableID, "kind", kind, "limit", lim.requests, "per", lim.per.String())
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
	writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, message)
	return false
}

// tooMany answers a key that is already over its limit, without counting
// the asking.
func (s *Server) tooMany(w http.ResponseWriter, key string, lim limit, message string) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(s.limiter.wait(key, lim))))
	writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, message)
}

func (s *Server) guestTable(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	// A code can be turned off at any moment: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	resp := guestTableResponse{
		Venue: guestVenueResponse{Name: table.VenueName, Currency: table.Currency, TaxAdded: s.guestTaxAdded(r, table.VenueID)},
		Table: guestTableNameResponse{Name: table.TableName},
	}
	session, ok := s.guestOpenSession(w, r, table)
	if !ok {
		return
	}
	resp.Session = session
	writeJSON(w, s.log, http.StatusOK, resp)
}

// guestTaxAdded is whether the venue's bills add tax to its prices. Where
// that cannot be read the landing still opens, and says what it said before
// ADR-386: a total before tax, which never promises a guest less than the
// bill. A venue that cannot be read takes no orders either.
func (s *Server) guestTaxAdded(r *http.Request, venueID int64) bool {
	if s.guestOrders == nil {
		return true
	}
	added, err := s.guestOrders.TaxAdded(r.Context(), venueID)
	if err != nil {
		s.log.Error("reading whether a venue adds tax failed", "venue_id", venueID, "err", err)
		return true
	}
	return added
}

// guestOpenSession hands the phone a session for the sitting when it has
// none that is good and the table has no ticket open (ADR-262), and has
// answered when it could not.
func (s *Server) guestOpenSession(w http.ResponseWriter, r *http.Request, table tablecodes.Table) (string, bool) {
	if s.guestOrders == nil {
		return "", true
	}
	if token := r.Header.Get(HeaderGuestSession); token != "" {
		_, err := s.guestOrders.GuestSession(r.Context(), table.VenueID, table.TableID, token)
		if err == nil {
			return "", true
		}
		if !errors.Is(err, svcorder.ErrGuestSession) {
			s.internalError(w, r, "checking a guest session failed", err, "venue_id", table.VenueID)
			return "", false
		}
	}
	key := "guest-s:" + strconv.FormatInt(table.VenueID, 10) + ":" + strconv.FormatInt(table.TableID, 10)
	if !s.limiter.within(key, guestSessionLimit) {
		s.log.Warn("a table is over its limit of guest sessions", "venue_id", table.VenueID, "table_id", table.TableID)
		s.tooMany(w, key, guestSessionLimit, "This table is asking too often. Wait a moment.")
		return "", false
	}
	token, issued, err := s.guestOrders.OpenGuestSession(r.Context(), table.VenueID, table.TableID)
	if err != nil {
		s.internalError(w, r, "opening a guest session failed", err, "venue_id", table.VenueID)
		return "", false
	}
	if issued {
		s.limiter.allow(key, guestSessionLimit)
	}
	return token, true
}

// guestSessionFor checks the phone's session for the table's sitting, and
// answers for one that is missing or no longer good, in words a guest can
// act on: the app scans the landing again for a new one.
func (s *Server) guestSessionFor(w http.ResponseWriter, r *http.Request, table tablecodes.Table) (svcorder.GuestSession, bool) {
	session, err := s.guestOrders.GuestSession(r.Context(), table.VenueID, table.TableID, r.Header.Get(HeaderGuestSession))
	if errors.Is(err, svcorder.ErrGuestSession) {
		writeGuestSessionRequired(w, s)
		return svcorder.GuestSession{}, false
	}
	if err != nil {
		s.internalError(w, r, "checking a guest session failed", err, "venue_id", table.VenueID)
		return svcorder.GuestSession{}, false
	}
	return session, true
}

func writeGuestSessionRequired(w http.ResponseWriter, s *Server) {
	w.Header().Set("Cache-Control", "no-store")
	writeError(w, s.log, http.StatusUnauthorized, CodeGuestSessionRequired, "Scan the code on your table again to continue.")
}

func (s *Server) guestMenu(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	s.writeGuestMenu(w, r, table.VenueID)
}

// writeGuestMenu answers a venue's menu as a guest reads it, whichever code
// led to the venue.
func (s *Server) writeGuestMenu(w http.ResponseWriter, r *http.Request, venueID int64) {
	m, err := s.menus.Menu(r.Context(), venueID)
	if err != nil {
		s.internalError(w, r, "reading the menu for a guest failed", err, "venue_id", venueID)
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
