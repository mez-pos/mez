package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/kitchen"
)

// TestKitchenOverHTTP: a kitchen display, with no one signed in, reads its
// station's board, bumps and recalls; a terminal cannot; refusals have
// their codes (ADR-090).
func TestKitchenOverHTTP(t *testing.T) {
	r := newRegister(t)
	code, err := r.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Pass", DeviceKind: auth.KindKitchen})
	if err != nil {
		t.Fatal(err)
	}
	kds := bearerOf(pairOK(t, r.handler, code.Code).Token)

	id := r.openOrder(t) // table 4
	for _, l := range []string{
		`{"item_id":1,"name":"Masala dosa","qty":2,"unit_price":{"minor":12000,"currency":"INR"},"station":"kitchen","course":1,"tax_rate":5000,"hsn":"996331","modifiers":[{"modifier_id":3,"name":"Extra chutney","price":{"minor":0,"currency":"INR"}}]}`,
		`{"item_id":2,"name":"Filter coffee","qty":1,"unit_price":{"minor":4000,"currency":"INR"},"station":"bar","course":1,"tax_rate":5000,"hsn":"996331"}`,
	} {
		r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", l)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/notes", `{"scope":"line","line_id":2,"text":"no onion"}`)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/send", ``)

	board := func(method, path string) (int, kitchenBoardResponse, Error) {
		t.Helper()
		rec := send(r.handler, method, path, "", kds)
		if rec.Code >= 400 {
			return rec.Code, kitchenBoardResponse{}, errorOf(t, rec)
		}
		var out kitchenBoardResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := board(http.MethodGet, "/api/v1/kitchen/kitchen/tickets")
	if status != http.StatusOK || len(out.Tickets) != 1 {
		t.Fatalf("kitchen board: %d %+v", status, out)
	}
	tk := out.Tickets[0]
	if tk.Table != "4" || tk.Status != kitchen.TicketFired || tk.Covers != 2 || len(tk.Lines) != 1 ||
		tk.Lines[0].Name != "Masala dosa" || tk.Lines[0].Qty != 2 || tk.Lines[0].Modifiers[0] != "Extra chutney" || tk.Lines[0].Notes[0] != "no onion" {
		t.Errorf("ticket = %+v", tk)
	}
	if _, bar, _ := board(http.MethodGet, "/api/v1/kitchen/bar/tickets"); len(bar.Tickets) != 1 || bar.Tickets[0].Lines[0].Name != "Filter coffee" {
		t.Errorf("bar board = %+v", bar.Tickets)
	}

	bump := fmt.Sprintf("/api/v1/kitchen/orders/%s/tickets/%d/bump", id, tk.TicketSeq)
	if _, out, _ = board(http.MethodPost, bump); out.Tickets[0].Status != kitchen.TicketReady || out.Tickets[0].ReadyAt == nil || out.RecallMinutes != 30 {
		t.Errorf("after the bump: %+v", out)
	}
	if status, _, apiErr := board(http.MethodPost, bump); status != http.StatusConflict || apiErr.Code != CodeInvalidTransition {
		t.Errorf("bumped twice: %d %s", status, apiErr.Code)
	}
	recall := fmt.Sprintf("/api/v1/kitchen/orders/%s/tickets/%d/recall", id, tk.TicketSeq)
	if _, out, _ = board(http.MethodPost, recall); out.Tickets[0].Status != kitchen.TicketFired || out.Tickets[0].ReadyAt != nil {
		t.Errorf("after the recall: %+v", out.Tickets[0])
	}
	if status, _, apiErr := board(http.MethodPost, fmt.Sprintf("/api/v1/kitchen/orders/%s/tickets/99/bump", id)); status != http.StatusNotFound || apiErr.Code != CodeTicketNotFound {
		t.Errorf("no such ticket: %d %s", status, apiErr.Code)
	}

	// The terminal's view of the order carries the ticket's status too.
	var read orderResponse
	rec := send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", r.asStaff(r.staffToken))
	if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil || len(read.Tickets) != 2 || read.Tickets[0].Status != kitchen.TicketFired {
		t.Errorf("order tickets = %+v (%v)", read.Tickets, err)
	}

	// A terminal is not a kitchen display.
	if rec := send(r.handler, http.MethodGet, "/api/v1/kitchen/kitchen/tickets", "", r.asStaff(r.staffToken)); rec.Code != http.StatusForbidden {
		t.Errorf("a terminal read the board: %d", rec.Code)
	}
}
