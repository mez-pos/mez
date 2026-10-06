package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/service/charges"
)

const (
	phoneA = "phone-aaaa"
	phoneB = "phone-bbbb"
	phoneC = "phone-cccc"
)

// share asks for a share of table 4's bill, with body as the JSON after the
// phone's name: `"people":3`, or `"amount":{...}`.
func (c *chargeWorld) share(phone, body string) *httptest.ResponseRecorder {
	return c.phoneSend(http.MethodPost, "/order/api/t/"+c.code+"/bill/charge", fmt.Sprintf(`{"phone":%q,%s}`, phone, body), nil)
}

func inrBody(minor int64) string {
	return fmt.Sprintf(`"amount":{"minor":%d,"currency":"INR"}`, minor)
}

// billFor is the bill as one phone reads it.
func (c *chargeWorld) billFor(t *testing.T, phone string) guestBillResponse {
	t.Helper()
	status, _, body := c.guest(http.MethodGet, "/order/api/t/"+c.code+"/bill?phone="+phone)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	var got guestBillResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return got
}

// payAndSettle has the guest pay the n'th charge the gateway started, and
// the phone ask how it stands.
func (c *chargeWorld) payAndSettle(t *testing.T, n int, id string) {
	t.Helper()
	c.gw.Pay(fmt.Sprintf("fake_%04d", n), gateway.UPI)
	if got := decodeCharge(t, c.ask(id)); got.Status != charges.Paid {
		t.Fatalf("charge %s after paying: %+v", id, got)
	}
}

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// TestTheBillSaysHowItSplits: where a bill can be paid online it says what
// paying the rest asks for, the most this phone can take, and an equal part
// for every number of people the phone may choose. The phone adds nothing up.
func TestTheBillSaysHowItSplits(t *testing.T) {
	c := newChargeWorld(t, "")
	bill := c.billFor(t, phoneA)
	if bill.Split == nil {
		t.Fatal("a bill that can be paid online says nothing of splitting it")
	}
	s := *bill.Split
	if s.Rest != inr(44000) || s.Unclaimed != inr(44000) {
		t.Errorf("rest %v and unclaimed %v, want 440.00 each", s.Rest, s.Unclaimed)
	}
	if len(s.Parts) != maxSplitPeople-1 || s.Parts[0] != (guestSplitPartResponse{People: 2, Amount: inr(22000)}) ||
		s.Parts[1] != (guestSplitPartResponse{People: 3, Amount: inr(14667)}) ||
		s.Parts[len(s.Parts)-1] != (guestSplitPartResponse{People: maxSplitPeople, Amount: inr(2200)}) {
		t.Errorf("parts = %+v", s.Parts)
	}

	// With no name the phone is told the rest, which is all it can ask for.
	if anon, _ := c.bill(t); anon.Split == nil || anon.Split.Rest != inr(44000) || anon.Split.Unclaimed != inr(44000) {
		t.Errorf("the bill with no phone named: %+v", anon.Split)
	}

	t.Run("a venue with no gateway says nothing of it", func(t *testing.T) {
		o := newOrderingWorld(t)
		o.seat(t)
		if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
			t.Fatal(rec.Body)
		}
		if bill, body := o.bill(t); bill.Split != nil {
			t.Errorf("split on a bill that cannot be paid online: %s", body)
		}
	})

	t.Run("a bill with nothing owed says nothing of it", func(t *testing.T) {
		c := newChargeWorld(t, "")
		started := decodeCharge(t, c.start())
		c.payAndSettle(t, 1, started.ID)
		if bill := c.billFor(t, phoneA); bill.Split != nil || bill.Online {
			t.Errorf("a paid bill: online %v, split %+v", bill.Online, bill.Split)
		}
	})
}

// TestGuestPaysAShare: a share is the phone's own charge for an amount, and
// what one phone is paying is not offered to the next.
func TestGuestPaysAShare(t *testing.T) {
	c := newChargeWorld(t, "https://cafe.example")

	rec := c.share(phoneA, inrBody(20000))
	if rec.Code != http.StatusCreated {
		t.Fatalf("a share: %d %s", rec.Code, rec.Body)
	}
	a := decodeCharge(t, rec)
	if a.Amount != inr(20000) || a.Status != charges.Open || a.URL == "" {
		t.Errorf("share = %+v", a)
	}
	if asked := c.gw.Started[0]; asked.Amount != inr(20000) || asked.ReturnURL != "https://cafe.example/order/t/"+c.code {
		t.Errorf("the gateway was asked for %+v", asked)
	}

	// The same phone again is the same charge; another phone is another.
	if again := c.share(phoneA, inrBody(20000)); again.Code != http.StatusOK || decodeCharge(t, again).ID != a.ID {
		t.Errorf("asked again: %d %s", again.Code, again.Body)
	}
	b := c.share(phoneB, inrBody(20000))
	if b.Code != http.StatusCreated || decodeCharge(t, b).ID == a.ID {
		t.Fatalf("another phone's share: %d %s", b.Code, b.Body)
	}

	// 40.00 is left for anyone else, and phone A could still take 240.00.
	if s := c.billFor(t, phoneC).Split; s == nil || s.Rest != inr(4000) || s.Unclaimed != inr(4000) || s.Parts[0].Amount != inr(4000) {
		t.Errorf("as a third phone reads it: %+v", s)
	}
	if s := c.billFor(t, phoneA).Split; s == nil || s.Rest != inr(4000) || s.Unclaimed != inr(24000) {
		t.Errorf("as phone A reads it: %+v", s)
	}

	over := c.share(phoneC, inrBody(5000))
	if over.Code != http.StatusConflict || errorOf(t, over).Code != CodeBillClaimed {
		t.Errorf("50.00 with 40.00 left: %d %s", over.Code, over.Body)
	}
	if len(c.gw.Started) != 2 {
		t.Errorf("the gateway was asked %d times, want 2", len(c.gw.Started))
	}

	// Pay online, with no body, is the rest.
	rest := c.start()
	if rest.Code != http.StatusCreated || decodeCharge(t, rest).Amount != inr(4000) {
		t.Fatalf("the rest: %d %s", rest.Code, rest.Body)
	}
	if none := c.share(phoneC, inrBody(100)); none.Code != http.StatusConflict || errorOf(t, none).Code != CodeBillClaimed {
		t.Errorf("a share with it all being paid: %d %s", none.Code, none.Body)
	}
}

// TestGuestsSplitEvenly: three phones each say three people. The server
// works out each part, and paid in turn they are 146.67, 146.67 and 146.66
// of 440.00: the bill is paid to the paisa.
func TestGuestsSplitEvenly(t *testing.T) {
	c := newChargeWorld(t, "")
	want := []int64{14667, 14667, 14666}
	for i, phone := range []string{phoneA, phoneB, phoneC} {
		if offered := c.billFor(t, phone).Split.Parts[1]; offered != (guestSplitPartResponse{People: 3, Amount: inr(want[i])}) {
			t.Errorf("guest %d is offered %+v, want %v", i+1, offered, inr(want[i]))
		}
		rec := c.share(phone, `"people":3`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("guest %d: %d %s", i+1, rec.Code, rec.Body)
		}
		got := decodeCharge(t, rec)
		if got.Amount != inr(want[i]) {
			t.Errorf("guest %d is asked for %v, want %v", i+1, got.Amount, inr(want[i]))
		}
		c.payAndSettle(t, i+1, got.ID)
	}
	bill, _ := c.bill(t)
	if !bill.BalanceDue.IsZero() || bill.Paid != inr(44000) {
		t.Errorf("after three parts: paid %v, owed %v", bill.Paid, bill.BalanceDue)
	}
	if late := c.share(phoneA, `"people":3`); late.Code != http.StatusConflict || errorOf(t, late).Code != CodeNothingToPay {
		t.Errorf("a part of a paid bill: %d %s", late.Code, late.Body)
	}
}

// TestAShareThatIsRefused: what is wrong with the request is said before
// any gateway is asked.
func TestAShareThatIsRefused(t *testing.T) {
	c := newChargeWorld(t, "")
	tests := []struct {
		name  string
		body  string
		field string
	}{
		{"no phone", `{"people":2}`, "phone"},
		{"a name too short to be a phone's", `{"phone":"a","people":2}`, "phone"},
		{"a name with something odd in it", `{"phone":"phone aaaa!","people":2}`, "phone"},
		{"neither a figure nor a number of people", `{"phone":"phone-aaaa"}`, "amount"},
		{"both a figure and a number of people", `{"phone":"phone-aaaa","people":2,"amount":{"minor":100,"currency":"INR"}}`, "amount"},
		{"among one", `{"phone":"phone-aaaa","people":1}`, "people"},
		{"among too many", `{"phone":"phone-aaaa","people":21}`, "people"},
		{"a figure of nothing", `{"phone":"phone-aaaa","amount":{"minor":0,"currency":"INR"}}`, "amount"},
		{"a figure in another currency", `{"phone":"phone-aaaa","amount":{"minor":100,"currency":"USD"}}`, "amount"},
		{"something else in the body", `{"phone":"phone-aaaa","people":2,"return_url":"https://evil.example"}`, ""},
		{"not JSON", `people=2`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := c.phoneSend(http.MethodPost, "/order/api/t/"+c.code+"/bill/charge", tt.body, nil)
			if got := errorOf(t, rec); rec.Code != http.StatusBadRequest || got.Code != CodeInvalidRequest || got.Field != tt.field {
				t.Errorf("%d %s, want 400 invalid_request on %q", rec.Code, rec.Body, tt.field)
			}
		})
	}
	if len(c.gw.Started) != 0 {
		t.Errorf("the gateway was asked %d times", len(c.gw.Started))
	}

	status, _, body := c.guest(http.MethodGet, "/order/api/t/"+c.code+"/bill?phone=a")
	if status != http.StatusBadRequest {
		t.Errorf("the bill for a phone with no proper name: %d %s", status, body)
	}
}
