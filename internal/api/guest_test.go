package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/service/tablecodes"
)

// guestWorld is a venue with a coffee menu, ten tables with codes, and the
// guest app mounted, as cmd/mez wires it.
type guestWorld struct {
	*till
	codes *tablecodes.Service
	// code is table 4's.
	code string
}

func newGuestWorld(t *testing.T) *guestWorld {
	t.Helper()
	w := menuTill(t)
	ctx := context.Background()
	if err := w.db.WriteTx(ctx, func(q *gen.Queries) error { return seedTables(ctx, q, 1) }); err != nil {
		t.Fatalf("seeding tables: %v", err)
	}
	codes, err := tablecodes.New(w.db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := codes.Ensure(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	w.rebuild(t, WithMenus(w.db), WithGuests(codes, "/order/"), WithStatic(map[string]fs.FS{"/order/": app}))
	return &guestWorld{till: w, codes: codes, code: list[3].Code}
}

// guest asks as a phone does: no device, no session, nothing but the path.
func (g *guestWorld) guest(method, path string) (int, http.Header, string) {
	rec := send(g.handler, method, path, "", nil)
	return rec.Code, rec.Header(), rec.Body.String()
}

func TestGuestTable(t *testing.T) {
	g := newGuestWorld(t)
	want, err := g.codes.Resolve(context.Background(), g.code)
	if err != nil {
		t.Fatal(err)
	}

	status, header, body := g.guest(http.MethodGet, "/order/api/t/"+g.code)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	var got guestTableResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got.Venue.Name != want.VenueName || got.Venue.Name == "" || got.Venue.Currency != "INR" || got.Table.Name != "4" {
		t.Errorf("got %+v, want venue %q and table 4", got, want.VenueName)
	}
	if header.Get("Cache-Control") != "no-store" {
		t.Error("where a code leads may be cached, and it can be turned off at any moment")
	}
	// Nothing about the venue's insides: no ids, no other tables.
	for _, word := range []string{"venue_id", "table_id", `"id"`, "seats"} {
		if strings.Contains(body, word) {
			t.Errorf("the answer mentions %s: %s", word, body)
		}
	}
}

func TestGuestMenu(t *testing.T) {
	g := newGuestWorld(t)
	status, header, body := g.guest(http.MethodGet, "/order/api/t/"+g.code+"/menu")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	var m guestMenuResponse
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if m.Currency != "INR" || m.Version == 0 || len(m.Categories) != 1 || len(m.Items) != 1 || len(m.ModifierGroups) != 1 {
		t.Fatalf("menu = %+v", m)
	}
	item := m.Items[0]
	if item.Name != "Flat white" || item.Price.Minor != 18000 || item.Price.Currency != "INR" || len(item.ModifierGroups) != 1 || item.SoldOut {
		t.Errorf("item = %+v", item)
	}
	group := m.ModifierGroups[0]
	if group.Name != "Milk" || group.MinSelect != 1 || len(group.Modifiers) != 2 || group.Modifiers[1].Price.Minor != 4000 {
		t.Errorf("group = %+v", group)
	}
	// What only a till needs stays at the till.
	for _, word := range []string{"tax_rate", "hsn", "station", "course", "tax_exempt", "sold_out_until"} {
		if strings.Contains(body, `"`+word) {
			t.Errorf("the guest's menu carries %s: %s", word, body)
		}
	}
	if header.Get("Cache-Control") != "no-store" {
		t.Error("the guest's menu may be cached, and sold out turns on the clock")
	}
}

func TestGuestCodesThatLeadNowhere(t *testing.T) {
	g := newGuestWorld(t)
	ctx := context.Background()
	list, err := g.codes.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Table 1's sticker is replaced; table 2 is taken off the floor.
	replaced := list[0].Code
	if _, err := g.codes.Renew(ctx, 1, list[0].TableID); err != nil {
		t.Fatal(err)
	}
	removed := list[1].Code
	err = g.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetTableActive(ctx, gen.SetTableActiveParams{Active: false, Now: seededAt, ID: list[1].TableID, VenueID: 1})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		code string
	}{
		{"a code nobody made", strings.Repeat("2", tablecodes.Length)},
		{"a sticker that was replaced", replaced},
		{"a table taken off the floor", removed},
		{"not the shape of a code", "abc"},
		{"the right code in capitals", strings.ToUpper(g.code)},
	}
	for _, tt := range tests {
		for _, suffix := range []string{"", "/menu"} {
			t.Run(tt.name+suffix, func(t *testing.T) {
				status, _, body := g.guest(http.MethodGet, "/order/api/t/"+tt.code+suffix)
				var envelope errorBody
				if status != http.StatusNotFound || json.Unmarshal([]byte(body), &envelope) != nil || envelope.Error.Code != CodeTableCodeUnknown {
					t.Fatalf("status %d, body %s; want 404 %s", status, body, CodeTableCodeUnknown)
				}
				// One answer for all of them, and words a guest can act on.
				if !strings.Contains(envelope.Error.Message, "ask a member of staff") {
					t.Errorf("message = %q", envelope.Error.Message)
				}
			})
		}
	}
	// The table that kept its sticker is untouched by all of that.
	if status, _, _ := g.guest(http.MethodGet, "/order/api/t/"+g.code); status != http.StatusOK {
		t.Errorf("table 4 after its neighbours changed: %d", status)
	}
}

func TestGuestAPIIsNotTheAppAndNotTheStaffAPI(t *testing.T) {
	g := newGuestWorld(t)
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"a call the API does not have", http.MethodGet, "/order/api/nope", http.StatusNotFound},
		{"the API's root", http.MethodGet, "/order/api/", http.StatusNotFound},
		{"a table with something after it", http.MethodGet, "/order/api/t/" + g.code + "/orders", http.StatusNotFound},
		{"writing where there is only reading", http.MethodPost, "/order/api/t/" + g.code, http.StatusNotFound},
		{"the staff API's shape under the guest's", http.MethodGet, "/order/api/v1/orders", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, header, body := g.guest(tt.method, tt.path)
			if status != tt.want {
				t.Fatalf("status %d, want %d: %s", status, tt.want, body)
			}
			// A mistyped call must not be answered with the app's page and
			// a 200, which a phone would try to read as JSON.
			if strings.Contains(body, "the guest app") || !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
				t.Errorf("answered with %q: %s", header.Get("Content-Type"), body)
			}
		})
	}

	// The app's own routes still get the app.
	if status, _, body := g.guest(http.MethodGet, "/order/t/"+g.code); status != http.StatusOK || !strings.Contains(body, "the guest app") {
		t.Errorf("the app at a table's path: %d %s", status, body)
	}
	// And a guest's code opens nothing on the staff API.
	if status, _, _ := g.guest(http.MethodGet, "/api/v1/menu"); status != http.StatusUnauthorized {
		t.Errorf("the staff menu with no device: %d, want 401", status)
	}
}

// TestGuessingCodesIsHeldToAFew: an address may try a handful of codes that
// lead nowhere. Past that it waits, and a right code among the wrong ones is
// not told apart from them.
func TestGuessingCodesIsHeldToAFew(t *testing.T) {
	g := newGuestWorld(t)
	path := "/order/api/t/" + strings.Repeat("2", tablecodes.Length)
	for i := range guestBadCodeLimit.requests {
		if status, _, _ := g.guest(http.MethodGet, path); status != http.StatusNotFound {
			t.Fatalf("guess %d: status %d", i+1, status)
		}
	}
	status, header, body := g.guest(http.MethodGet, path)
	if status != http.StatusTooManyRequests || header.Get("Retry-After") == "" || !strings.Contains(body, string(CodeRateLimited)) {
		t.Fatalf("past the limit: status %d, Retry-After %q, %s", status, header.Get("Retry-After"), body)
	}
	if status, _, _ := g.guest(http.MethodGet, "/order/api/t/"+g.code); status != http.StatusTooManyRequests {
		t.Errorf("a good code from an address that has been guessing: %d", status)
	}
}

// TestAGoodCodeIsNotCountedAsAGuess: a table's own phones, asking as often
// as their screens do, never use up the guesses.
func TestAGoodCodeIsNotCountedAsAGuess(t *testing.T) {
	g := newGuestWorld(t)
	for i := range guestBadCodeLimit.requests * 3 {
		if status, _, body := g.guest(http.MethodGet, "/order/api/t/"+g.code); status != http.StatusOK {
			t.Fatalf("read %d of a good code: %d %s", i+1, status, body)
		}
	}
}

func TestGuestMenuSaysSoldOutByTheClock(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time {
		t := now.Add(d)
		return &t
	}
	price := money.Amount{Minor: 1000, Currency: "INR"}
	m := menu.Menu{Version: 7, Currency: "INR", Items: []menu.Item{
		{ID: 1, Name: "On sale", Price: price},
		{ID: 2, Name: "Sold out until tomorrow", Price: price, SoldOutUntil: at(12 * time.Hour)},
		{ID: 3, Name: "Sold out yesterday, back today", Price: price, SoldOutUntil: at(-time.Minute)},
		{ID: 4, Name: "Back this very second", Price: price, SoldOutUntil: at(0)},
		{ID: 5, Name: "With what the card shows", Price: price, Photo: "abc.jpg", Description: "Slow and dark.", Diet: menu.Diet("veg"), GlutenFree: true, Popular: true},
	}}
	got := toGuestMenuResponse(m, now)

	want := []bool{false, true, false, false, false}
	for i, it := range got.Items {
		if it.SoldOut != want[i] {
			t.Errorf("%s: sold_out = %v, want %v", it.Name, it.SoldOut, want[i])
		}
		if it.ModifierGroups == nil {
			t.Errorf("%s: modifier_groups is null, want a list", it.Name)
		}
	}
	card := got.Items[4]
	if card.Photo != "abc.jpg" || card.Description != "Slow and dark." || card.Diet != "veg" || !card.GlutenFree || !card.Popular {
		t.Errorf("card = %+v", card)
	}
	if got.Version != 7 || got.Currency != "INR" || got.Categories == nil || got.ModifierGroups == nil {
		t.Errorf("menu = %+v, want lists and not nulls", got)
	}
}
