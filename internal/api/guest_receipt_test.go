package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

func (o *orderingWorld) receipt(code, bill string) *httptest.ResponseRecorder {
	return o.phoneSend(http.MethodGet, "/order/api/t/"+code+"/receipts/"+bill, "", nil)
}

var counterStaff = svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}

// TestGuestKeepsAReceipt: the bill names itself while it is open; once it is
// paid the phone asks for its receipt by that name, and is given the
// counter's own invoice with nothing of the counterStaff in it.
func TestGuestKeepsAReceipt(t *testing.T) {
	o := newOrderingWorld(t)
	ctx := context.Background()
	seated := o.seat(t)
	placed := decodeRound(t, o.place("k1", 2))

	bill, _ := o.bill(t)
	if bill.Bill != seated.PublicID || bill.Bill == "" {
		t.Fatalf("the open bill is named %q, want %s", bill.Bill, seated.PublicID)
	}
	// The order says which bill it is on, so a phone that never opened the
	// bill can still ask for the receipt.
	if tracked := o.tracked(t, placed.ID); tracked.Bill != seated.PublicID {
		t.Errorf("the order is on bill %q, want %s", tracked.Bill, seated.PublicID)
	}
	if rec := o.receipt(o.code, bill.Bill); rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeReceiptNotFound {
		t.Fatalf("a receipt for a bill still owing: %d %s", rec.Code, rec.Body)
	}

	paid, err := o.orders.TenderPayment(ctx, counterStaff, seated.OrderID, svcorder.TenderPayment{
		Method: order.PaymentUPI, Amount: bill.BalanceDue, Reference: "upi-ref-4471",
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := o.receipt(o.code, bill.Bill)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("the receipt: %d %s", rec.Code, rec.Body)
	}
	var got guestReceiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.InvoiceNo != paid.State.Invoices[0].InvoiceNo || got.InvoiceNo == "" || got.Bill != bill.Bill || got.TimeZone == "" || got.IssuedAt.IsZero() {
		t.Errorf("receipt %q for %q, issued %v in %q", got.InvoiceNo, got.Bill, got.IssuedAt, got.TimeZone)
	}
	if got.Seller.Name == "" || len(got.Lines) != 1 || got.Lines[0].Qty != 2 || len(got.Lines[0].Choices) != 1 || got.Lines[0].Amount != bill.Lines[0].Amount {
		t.Errorf("seller %+v, lines %+v", got.Seller, got.Lines)
	}
	if got.Total != bill.Total || got.Items != bill.Items || got.TaxIncluded != bill.TaxIncluded || len(got.Taxes) != len(bill.Taxes) {
		t.Errorf("the receipt's money is not the bill's: %+v against %+v", got, bill)
	}
	if len(got.Payments) != 1 || got.Payments[0].Method != order.PaymentUPI || got.Payments[0].Amount != bill.Total {
		t.Errorf("payments = %+v", got.Payments)
	}
	zero := money.Amount{Currency: bill.Total.Currency}
	if got.Credited != zero || got.Refunded != zero {
		t.Errorf("credited %v, refunded %v", got.Credited, got.Refunded)
	}
	// Nothing of the counterStaff: no payment's reference, no ids of lines or staff.
	for _, leak := range []string{"upi-ref-4471", "reference", "line_id", "actor", "staff", "device", "tendered_by", "seq", "table_id"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("the receipt says %q: %s", leak, rec.Body)
		}
	}

	// Closed, the bill reads "nothing open", and the receipt is still there.
	if _, err := o.orders.Close(ctx, counterStaff, seated.OrderID); err != nil {
		t.Fatal(err)
	}
	if after, _ := o.bill(t); after.Open || after.Bill != "" {
		t.Errorf("the bill after the close = %+v", after)
	}
	if again := o.receipt(o.code, bill.Bill); again.Code != http.StatusOK {
		t.Errorf("the receipt after the close: %d %s", again.Code, again.Body)
	}
}

// TestAReceiptIsItsTablesOwn: a table's code reads its own bills. Another
// table's, a bill that is not there, and a code that leads nowhere each
// find nothing.
func TestAReceiptIsItsTablesOwn(t *testing.T) {
	o := newOrderingWorld(t)
	ctx := context.Background()
	seated := o.seat(t)
	o.place("k1", 1)
	bill, _ := o.bill(t)
	if _, err := o.orders.TenderPayment(ctx, counterStaff, seated.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: bill.BalanceDue}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		code string
		bill string
		want ErrorCode
	}{
		{"another table's code", o.tableCode(t, 5), bill.Bill, CodeReceiptNotFound},
		{"no such bill", o.code, "ord_NOSUCH01", CodeReceiptNotFound},
		{"a code that leads nowhere", "nosuchcode22", bill.Bill, CodeTableCodeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := o.receipt(tt.code, tt.bill)
			if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != tt.want {
				t.Errorf("%d %s, want 404 %s", rec.Code, rec.Body, tt.want)
			}
		})
	}
}

// TestGuestReceiptNamesTheRegistration: the receipt carries the profile its
// invoice was issued under, so the phone says "GSTIN" as the paper one does
// (GST-15).
func TestGuestReceiptNamesTheRegistration(t *testing.T) {
	o := newOrderingWorld(t)
	o.giveGST(t, "inclusive")
	seated := o.seat(t)
	decodeRound(t, o.place("k1", 1))
	bill, _ := o.bill(t)
	if _, err := o.orders.TenderPayment(context.Background(), counterStaff, seated.OrderID, svcorder.TenderPayment{
		Method: order.PaymentUPI, Amount: bill.BalanceDue, Reference: "upi-ref-15",
	}); err != nil {
		t.Fatal(err)
	}
	rec := o.receipt(o.code, bill.Bill)
	var got guestReceiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("the receipt: %d %s", rec.Code, rec.Body)
	}
	if got.Seller.TaxProfile != "gst_in" || got.Seller.Registration != "29ABCDE1234F1Z5" {
		t.Errorf("seller = %+v, want gst_in and its GSTIN", got.Seller)
	}
}
