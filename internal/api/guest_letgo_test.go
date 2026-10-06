package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/service/charges"
)

func (c *chargeWorld) letGo(code, id string) *httptest.ResponseRecorder {
	return c.phoneSend(http.MethodPost, "/order/api/t/"+code+"/bill/charge/"+id+"/let-go", "", nil)
}

// TestGuestGivesUpAPaymentPage: a guest looked at Pay online and came back.
// Their page held the whole bill; given up, it holds none of it, the phone
// is no longer sent to it, and the table can split.
func TestGuestGivesUpAPaymentPage(t *testing.T) {
	c := newChargeWorld(t, "")
	whole := decodeCharge(t, c.start())
	if s := c.billFor(t, phoneA).Split; s == nil || !s.Unclaimed.IsZero() {
		t.Fatalf("with the whole bill's page open, a phone can take %+v", s)
	}

	rec := c.letGo(c.code, whole.ID)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("letting go: %d %s", rec.Code, rec.Body)
	}
	got := decodeCharge(t, rec)
	if !got.LetGo || got.Status != charges.Open || got.URL != "" || got.ID != whole.ID {
		t.Errorf("let go = %+v, want it open, let go and with no page to go to", got)
	}
	// Asked how it stands, it says the same; and twice is once.
	if asked := decodeCharge(t, c.ask(whole.ID)); !asked.LetGo || asked.URL != "" || asked.Status != charges.Open {
		t.Errorf("asked after letting go = %+v", asked)
	}
	if again := c.letGo(c.code, whole.ID); again.Code != http.StatusOK || !decodeCharge(t, again).LetGo {
		t.Errorf("letting go again: %d %s", again.Code, again.Body)
	}

	if s := c.billFor(t, phoneA).Split; s == nil || s.Unclaimed != inr(44000) || s.Rest != inr(44000) {
		t.Errorf("after letting go, the bill splits as %+v", s)
	}
	if share := c.share(phoneA, `"people":2`); share.Code != http.StatusCreated || decodeCharge(t, share).Amount != inr(22000) {
		t.Errorf("a half after letting go: %d %s", share.Code, share.Body)
	}
	// Paying online again is a new page.
	if rest := decodeCharge(t, c.start()); rest.ID == whole.ID || rest.Amount != inr(22000) || rest.URL == "" || rest.LetGo {
		t.Errorf("the rest = %+v", rest)
	}
}

// TestGivingUpAPageThatWasPaid: the guest paid and then tapped. They are
// told they paid, and the payment is on the bill.
func TestGivingUpAPageThatWasPaid(t *testing.T) {
	c := newChargeWorld(t, "")
	whole := decodeCharge(t, c.start())
	c.gw.Pay("fake_0001", gateway.UPI)
	got := decodeCharge(t, c.letGo(c.code, whole.ID))
	if got.Status != charges.Paid || got.LetGo {
		t.Errorf("letting go of a paid page = %+v", got)
	}
	if bill, _ := c.bill(t); !bill.BalanceDue.IsZero() {
		t.Errorf("the bill owes %v", bill.BalanceDue)
	}
}

// TestGivingUpIsTheTablesOwn: a table's code lets go of nothing at another
// table, and of nothing that is not there.
func TestGivingUpIsTheTablesOwn(t *testing.T) {
	c := newChargeWorld(t, "")
	whole := decodeCharge(t, c.start())
	other := c.tableCode(t, 5)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"another table's code": c.letGo(other, whole.ID),
		"no such charge":       c.letGo(c.code, "chg_NOSUCH01"),
	} {
		if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeChargeNotFound {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := c.letGo("nosuchcode", whole.ID); rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeTableCodeUnknown {
		t.Errorf("a code that leads nowhere: %d %s", rec.Code, rec.Body)
	}
	if asked := decodeCharge(t, c.ask(whole.ID)); asked.LetGo || asked.URL == "" {
		t.Errorf("the charge after all that = %+v, want it untouched", asked)
	}
}
