package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// paidWorld is requestsWorld at a venue that takes UPI, with table 4
// seated and two oat flat whites on its ticket: 440.00 owed.
type paidWorld struct {
	*requestsWorld
	ticket svcorder.Result
}

func newPaidWorld(t *testing.T) *paidWorld {
	t.Helper()
	w := newRequestsWorld(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdatePayments(context.Background(), 1, settings.PaymentsInput{UPIAddress: "cafe@okbank"}); err != nil {
		t.Fatal(err)
	}
	ticket := w.seat(t)
	if rec := w.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	return &paidWorld{requestsWorld: w, ticket: ticket}
}

func (p *paidWorld) tell() *httptest.ResponseRecorder {
	return p.phoneSend(http.MethodPost, "/order/api/t/"+p.code+"/bill/paid", "", nil)
}

func (p *paidWorld) notices(t *testing.T) []paidNoticeResponse {
	t.Helper()
	rec := p.staffCall(http.MethodGet, "/api/v1/guest-requests")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing: %d %s", rec.Code, rec.Body)
	}
	var got guestRequestsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	if got.Paid == nil || got.Requests == nil {
		t.Fatalf("a list is missing, not empty: %s", rec.Body)
	}
	return got.Paid
}

// TestGuestSaysTheyPaid: the terminal is told which table and how much,
// the phone's bill says staff have been told, and the bill is no more paid
// than it was.
func TestGuestSaysTheyPaid(t *testing.T) {
	p := newPaidWorld(t)
	if got := p.notices(t); len(got) != 0 {
		t.Fatalf("before anyone said anything: %+v", got)
	}
	if bill, _ := p.bill(t); bill.PaidTold {
		t.Error("the bill says staff were told before anyone told them")
	}

	rec := p.tell()
	if rec.Code != http.StatusCreated {
		t.Fatalf("telling: %d %s", rec.Code, rec.Body)
	}
	var told guestPaidResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &told); err != nil {
		t.Fatal(err)
	}
	if !told.Told || told.Amount != (money.Amount{Minor: 44000, Currency: "INR"}) {
		t.Errorf("answer = %+v", told)
	}
	// Tapped again, it is said once, and answers 200.
	if again := p.tell(); again.Code != http.StatusOK {
		t.Errorf("telling twice: %d %s", again.Code, again.Body)
	}

	got := p.notices(t)
	if len(got) != 1 {
		t.Fatalf("%d notices, want 1", len(got))
	}
	n := got[0]
	if !strings.HasPrefix(n.ID, "pdn_") || n.TableID != 4 || n.TableName != "4" || n.OrderID != p.ticket.PublicID ||
		n.Amount.Minor != 44000 || n.Due.Minor != 44000 || n.ToldAt.IsZero() {
		t.Errorf("notice = %+v", n)
	}

	bill, body := p.bill(t)
	if !bill.PaidTold {
		t.Errorf("the bill does not say staff have been told: %s", body)
	}
	if bill.Paid.Minor != 0 || bill.BalanceDue.Minor != 44000 || bill.UPI == nil {
		t.Errorf("saying so changed the bill: paid %v, due %v, upi %v", bill.Paid, bill.BalanceDue, bill.UPI)
	}
}

// TestStaffRecordThePayment: the payment staff record on the terminal is
// what settles the bill, and with it the notice has nothing left to say.
func TestStaffRecordThePayment(t *testing.T) {
	p := newPaidWorld(t)
	if rec := p.tell(); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	if _, err := p.orders.TenderPayment(context.Background(), cashier, p.ticket.OrderID, svcorder.TenderPayment{
		Method: order.PaymentUPI, Amount: money.Amount{Minor: 44000, Currency: "INR"}, Reference: "UTR 123",
	}); err != nil {
		t.Fatal(err)
	}
	if got := p.notices(t); len(got) != 0 {
		t.Errorf("still told after the payment was recorded: %+v", got)
	}
	bill, _ := p.bill(t)
	if bill.PaidTold || bill.BalanceDue.Minor != 0 || bill.Paid.Minor != 44000 || bill.UPI != nil {
		t.Errorf("the settled bill: told %v, paid %v, due %v, upi %v", bill.PaidTold, bill.Paid, bill.BalanceDue, bill.UPI)
	}
	// Nothing left to pay, so nothing left to say.
	if rec := p.tell(); rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNothingToPay {
		t.Errorf("telling on a settled bill: %d %s", rec.Code, rec.Body)
	}
}

// TestStaffPutANoticeAway: they looked and no money had come.
func TestStaffPutANoticeAway(t *testing.T) {
	p := newPaidWorld(t)
	if rec := p.tell(); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	id := p.notices(t)[0].ID
	path := "/api/v1/paid-notices/" + id + "/seen"

	if rec := send(p.handler, http.MethodPost, path, "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("nobody puts it away: %d", rec.Code)
	}
	if rec := send(p.handler, http.MethodPost, "/order"+path, "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("a guest reaches for it under their own mount: %d", rec.Code)
	}
	if len(p.notices(t)) != 1 {
		t.Fatal("the notice was put away by someone who may not")
	}

	if rec := p.staffCall(http.MethodPost, path); rec.Code != http.StatusNoContent {
		t.Fatalf("putting it away: %d %s", rec.Code, rec.Body)
	}
	if got := p.notices(t); len(got) != 0 {
		t.Errorf("still listed: %+v", got)
	}
	if bill, _ := p.bill(t); bill.PaidTold || bill.BalanceDue.Minor != 44000 {
		t.Errorf("after it was put away: told %v, due %v", bill.PaidTold, bill.BalanceDue)
	}
	if rec := p.staffCall(http.MethodPost, path); rec.Code != http.StatusNoContent {
		t.Errorf("put away twice: %d", rec.Code)
	}
	unknown := p.staffCall(http.MethodPost, "/api/v1/paid-notices/pdn_NOPE0000/seen")
	if unknown.Code != http.StatusNotFound || errorOf(t, unknown).Code != CodeNoticeNotFound {
		t.Errorf("a notice that does not exist: %d %s", unknown.Code, unknown.Body)
	}
	// The guest can say so again.
	if rec := p.tell(); rec.Code != http.StatusCreated {
		t.Errorf("saying so again: %d %s", rec.Code, rec.Body)
	}
}

func TestGuestCannotSayPaidWithNothingToPay(t *testing.T) {
	// A venue with no UPI address.
	o := newOrderingWorld(t)
	o.seat(t)
	if rec := o.place("k1", 1); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	rec := o.phoneSend(http.MethodPost, "/order/api/t/"+o.code+"/bill/paid", "", nil)
	if rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNothingToPay {
		t.Errorf("with no UPI address: %d %s", rec.Code, rec.Body)
	}
	// A code that leads nowhere.
	if rec := o.phoneSend(http.MethodPost, "/order/api/t/abcdefghjkmn/bill/paid", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("a code that leads nowhere: %d", rec.Code)
	}
}
