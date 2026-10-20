package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/gateway"
	"github.com/mez-pos/mez/internal/service/charges"
)

func decodeSheet(t *testing.T, o *orderingWorld, owner map[string]string) tableCodeSheetResponse {
	t.Helper()
	rec := send(o.handler, http.MethodGet, "/api/v1/table-codes/sheet", "", owner)
	var sheet tableCodeSheetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sheet); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("the sheet: %d %s", rec.Code, rec.Body)
	}
	return sheet
}

// TestWithNoGatewayThePickupCodeSaysItTakesNoOrders: the code can be made,
// and the back office is told it leads to a page that takes no orders. There
// is no card for the door: a poster that leads to a refusal is not printed.
func TestWithNoGatewayThePickupCodeSaysItTakesNoOrders(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)

	made := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/pickup-code/renew", "", owner)).Pickup
	if made.Code == "" || made.Ordering {
		t.Errorf("pickup = %+v, want a code that takes no orders", made)
	}
	sheet := decodeSheet(t, o, owner)
	if sheet.Pickup != nil || len(sheet.Tables) == 0 {
		t.Errorf("the sheet with no gateway: pickup %+v, %d tables", sheet.Pickup, len(sheet.Tables))
	}
}

// TestThePickupCodeIsOnTheSheetWhereOrdersAreTaken: with a gateway the list
// says orders to collect are taken, and the sheet has a card for the door:
// the pickup code's own address, drawn, and no table's.
func TestThePickupCodeIsOnTheSheetWhereOrdersAreTaken(t *testing.T) {
	p := newPrepayWorld(t, "https://cafe.example")
	svc, err := charges.New(charges.Options{
		DB: p.db, Orders: p.orders,
		Gateways: func(context.Context, int64) (gateway.Gateway, error) { return p.gw, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	p.rebuild(t, WithMenus(p.db), WithGuests(p.codes, "/order/"), WithGuestOrders(p.orders), WithCharges(svc, "https://cafe.example"),
		WithTableCodes(p.codes, "https://cafe.example"))
	owner := p.asStaff(p.signInAs(t, "stf_PRIYA001", "2468").Token)

	// No code yet: orders would be taken, and there is nothing to print.
	before := decodeCodes(t, send(p.handler, http.MethodGet, "/api/v1/table-codes", "", owner)).Pickup
	if before.Code != "" || !before.Ordering {
		t.Errorf("before a code is made: %+v", before)
	}
	if sheet := decodeSheet(t, p.orderingWorld, owner); sheet.Pickup != nil {
		t.Errorf("a card for the door with no pickup code: %+v", sheet.Pickup)
	}

	made := decodeCodes(t, send(p.handler, http.MethodPost, "/api/v1/pickup-code/renew", "", owner)).Pickup
	if made.Code == "" || !made.Ordering || made.URL != "https://cafe.example/order/p/"+made.Code {
		t.Fatalf("pickup = %+v", made)
	}
	sheet := decodeSheet(t, p.orderingWorld, owner)
	if sheet.Pickup == nil || sheet.Pickup.URL != made.URL || !strings.HasPrefix(sheet.Pickup.SVG, "<svg") {
		t.Fatalf("the door's card = %+v", sheet.Pickup)
	}
	for _, tb := range sheet.Tables {
		if tb.SVG == sheet.Pickup.SVG || tb.URL == sheet.Pickup.URL {
			t.Errorf("table %d has the door's card", tb.TableID)
		}
	}

	// Turned off, the card is gone and the tables' are as they were.
	decodeCodes(t, send(p.handler, http.MethodPost, "/api/v1/pickup-code/revoke", "", owner))
	if after := decodeSheet(t, p.orderingWorld, owner); after.Pickup != nil || len(after.Tables) != len(sheet.Tables) {
		t.Errorf("after turning it off: pickup %+v, %d tables", after.Pickup, len(after.Tables))
	}
}
