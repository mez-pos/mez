package razorpay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
)

const (
	testKeyID  = "rzp_test_AbCdEf123456"
	testSecret = "s3cr3t-that-must-never-be-seen"
)

var at = time.Date(2026, 10, 6, 5, 0, 0, 0, time.UTC)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// razorpayStub is a stand-in for Razorpay's API: it answers what a test
// tells it to and keeps what it was asked.
type razorpayStub struct {
	t       *testing.T
	status  int
	answer  string
	method  string
	path    string
	body    string
	user    string
	pass    string
	hasAuth bool
	calls   int
}

func (s *razorpayStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.calls++
	s.method, s.path = r.Method, r.URL.Path
	raw, _ := io.ReadAll(r.Body)
	s.body = string(raw)
	s.user, s.pass, s.hasAuth = r.BasicAuth()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.status)
	_, _ = io.WriteString(w, s.answer)
}

func newStub(t *testing.T, status int, answer string) (*Gateway, *razorpayStub) {
	t.Helper()
	stub := &razorpayStub{t: t, status: status, answer: answer}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)
	g, err := New(Options{KeyID: testKeyID, KeySecret: testSecret, BaseURL: srv.URL, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return g, stub
}

// created is Razorpay's own example of a link just made, cut to what
// matters, with a field this adapter has never heard of.
const created = `{
  "id": "plink_ExjpAUN3gVHrPJ", "amount": 16800, "amount_paid": 0, "currency": "INR", "status": "created",
  "description": "Table 4", "reference_id": "chg_7K3M9QX2", "short_url": "https://rzp.io/i/nxrHnLJ",
  "callback_url": "https://cafe.example/order/t/abc#bill", "callback_method": "get",
  "created_at": 1591097057, "expire_by": 1691097057, "payments": null, "accept_partial": false,
  "whatsapp_link": false, "something_new": {"a": 1}
}`

func TestStartMakesALink(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, created)
	got, err := g.Start(context.Background(), gateway.Charge{
		Reference: "chg_7K3M9QX2", Amount: inr(16800), Description: "Table 4", ReturnURL: "https://cafe.example/order/t/abc#bill",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "plink_ExjpAUN3gVHrPJ" || got.URL != "https://rzp.io/i/nxrHnLJ" {
		t.Errorf("checkout = %+v", got)
	}
	if stub.method != http.MethodPost || stub.path != "/v1/payment_links/" {
		t.Errorf("asked %s %s", stub.method, stub.path)
	}
	if !stub.hasAuth || stub.user != testKeyID || stub.pass != testSecret {
		t.Error("the keys did not go as basic auth")
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(stub.body), &sent); err != nil {
		t.Fatalf("the body is not JSON: %s", stub.body)
	}
	want := map[string]any{
		"amount": float64(16800), "currency": "INR", "reference_id": "chg_7K3M9QX2", "description": "Table 4",
		"callback_url": "https://cafe.example/order/t/abc#bill", "callback_method": "get",
		// Whole or nothing, nobody texted or reminded, and half an hour to pay.
		"accept_partial": false, "reminder_enable": false,
		"expire_by": float64(at.Add(30 * time.Minute).Unix()),
	}
	for k, v := range want {
		if sent[k] != v {
			t.Errorf("%s = %v, want %v", k, sent[k], v)
		}
	}
	if n, ok := sent["notify"].(map[string]any); !ok || n["sms"] != false || n["email"] != false {
		t.Errorf("notify = %v: Razorpay would text or email a guest mez knows nothing of", sent["notify"])
	}
	if strings.Contains(stub.body, testSecret) || strings.Contains(stub.body, testKeyID) {
		t.Error("a key is in the body")
	}
}

func TestStartWithNoReturnAddress(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, created)
	if _, err := g.Start(context.Background(), gateway.Charge{Reference: "chg_1", Amount: inr(16800)}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stub.body, "callback") || strings.Contains(stub.body, `"description"`) {
		t.Errorf("empty fields were sent: %s", stub.body)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   gateway.Status
	}{
		{"just made", created, gateway.Status{State: gateway.Pending}},
		{"paid by card", `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"paid",
			"payments":[{"payment_id":"pay_KBnebtRrhTEatJ","amount":16800,"method":"card","status":"captured","created_at":1661852741}]}`,
			gateway.Status{State: gateway.Paid, Amount: inr(16800), Method: gateway.Card, Reference: "pay_KBnebtRrhTEatJ"}},
		{"paid by UPI", `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"paid",
			"payments":[{"payment_id":"pay_UPI","amount":16800,"method":"upi","status":"captured"}]}`,
			gateway.Status{State: gateway.Paid, Amount: inr(16800), Method: gateway.UPI, Reference: "pay_UPI"}},
		{"paid by net banking", `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"paid",
			"payments":[{"payment_id":"pay_NB","amount":16800,"method":"netbanking","status":"captured"}]}`,
			gateway.Status{State: gateway.Paid, Amount: inr(16800), Method: gateway.Other, Reference: "pay_NB"}},
		{"paid, with a failed try before it left out by Razorpay and the paying one last", `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"paid",
			"payments":[{"payment_id":"pay_OLD","amount":16800,"method":"card","status":"failed"},{"payment_id":"pay_NEW","amount":16800,"method":"upi","status":"captured"}]}`,
			gateway.Status{State: gateway.Paid, Amount: inr(16800), Method: gateway.UPI, Reference: "pay_NEW"}},
		{"paid, and Razorpay names no payment", `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"paid","payments":null}`,
			gateway.Status{State: gateway.Paid, Amount: inr(16800), Method: gateway.Other}},
		// It says what was taken, so the caller can see it is not what was asked.
		{"part paid, which no link of ours allows", `{"id":"plink_1","amount":16800,"amount_paid":5000,"currency":"INR","status":"partially_paid",
			"payments":[{"payment_id":"pay_PART","amount":5000,"method":"upi","status":"captured"}]}`,
			gateway.Status{State: gateway.Paid, Amount: inr(5000), Method: gateway.UPI, Reference: "pay_PART"}},
		{"expired", `{"id":"plink_1","amount":16800,"amount_paid":0,"currency":"INR","status":"expired"}`, gateway.Status{State: gateway.Failed}},
		{"cancelled", `{"id":"plink_1","amount":16800,"amount_paid":0,"currency":"INR","status":"cancelled"}`, gateway.Status{State: gateway.Failed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, stub := newStub(t, http.StatusOK, tt.answer)
			got, err := g.Status(context.Background(), "plink_1")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
			if stub.method != http.MethodGet || stub.path != "/v1/payment_links/plink_1" || stub.body != "" {
				t.Errorf("asked %s %s with %q", stub.method, stub.path, stub.body)
			}
		})
	}
}

// TestAStatusMezDoesNotKnowDecidesNothing: a word Razorpay adds one day is
// not read as paid, and not as failed either.
func TestAStatusMezDoesNotKnowDecidesNothing(t *testing.T) {
	g, _ := newStub(t, http.StatusOK, `{"id":"plink_1","amount":16800,"amount_paid":16800,"currency":"INR","status":"under_review"}`)
	if _, err := g.Status(context.Background(), "plink_1"); !errors.Is(err, gateway.ErrUnavailable) {
		t.Errorf("err = %v, want it undecided", err)
	}
}

func TestAnIDCannotLeaveItsPath(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, created)
	if _, err := g.Status(context.Background(), "plink_1/../../customers?x=1"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stub.path, "/v1/payment_links/") || strings.Contains(stub.path, "customers?") && !strings.Contains(stub.path, "plink_1") {
		t.Errorf("asked %s", stub.path)
	}
	if _, err := g.Status(context.Background(), ""); !errors.Is(err, gateway.ErrRefused) {
		t.Errorf("an empty id: %v", err)
	}
}

func TestFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		answer string
		want   error
		says   string
	}{
		{"the keys are wrong", http.StatusUnauthorized, `{"error":{"code":"BAD_REQUEST_ERROR","description":"Authentication failed"}}`, gateway.ErrRefused, "does not accept the venue's key"},
		{"Razorpay will not take it", http.StatusBadRequest, `{"error":{"code":"BAD_REQUEST_ERROR","description":"The amount must be atleast INR 1.00"}}`, gateway.ErrRefused, "The amount must be atleast INR 1.00"},
		{"a refusal with no words", http.StatusForbidden, `nope`, gateway.ErrRefused, "it answered 403"},
		{"a link that is not there", http.StatusNotFound, `{"error":{"description":"The id provided does not exist"}}`, gateway.ErrRefused, "does not exist"},
		{"Razorpay is down", http.StatusBadGateway, `<html>bad gateway</html>`, gateway.ErrUnavailable, "answered 502"},
		{"Razorpay says slow down", http.StatusTooManyRequests, `{}`, gateway.ErrUnavailable, "answered 429"},
		{"an answer that is not JSON", http.StatusOK, `<html>hello</html>`, gateway.ErrUnavailable, "not the JSON expected"},
		{"a link with no address to pay at", http.StatusOK, `{"id":"plink_1","status":"created","short_url":""}`, gateway.ErrUnavailable, "no link to pay"},
		{"a link whose address is not https", http.StatusOK, `{"id":"plink_1","status":"created","short_url":"javascript:alert(1)"}`, gateway.ErrUnavailable, "no link to pay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _ := newStub(t, tt.status, tt.answer)
			_, err := g.Start(context.Background(), gateway.Charge{Reference: "chg_1", Amount: inr(16800)})
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("%q does not say %q", err, tt.says)
			}
			if strings.Contains(err.Error(), testSecret) || strings.Contains(err.Error(), testKeyID) {
				t.Errorf("a key is in the error: %v", err)
			}
		})
	}
}

// TestRazorpayNotReached: nothing is listening. It is unavailable, and the
// error carries no key.
func TestRazorpayNotReached(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close()
	g, err := New(Options{KeyID: testKeyID, KeySecret: testSecret, BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error {
			_, err := g.Start(context.Background(), gateway.Charge{Reference: "chg_1", Amount: inr(16800)})
			return err
		},
		func() error { _, err := g.Status(context.Background(), "plink_1"); return err },
	} {
		err := call()
		if !errors.Is(err, gateway.ErrUnavailable) {
			t.Errorf("err = %v, want unavailable", err)
		}
		if err != nil && strings.Contains(err.Error(), testSecret) {
			t.Errorf("the secret is in the error: %v", err)
		}
	}
}

func TestACancelledContextStopsTheCall(t *testing.T) {
	g, _ := newStub(t, http.StatusOK, created)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.Status(ctx, "plink_1"); !errors.Is(err, gateway.ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestStartRefusesNothing(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, created)
	if _, err := g.Start(context.Background(), gateway.Charge{Reference: "chg_1", Amount: inr(0)}); !errors.Is(err, gateway.ErrRefused) {
		t.Errorf("a charge for nothing: %v", err)
	}
	if stub.calls != 0 {
		t.Error("Razorpay was asked for a charge of nothing")
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		ok   bool
	}{
		{"both keys", Options{KeyID: testKeyID, KeySecret: testSecret}, true},
		{"no key id", Options{KeySecret: testSecret}, false},
		{"no secret", Options{KeyID: testKeyID}, false},
		{"blank keys", Options{KeyID: "  ", KeySecret: " "}, false},
		{"an address that is not one", Options{KeyID: testKeyID, KeySecret: testSecret, BaseURL: "razorpay"}, false},
		{"an address with another scheme", Options{KeyID: testKeyID, KeySecret: testSecret, BaseURL: "ftp://api.razorpay.com"}, false},
	}
	for _, tt := range tests {
		g, err := New(tt.opts)
		if (err == nil) != tt.ok {
			t.Errorf("%s: err = %v", tt.name, err)
		}
		if err != nil && strings.Contains(err.Error(), testSecret) {
			t.Errorf("%s: the secret is in the error", tt.name)
		}
		if tt.ok && (g.Name() != "razorpay" || g.base != DefaultBaseURL) {
			t.Errorf("%s: name %q, base %q", tt.name, g.Name(), g.base)
		}
	}
}

func TestMethodOf(t *testing.T) {
	for in, want := range map[string]gateway.Method{
		"upi": gateway.UPI, "card": gateway.Card, "netbanking": gateway.Other, "wallet": gateway.Other,
		"emi": gateway.Other, "bank_transfer": gateway.Other, "": gateway.Other,
	} {
		if got := methodOf(in); got != want {
			t.Errorf("methodOf(%q) = %s, want %s", in, got, want)
		}
	}
}
