package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// lower sets a limit for one test and puts it back. The API's tests do not
// run in parallel, so a package variable is safe to borrow.
func lower(t *testing.T, target *limit, to limit) {
	t.Helper()
	was := *target
	*target = to
	t.Cleanup(func() { *target = was })
}

// TestATablesOrdersAreHeldToAFew: past the limit a table's code places no
// more, is told so in words a guest can act on, and nothing reaches the
// kitchen. Another table is not held back by it.
func TestATablesOrdersAreHeldToAFew(t *testing.T) {
	lower(t, &guestOrderLimit, limit{requests: 3, per: 10 * time.Minute})
	o := newOrderingWorld(t)
	o.seat(t)
	for i := range 3 {
		if rec := o.place(fmt.Sprintf("k%d", i), 1); rec.Code != http.StatusCreated {
			t.Fatalf("order %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := o.place("k-over", 1)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("past the limit: %d %s", rec.Code, rec.Body)
	}
	if got := errorOf(t, rec); got.Code != CodeRateLimited || !strings.Contains(got.Message, "member of staff") {
		t.Errorf("refusal = %+v", got)
	}
	// Sending one that was already placed again is asking too: it is
	// counted, and refused. The order itself stands.
	if again := o.place("k0", 1); again.Code != http.StatusTooManyRequests {
		t.Errorf("an order sent again past the limit: %d", again.Code)
	}
	bill, _ := o.bill(t)
	if len(bill.Lines) != 3 {
		t.Errorf("%d lines on the bill, want the 3 that were placed", len(bill.Lines))
	}

	// Table 5 has its own count.
	other := o.phoneSend(http.MethodPost, "/order/api/t/"+o.tableCode(t, 5)+"/orders",
		fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":1}]}`, o.white, o.oat),
		map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "t5"})
	if other.Code != http.StatusCreated {
		t.Errorf("table 5 after table 4 hit its limit: %d %s", other.Code, other.Body)
	}
}

// TestWhatATableChangesIsCountedApartFromWhatItReads: a table at its limit
// for changing things can still read its bill and track its order, and a
// table reading as fast as its screens ask is never stopped from paying.
func TestWhatATableChangesIsCountedApartFromWhatItReads(t *testing.T) {
	lower(t, &guestWriteLimit, limit{requests: 2, per: time.Minute})
	o := newOrderingWorld(t)
	o.seat(t)
	placed := decodeRound(t, o.place("k1", 1))

	// One change used by the order. A second, then the third is refused.
	paid := func() int {
		return o.phoneSend(http.MethodPost, "/order/api/t/"+o.code+"/bill/paid", "", nil).Code
	}
	if code := paid(); code == http.StatusTooManyRequests {
		t.Fatalf("the second change: %d", code)
	}
	if code := paid(); code != http.StatusTooManyRequests {
		t.Fatalf("the third change: %d, want it refused", code)
	}
	// With changes at their limit the table still reads.
	if rec := o.track(o.code, placed.ID); rec.Code != http.StatusOK {
		t.Fatalf("tracking with changes at their limit: %d %s", rec.Code, rec.Body)
	}

	// Reading has its own limit. Table 5 has read nothing yet: five, then
	// refused, and table 4 reads on.
	lower(t, &guestReadLimit, limit{requests: 5, per: time.Minute})
	five := "/order/api/t/" + o.tableCode(t, 5)
	for i := range 5 {
		if status, _, body := o.guest(http.MethodGet, five); status != http.StatusOK {
			t.Fatalf("read %d: %d %s", i+1, status, body)
		}
	}
	rec := o.phoneSend(http.MethodGet, five, "", nil)
	if rec.Code != http.StatusTooManyRequests || errorOf(t, rec).Code != CodeRateLimited || rec.Header().Get("Retry-After") == "" {
		t.Errorf("the sixth read: %d %s", rec.Code, rec.Body)
	}
	// And it can still change things: a read too many does not stop a table paying.
	if rec := o.phoneSend(http.MethodPost, five+"/bill/paid", "", nil); rec.Code == http.StatusTooManyRequests {
		t.Errorf("a change at a table that has read too much: %d", rec.Code)
	}
}

// TestARenewedCodeIsTheSameTable: the count is the table's, so a new code
// does not start a new one.
func TestARenewedCodeIsTheSameTable(t *testing.T) {
	o := newOrderingWorld(t)
	lower(t, &guestReadLimit, limit{requests: 2, per: time.Minute})
	for range 2 {
		if status, _, _ := o.guest(http.MethodGet, "/order/api/t/"+o.tableCode(t, 5)); status != http.StatusOK {
			t.Fatalf("status %d", status)
		}
	}
	fresh, err := o.codes.Renew(t.Context(), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if status, _, _ := o.guest(http.MethodGet, "/order/api/t/"+fresh); status != http.StatusTooManyRequests {
		t.Errorf("table 5 by its new code, past its limit: %d", status)
	}
}

// TestTheLimitsFitWhatAFullTableAsksByItself: eight phones at one table,
// each tracking an order every ten seconds, reading the bill every fifteen
// and asking about a payment every four, for a minute. The table's limit is
// well over it, and fifty such phones behind one address are under the
// address's. If a screen is made to ask more often, this is what says the
// limits have to move with it.
func TestTheLimitsFitWhatAFullTableAsksByItself(t *testing.T) {
	const perPhone = 60/10 + 60/15 + 60/4 + 1 // and the menu, once a minute
	if table := 8 * perPhone; guestReadLimit.requests < 2*table {
		t.Errorf("a table of eight asks %d a minute by itself, and may read %d: less than twice it", table, guestReadLimit.requests)
	}
	if room := 50 * perPhone; guestLimit.requests < room {
		t.Errorf("fifty phones behind one address ask %d a minute, and the address may ask %d", room, guestLimit.requests)
	}
	if guestOrderLimit.requests < 8 || guestWriteLimit.requests < 16 {
		t.Errorf("a table of eight cannot each order (%d) or each pay in a share with a second thought (%d)", guestOrderLimit.requests, guestWriteLimit.requests)
	}
	if guestBadCodeLimit.requests > guestReadLimit.requests/10 {
		t.Errorf("guessing is allowed %d a minute, too near what a table may read", guestBadCodeLimit.requests)
	}
}
