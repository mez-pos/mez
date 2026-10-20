package stripe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/gateway"
)

const testKey = "sk_test_s3cr3t-that-must-never-be-seen"

var at = time.Date(2026, 10, 6, 5, 0, 0, 0, time.UTC)

func amount(minor int64, currency string) money.Amount {
	return money.Amount{Minor: minor, Currency: currency}
}

// stripeStub is a stand-in for Stripe's API: it answers what a test tells
// it to and keeps what it was asked.
type stripeStub struct {
	status int
	answer string

	method      string
	path        string
	query       url.Values
	form        url.Values
	auth        string
	contentType string
	idempotency string
	calls       int
}

func (s *stripeStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.calls++
	s.method, s.path, s.query = r.Method, r.URL.Path, r.URL.Query()
	raw, _ := io.ReadAll(r.Body)
	s.form, _ = url.ParseQuery(string(raw))
	s.auth = r.Header.Get("Authorization")
	s.contentType = r.Header.Get("Content-Type")
	s.idempotency = r.Header.Get("Idempotency-Key")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.status)
	_, _ = io.WriteString(w, s.answer)
}

func newStub(t *testing.T, status int, answer string) (*Gateway, *stripeStub) {
	t.Helper()
	stub := &stripeStub{status: status, answer: answer}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)
	g, err := New(Options{SecretKey: testKey, BaseURL: srv.URL, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return g, stub
}

var charge = gateway.Charge{
	Reference: "chg_7K3M9QX2", Amount: amount(16800, "INR"),
	Description: "Table T5, Filter Coffee House", ReturnURL: "https://cafe.example/order/t/abc",
}

const opened = `{"id":"cs_test_a1B2","object":"checkout.session","status":"open","payment_status":"unpaid",
"amount_total":16800,"currency":"inr","payment_intent":null,"url":"https://checkout.stripe.com/c/pay/cs_test_a1B2#frag"}`

func TestNew(t *testing.T) {
	if _, err := New(Options{SecretKey: "  "}); err == nil {
		t.Error("an adapter with no key was built")
	}
	if _, err := New(Options{SecretKey: testKey, BaseURL: "not an address"}); err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("a bad address: %v", err)
	}
	g, err := New(Options{SecretKey: " " + testKey + " "})
	if err != nil || g.Name() != "stripe" || g.base != DefaultBaseURL || g.secretKey != testKey {
		t.Errorf("g = %+v, %v", g, err)
	}
}

// TestStartMakesOneSession: what Stripe is asked for, exactly, and what is
// made of its answer.
func TestStartMakesOneSession(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, opened)
	got, err := g.Start(context.Background(), charge)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "cs_test_a1B2" || got.URL != "https://checkout.stripe.com/c/pay/cs_test_a1B2#frag" {
		t.Errorf("checkout = %+v", got)
	}
	if stub.method != http.MethodPost || stub.path != "/v1/checkout/sessions" || stub.calls != 1 {
		t.Errorf("asked %s %s, %d times", stub.method, stub.path, stub.calls)
	}
	if stub.auth != "Bearer "+testKey || stub.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("auth %q, content type %q", stub.auth, stub.contentType)
	}
	// The charge's own id is the idempotency key: asked twice, one session.
	if stub.idempotency != "chg_7K3M9QX2" {
		t.Errorf("idempotency key %q", stub.idempotency)
	}
	want := map[string]string{
		"mode":                                "payment",
		"success_url":                         "https://cafe.example/order/t/abc",
		"cancel_url":                          "https://cafe.example/order/t/abc",
		"client_reference_id":                 "chg_7K3M9QX2",
		"metadata[mez_charge]":                "chg_7K3M9QX2",
		"expires_at":                          "1791264660", // 05:31:00 UTC, thirty-one minutes on
		"line_items[0][quantity]":             "1",
		"line_items[0][price_data][currency]": "inr",
		// Minor units, as mez counts: 168.00 is 16800, and no float is near it.
		"line_items[0][price_data][unit_amount]":        "16800",
		"line_items[0][price_data][product_data][name]": "Table T5, Filter Coffee House",
	}
	for k, v := range want {
		if stub.form.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, stub.form.Get(k), v)
		}
	}
	if len(stub.form) != len(want) {
		t.Errorf("stripe was sent %d fields, want %d: %v", len(stub.form), len(want), stub.form)
	}
	if at.Add(sessionLife).Unix() != 1791264660 {
		t.Fatalf("the test's own arithmetic is off: %d", at.Add(sessionLife).Unix())
	}
}

func TestStartNamesTheLineWhenThereIsNoDescription(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, opened)
	c := charge
	c.Description = "  "
	if _, err := g.Start(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if got := stub.form.Get("line_items[0][price_data][product_data][name]"); got != "chg_7K3M9QX2" {
		t.Errorf("name = %q", got)
	}
	c.Description = strings.Repeat("é", 400)
	if _, err := g.Start(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if got := []rune(stub.form.Get("line_items[0][price_data][product_data][name]")); len(got) != maxName {
		t.Errorf("name is %d characters, want it cut to %d", len(got), maxName)
	}
}

// TestStartRefusesBeforeAskingStripe: what Stripe would get wrong, or
// would refuse, is refused here and Stripe is not called.
func TestStartRefusesBeforeAskingStripe(t *testing.T) {
	tests := []struct {
		name   string
		change func(c *gateway.Charge)
		says   string
	}{
		{"nothing to charge", func(c *gateway.Charge) { c.Amount = amount(0, "INR") }, "above zero"},
		{"a negative amount", func(c *gateway.Charge) { c.Amount = amount(-100, "INR") }, "above zero"},
		{"nowhere to send the guest back to", func(c *gateway.Charge) { c.ReturnURL = "" }, "public_url"},
		// Stripe counts yen in whole yen and dinars in thousandths: sent as
		// mez counts them they would be a charge of the wrong size.
		{"a currency with no decimals", func(c *gateway.Charge) { c.Amount = amount(500, "JPY") }, "JPY"},
		{"a currency with three decimals", func(c *gateway.Charge) { c.Amount = amount(5000, "KWD") }, "KWD"},
		{"a currency stripe has a rule of its own for", func(c *gateway.Charge) { c.Amount = amount(5000, "HUF") }, "HUF"},
		{"an ariary, two decimals to mez and none to stripe", func(c *gateway.Charge) { c.Amount = amount(5000, "MGA") }, "MGA"},
		{"no currency", func(c *gateway.Charge) { c.Amount = amount(5000, "") }, "does not take"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, stub := newStub(t, http.StatusOK, opened)
			c := charge
			tt.change(&c)
			_, err := g.Start(context.Background(), c)
			if !errors.Is(err, gateway.ErrRefused) || !strings.Contains(err.Error(), tt.says) {
				t.Fatalf("err = %v, want it refused saying %q", err, tt.says)
			}
			if stub.calls != 0 {
				t.Errorf("stripe was asked %d times", stub.calls)
			}
		})
	}
}

func TestStartTakesTwoDecimalCurrencies(t *testing.T) {
	for _, currency := range []string{"INR", "USD", "EUR", "GBP", "AED", "SGD", "usd"} {
		g, stub := newStub(t, http.StatusOK, opened)
		c := charge
		c.Amount = amount(1250, currency)
		if _, err := g.Start(context.Background(), c); err != nil {
			t.Fatalf("%s: %v", currency, err)
		}
		if got := stub.form.Get("line_items[0][price_data][currency]"); got != strings.ToLower(currency) {
			t.Errorf("%s was sent as %q", currency, got)
		}
		if got := stub.form.Get("line_items[0][price_data][unit_amount]"); got != "1250" {
			t.Errorf("%s: amount sent as %q", currency, got)
		}
	}
}

func TestStartWhenStripeAnswersBadly(t *testing.T) {
	tests := []struct {
		name   string
		status int
		answer string
		want   error
		says   string
	}{
		{"the key is wrong", 401, `{"error":{"message":"Invalid API Key provided: sk_test_****seen","type":"invalid_request_error"}}`, gateway.ErrRefused, "secret key"},
		{"stripe refuses the request", 400, `{"error":{"message":"Amount must be at least 50 cents","type":"invalid_request_error"}}`, gateway.ErrRefused, "at least 50 cents"},
		{"a refusal with no words", 402, `{}`, gateway.ErrRefused, "402"},
		{"too many requests", 429, `{"error":{"message":"slow down"}}`, gateway.ErrUnavailable, "429"},
		{"its twin is still running", 409, `{"error":{"message":"in progress"}}`, gateway.ErrUnavailable, "409"},
		{"stripe is down", 503, `<html>bad gateway</html>`, gateway.ErrUnavailable, "503"},
		{"an answer that is not JSON", 200, `<html>`, gateway.ErrUnavailable, "JSON"},
		{"a session with no page", 200, `{"id":"cs_test_1","status":"open","url":""}`, gateway.ErrUnavailable, "no page"},
		{"a page that is not https", 200, `{"id":"cs_test_1","status":"open","url":"http://checkout.stripe.com/x"}`, gateway.ErrUnavailable, "no page"},
		{"a page with no session", 200, `{"url":"https://checkout.stripe.com/x"}`, gateway.ErrUnavailable, "no page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _ := newStub(t, tt.status, tt.answer)
			_, err := g.Start(context.Background(), charge)
			if !errors.Is(err, tt.want) || !strings.Contains(err.Error(), tt.says) {
				t.Fatalf("err = %v, want %v saying %q", err, tt.want, tt.says)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("the key is in the error: %v", err)
			}
		})
	}
}

// TestTheKeyIsNeverInAnError: not when Stripe cannot be reached, and not
// if Stripe were to say it back.
func TestTheKeyIsNeverInAnError(t *testing.T) {
	g, err := New(Options{SecretKey: testKey, BaseURL: "http://127.0.0.1:1", Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Start(context.Background(), charge)
	if !errors.Is(err, gateway.ErrUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Errorf("unreachable: %v", err)
	}
	if _, err = g.Status(context.Background(), "cs_test_1"); !errors.Is(err, gateway.ErrUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Errorf("unreachable: %v", err)
	}

	echo, _ := newStub(t, 400, `{"error":{"message":"bad key `+testKey+` here"}}`)
	_, err = echo.Start(context.Background(), charge)
	if !errors.Is(err, gateway.ErrRefused) || strings.Contains(err.Error(), testKey) || !strings.Contains(err.Error(), "[secret]") {
		t.Errorf("echoed back: %v", err)
	}
}

func TestStatus(t *testing.T) {
	const expanded = `{"id":"pi_3Nabc","object":"payment_intent","latest_charge":{"id":"ch_1","payment_method_details":{"type":"%s"}}}`
	paid := func(kind string) string {
		return `{"id":"cs_test_a1B2","status":"complete","payment_status":"paid","amount_total":16800,"currency":"inr","payment_intent":` +
			strings.Replace(expanded, "%s", kind, 1) + `}`
	}
	tests := []struct {
		name   string
		answer string
		want   gateway.Status
	}{
		{"open", opened, gateway.Status{State: gateway.Pending}},
		{"expired", `{"id":"cs_test_a1B2","status":"expired","payment_status":"unpaid"}`, gateway.Status{State: gateway.Failed}},
		// Some ways of paying confirm later: complete is not yet paid.
		{"complete, and the money not yet there", `{"id":"cs_test_a1B2","status":"complete","payment_status":"unpaid","amount_total":16800,"currency":"inr"}`,
			gateway.Status{State: gateway.Pending}},
		{"paid by card", paid("card"), gateway.Status{State: gateway.Paid, Amount: amount(16800, "INR"), Method: gateway.Card, Reference: "pi_3Nabc"}},
		{"paid by UPI", paid("upi"), gateway.Status{State: gateway.Paid, Amount: amount(16800, "INR"), Method: gateway.UPI, Reference: "pi_3Nabc"}},
		{"paid some other way", paid("us_bank_account"), gateway.Status{State: gateway.Paid, Amount: amount(16800, "INR"), Method: gateway.Other, Reference: "pi_3Nabc"}},
		{"paid, the intent not expanded", `{"id":"cs_test_a1B2","status":"complete","payment_status":"paid","amount_total":16800,"currency":"usd","payment_intent":"pi_3Nabc"}`,
			gateway.Status{State: gateway.Paid, Amount: amount(16800, "USD"), Method: gateway.Other, Reference: "pi_3Nabc"}},
		{"paid, with no intent at all", `{"id":"cs_test_a1B2","status":"complete","payment_status":"paid","amount_total":16800,"currency":"inr","payment_intent":null}`,
			gateway.Status{State: gateway.Paid, Amount: amount(16800, "INR"), Method: gateway.Other, Reference: "cs_test_a1B2"}},
		// A different amount is told as it is: the caller refuses to apply it.
		{"paid a different amount", `{"id":"cs_test_a1B2","status":"complete","payment_status":"paid","amount_total":15000,"currency":"inr","payment_intent":"pi_1"}`,
			gateway.Status{State: gateway.Paid, Amount: amount(15000, "INR"), Method: gateway.Other, Reference: "pi_1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, stub := newStub(t, http.StatusOK, tt.answer)
			got, err := g.Status(context.Background(), "cs_test_a1B2")
			if err != nil || got != tt.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tt.want)
			}
			if stub.method != http.MethodGet || stub.path != "/v1/checkout/sessions/cs_test_a1B2" || stub.auth != "Bearer "+testKey {
				t.Errorf("asked %s %s with %q", stub.method, stub.path, stub.auth)
			}
			if got := stub.query["expand[]"]; len(got) != 1 || got[0] != "payment_intent.latest_charge" {
				t.Errorf("expand = %v", got)
			}
			if stub.idempotency != "" {
				t.Error("a read carries an idempotency key")
			}
		})
	}
}

func TestStatusWhenItCannotSay(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		status int
		answer string
		want   error
	}{
		{"no session named", "", 200, opened, gateway.ErrRefused},
		{"a session stripe does not know", "cs_test_nope", 404, `{"error":{"message":"No such checkout.session: 'cs_test_nope'"}}`, gateway.ErrRefused},
		{"a state mez does not know", "cs_test_1", 200, `{"id":"cs_test_1","status":"paused","payment_status":"unpaid"}`, gateway.ErrUnavailable},
		{"complete with nothing to pay", "cs_test_1", 200, `{"id":"cs_test_1","status":"complete","payment_status":"no_payment_required"}`, gateway.ErrUnavailable},
		{"stripe is down", "cs_test_1", 500, ``, gateway.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _ := newStub(t, tt.status, tt.answer)
			got, err := g.Status(context.Background(), tt.id)
			if !errors.Is(err, tt.want) || got != (gateway.Status{}) {
				t.Fatalf("got %+v, %v; want %v and nothing said", got, err, tt.want)
			}
		})
	}
}

// TestASessionIdCannotReachAnotherPath: an id is one path segment.
func TestASessionIdCannotReachAnotherPath(t *testing.T) {
	g, stub := newStub(t, http.StatusOK, opened)
	if _, err := g.Status(context.Background(), "../../v1/refunds"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stub.path, "/v1/checkout/sessions/") || strings.Contains(stub.path, "/v1/refunds") && !strings.HasPrefix(stub.path, "/v1/checkout/sessions/..") {
		t.Errorf("asked %s", stub.path)
	}
}
