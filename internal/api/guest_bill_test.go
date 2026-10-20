package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

func (o *orderingWorld) bill(t *testing.T) (guestBillResponse, string) {
	t.Helper()
	status, header, body := o.guest(http.MethodGet, "/order/api/t/"+o.code+"/bill")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	if header.Get("Cache-Control") != "no-store" {
		t.Error("a bill may be cached, and it moves with every round and payment")
	}
	var got guestBillResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return got, body
}

// giveGST puts the venue on GST with prices that exclude it, as the order
// tests do: it is what makes the tax rows worth reading.
func (o *orderingWorld) giveGST(t *testing.T, pricing string) {
	t.Helper()
	ctx := context.Background()
	err := o.db.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID: 1, Name: "Filter Coffee House", LegalName: "Filter Coffee House",
			Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4,
			TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 1, UpdatedAt: seededAt,
		}); err != nil {
			return err
		}
		for key, value := range map[string]string{svcorder.SettingTaxStateCode: "29", svcorder.SettingTaxPricing: pricing} {
			if err := q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: key, Value: value, UpdatedAt: seededAt}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giving the venue GST: %v", err)
	}
}

var cashier = svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}

// TestGuestBillWithNoTicket: nothing ordered, or the last bill closed. The
// answer is a bill with nothing on it, in the venue's currency, and not an
// error.
func TestGuestBillWithNoTicket(t *testing.T) {
	o := newOrderingWorld(t)
	got, _ := o.bill(t)
	if got.Open || got.Lines == nil || len(got.Lines) != 0 || got.Taxes == nil ||
		got.Total != (money.Amount{Currency: "INR"}) || got.BalanceDue.Currency != "INR" {
		t.Errorf("bill = %+v", got)
	}

	// A request nobody has accepted is not on any bill.
	if rec := o.place("k1", 1); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	if got, _ := o.bill(t); got.Open {
		t.Errorf("a waiting request is on the bill: %+v", got)
	}
}

// TestGuestBillIsTheTablesTicket: everything on the table's ticket, a
// waiter's lines and a guest's alike, with the tax the venue worked out and
// what has been paid.
func TestGuestBillIsTheTablesTicket(t *testing.T) {
	o := newOrderingWorld(t)
	o.giveGST(t, "exclusive")
	ctx := context.Background()
	seated := o.seat(t)

	// Two oat flat whites from the phone: 2 x (180 + 40) = 440.00.
	if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	// And three lines a waiter rang up: one stays, one is comped, one voided.
	add := func(name string, minor int64) int64 {
		res, err := o.orders.AddLine(ctx, cashier, seated.OrderID, svcorder.AddLine{
			ItemID: 99, Name: name, Qty: 1, UnitPrice: money.Amount{Minor: minor, Currency: "INR"}, Station: "bar", TaxRate: 5000, HSN: "996331",
		})
		if err != nil {
			t.Fatalf("adding %s: %v", name, err)
		}
		return res.State.LastSeq
	}
	add("Biscotti", 6000)
	comped := add("Birthday cake", 30000)
	voided := add("Rung up by mistake", 99900)
	if _, err := o.orders.CompLine(ctx, cashier, seated.OrderID, svcorder.CompLine{LineID: comped, ReasonCode: "birthday", AuthorisedBy: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.orders.VoidLine(ctx, cashier, seated.OrderID, svcorder.VoidLine{LineID: voided, ReasonCode: "entered_in_error", AuthorisedBy: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.orders.TenderPayment(ctx, cashier, seated.OrderID, svcorder.TenderPayment{
		Method: order.PaymentCash, Amount: money.Amount{Minor: 20000, Currency: "INR"},
	}); err != nil {
		t.Fatal(err)
	}

	got, body := o.bill(t)
	if !got.Open || got.TaxIncluded {
		t.Fatalf("bill = %+v", got)
	}
	if len(got.Lines) != 3 {
		t.Fatalf("%d lines, want the flat whites, the biscotti and the comped cake: %+v", len(got.Lines), got.Lines)
	}
	white, cake := got.Lines[0], got.Lines[2]
	if white.Name != "Flat white" || white.Qty != 2 || white.Amount.Minor != 44000 || len(white.Choices) != 1 || white.Choices[0] != "Oat" || white.Comped {
		t.Errorf("the guest's line = %+v", white)
	}
	if cake.Name != "Birthday cake" || !cake.Comped {
		t.Errorf("the comped line = %+v", cake)
	}
	// 440 + 60 = 500.00 charged; 5% GST is 12.50 CGST and 12.50 SGST.
	if got.Items.Minor != 50000 || got.Discounts.Minor != 0 || got.Total.Minor != 52500 {
		t.Errorf("items %v, discounts %v, total %v", got.Items, got.Discounts, got.Total)
	}
	if len(got.Taxes) != 2 || got.Taxes[0].Name != "CGST" || got.Taxes[0].Rate != 2500 || got.Taxes[0].Amount.Minor != 1250 || got.Taxes[1].Name != "SGST" {
		t.Errorf("taxes = %+v", got.Taxes)
	}
	if got.Paid.Minor != 20000 || got.BalanceDue.Minor != 32500 {
		t.Errorf("paid %v, due %v", got.Paid, got.BalanceDue)
	}
	// What only a till needs stays at the till, the voided line with it.
	for _, word := range []string{"Rung up by mistake", "order_id", `"id"`, "hsn", "station", "reason", "authorised", "actor", "tender"} {
		if strings.Contains(body, word) {
			t.Errorf("the bill carries %q: %s", word, body)
		}
	}
}

// TestGuestBillWithTaxInThePrices: the rows explain the total and the phone
// is told not to add them to it.
func TestGuestBillWithTaxInThePrices(t *testing.T) {
	o := newOrderingWorld(t)
	o.giveGST(t, "inclusive")
	o.seat(t)
	if rec := o.place("k1", 1); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	got, _ := o.bill(t)
	if !got.TaxIncluded || got.Items.Minor != 22000 || got.Total.Minor != 22000 || len(got.Taxes) != 2 {
		t.Errorf("bill = %+v", got)
	}
}

func TestGuestBillForACodeThatLeadsNowhere(t *testing.T) {
	o := newOrderingWorld(t)
	status, _, body := o.guest(http.MethodGet, "/order/api/t/abcdefghjkmn/bill")
	if status != http.StatusNotFound || !strings.Contains(body, string(CodeTableCodeUnknown)) {
		t.Errorf("status %d, body %s", status, body)
	}
}
