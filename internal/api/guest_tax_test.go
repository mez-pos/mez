package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// taxAddedSays reads tax_added where a guest's phone and the till read it:
// the table's landing, the pickup code's, and the list of requests.
func (w *requestsWorld) taxAddedSays(t *testing.T) (table, pickup, requests bool) {
	t.Helper()
	status, _, body := w.guest(http.MethodGet, "/order/api/t/"+w.code)
	var landing guestTableResponse
	if err := json.Unmarshal([]byte(body), &landing); status != http.StatusOK || err != nil {
		t.Fatalf("the table's landing: %d %s", status, body)
	}
	code, err := w.codes.RenewPickup(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	rec := send(w.handler, http.MethodGet, "/order/api/p/"+code, "", nil)
	var collect guestPickupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &collect); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("the pickup code's landing: %d %s", rec.Code, rec.Body)
	}
	rec = w.staffCall(http.MethodGet, "/api/v1/guest-requests")
	var list guestRequestsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("the requests: %d %s", rec.Code, rec.Body)
	}
	return landing.Venue.TaxAdded, collect.Venue.TaxAdded, list.TaxAdded
}

// TestTaxAddedFollowsTheVenuesPricing: a cart's total and a table's request
// say "before tax" only where the bill adds it (ADR-386, GST-2, TILL-18).
func TestTaxAddedFollowsTheVenuesPricing(t *testing.T) {
	tests := []struct {
		name    string
		pricing string // "" leaves the venue untaxed
		want    bool
	}{
		{"an untaxed venue adds none", "", false},
		{"GST in the prices", "inclusive", false},
		{"GST on top of the prices", "exclusive", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newRequestsWorld(t)
			if tc.pricing != "" {
				w.giveGST(t, tc.pricing)
			}
			table, pickup, requests := w.taxAddedSays(t)
			if table != tc.want || pickup != tc.want || requests != tc.want {
				t.Errorf("tax_added: table %v, pickup %v, requests %v; want %v", table, pickup, requests, tc.want)
			}
		})
	}
}

// TestTaxAddedWhereTheVenueCannotBeRead: a guest still reaches the landing,
// which says what it said before, a total before tax; the till is told what
// is wrong, as for its other reads of the venue.
func TestTaxAddedWhereTheVenueCannotBeRead(t *testing.T) {
	w := newRequestsWorld(t)
	ctx := context.Background()
	if err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: svcorder.SettingTaxPricing, Value: "sometimes", UpdatedAt: seededAt})
	}); err != nil {
		t.Fatal(err)
	}

	status, _, body := w.guest(http.MethodGet, "/order/api/t/"+w.code)
	var landing guestTableResponse
	if err := json.Unmarshal([]byte(body), &landing); status != http.StatusOK || err != nil || !landing.Venue.TaxAdded {
		t.Errorf("the landing: %d %s, want 200 with tax_added", status, body)
	}
	rec := w.staffCall(http.MethodGet, "/api/v1/guest-requests")
	if got := errorOf(t, rec); rec.Code != http.StatusConflict || got.Code != CodeVenueNotConfigured {
		t.Errorf("the requests: %d %s, want 409 %s", rec.Code, rec.Body, CodeVenueNotConfigured)
	}
}
