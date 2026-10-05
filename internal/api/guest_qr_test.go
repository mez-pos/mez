package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/qr"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// TestGuestBillQR: the picture is the bill's own UPI link, for the same
// balance, and there is none when there is nothing to pay by UPI.
func TestGuestBillQR(t *testing.T) {
	o := newOrderingWorld(t)
	path := "/order/api/t/" + o.code + "/bill/upi.svg"

	// No ticket, then a ticket and no address: nothing to draw.
	if status, _, body := o.guest(http.MethodGet, path); status != http.StatusNotFound || !strings.Contains(body, string(CodeNothingToPay)) {
		t.Errorf("with no ticket: %d %s", status, body)
	}
	o.seat(t)
	if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	if status, _, body := o.guest(http.MethodGet, path); status != http.StatusNotFound || !strings.Contains(body, string(CodeNothingToPay)) {
		t.Errorf("with no address: %d %s", status, body)
	}

	svc, err := settings.New(settings.Options{DB: o.db})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdatePayments(context.Background(), 1, settings.PaymentsInput{UPIAddress: "cafe@okbank"}); err != nil {
		t.Fatal(err)
	}

	status, header, body := o.guest(http.MethodGet, path+"?am=44000")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	if header.Get("Content-Type") != "image/svg+xml" || header.Get("Cache-Control") != "no-store" ||
		header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("headers = %v", header)
	}

	// It is the code of the link the bill carries, and no other.
	bill, _ := o.bill(t)
	if bill.UPI == nil {
		t.Fatal("the bill offers no UPI")
	}
	code, err := qr.Encode([]byte(bill.UPI.Link))
	if err != nil {
		t.Fatal(err)
	}
	if body != code.SVG(4) {
		t.Errorf("the picture is not the bill's link drawn:\n%.200s", body)
	}
	if !strings.Contains(bill.UPI.Link, "am=440.00") {
		t.Errorf("link = %s", bill.UPI.Link)
	}

	if status, _, _ := o.guest(http.MethodGet, "/order/api/t/abcdefghjkmn/bill/upi.svg"); status != http.StatusNotFound {
		t.Errorf("a code that leads nowhere: %d", status)
	}
}
