package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

func (o *orderingWorld) track(code, round string) *httptest.ResponseRecorder {
	return o.phoneSend(http.MethodGet, "/order/api/t/"+code+"/orders/"+round, "", nil)
}

func (o *orderingWorld) tracked(t *testing.T, round string) guestRoundResponse {
	t.Helper()
	rec := o.track(o.code, round)
	if rec.Code != http.StatusOK {
		t.Fatalf("tracking %s: %d %s", round, rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("how an order stands may be cached, and it moves when the kitchen does")
	}
	return decodeRound(t, rec)
}

// TestGuestTracksAnOrder: a request says it is waiting and nothing of the
// kitchen; accepted, it is with the kitchen; and it follows the bar's own
// ticket from there, as the kitchen display moves it.
func TestGuestTracksAnOrder(t *testing.T) {
	o := newOrderingWorld(t)
	ctx := context.Background()
	placed := decodeRound(t, o.place("k1", 2))

	got := o.tracked(t, placed.ID)
	if got.Status != svcorder.RoundWaiting || got.Progress != "" || got.ID != placed.ID || got.Subtotal != placed.Subtotal ||
		len(got.Lines) != 1 || got.Lines[0].Name != placed.Lines[0].Name {
		t.Fatalf("a waiting order = %+v", got)
	}
	if strings.Contains(o.track(o.code, placed.ID).Body.String(), "progress") {
		t.Error("an order nobody has accepted says something of its progress")
	}

	staff := svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}
	accepted, err := o.orders.AcceptRound(ctx, staff, placed.ID)
	if err != nil {
		t.Fatal(err)
	}
	orderID := accepted.Round.OrderID
	if got := o.tracked(t, placed.ID); got.Status != svcorder.RoundSent || got.Progress != order.ProgressSent {
		t.Fatalf("accepted = %+v, want sent and with the kitchen", got)
	}

	res, err := o.orders.Order(ctx, 1, orderID)
	if err != nil || len(res.State.Tickets) != 1 {
		t.Fatalf("tickets = %+v, %v", res.State.Tickets, err)
	}
	ticket := svcorder.BumpTicket{TicketSeq: res.State.Tickets[0].Seq}
	kds := svcorder.Meta{VenueID: 1, DeviceID: 9}
	if _, err := o.orders.Start(ctx, kds, orderID, ticket); err != nil {
		t.Fatal(err)
	}
	if got := o.tracked(t, placed.ID); got.Progress != order.ProgressPreparing {
		t.Errorf("once the bar has started: %q", got.Progress)
	}
	if _, err := o.orders.Bump(ctx, kds, orderID, ticket); err != nil {
		t.Fatal(err)
	}
	if got := o.tracked(t, placed.ID); got.Progress != order.ProgressReady || got.Status != svcorder.RoundSent {
		t.Errorf("once it is on the pass: %+v", got)
	}
}

// TestADeclinedOrderIsTrackedAsDeclined: and has no progress.
func TestADeclinedOrderIsTrackedAsDeclined(t *testing.T) {
	o := newOrderingWorld(t)
	placed := decodeRound(t, o.place("k1", 1))
	if _, err := o.orders.DeclineRound(context.Background(), svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}, placed.ID); err != nil {
		t.Fatal(err)
	}
	if got := o.tracked(t, placed.ID); got.Status != svcorder.RoundDeclined || got.Progress != "" {
		t.Errorf("declined = %+v", got)
	}
}

// TestAnOrderIsTrackedByItsOwnTable: another table's code, an order that is
// not there and a code that leads nowhere each find nothing.
func TestAnOrderIsTrackedByItsOwnTable(t *testing.T) {
	o := newOrderingWorld(t)
	placed := decodeRound(t, o.place("k1", 1))
	tests := []struct {
		name  string
		code  string
		round string
		want  ErrorCode
	}{
		{"another table's code", o.tableCode(t, 5), placed.ID, CodeRequestNotFound},
		{"no such order", o.code, "rnd_NOSUCH01", CodeRequestNotFound},
		{"a code that leads nowhere", "nosuchcode22", placed.ID, CodeTableCodeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := o.track(tt.code, tt.round)
			if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != tt.want {
				t.Errorf("%d %s, want 404 %s", rec.Code, rec.Body, tt.want)
			}
		})
	}
}
