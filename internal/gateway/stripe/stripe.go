// Package stripe is the Stripe adapter (phase 11, Task 4f6a, ADR-230):
// gateway.Gateway over Stripe's Checkout Sessions API.
//
// A charge is a Checkout Session: Stripe's own page, for one amount, paid
// once. Start makes one and Status reads it back. The secret key goes to
// Stripe as a bearer token and nowhere else: not into an error, a log or
// anything this package returns.
//
// The API as used here, from Stripe's documentation:
//
//	POST /v1/checkout/sessions        form-encoded: mode=payment, one line
//	                                  item priced inline, success_url,
//	                                  cancel_url, client_reference_id,
//	                                  expires_at
//	GET  /v1/checkout/sessions/{id}   status: open, complete, expired;
//	                                  payment_status: paid, unpaid,
//	                                  no_payment_required; amount_total;
//	                                  currency; payment_intent
//
// Stripe counts money in a currency's smallest unit, as mez does, for the
// currencies with two decimal places. For the others its rules have
// exceptions of their own, and a mistake there is a charge a hundred times
// too large or too small, so this adapter takes none of them.
package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
)

// Name is how this provider is named in configuration and on a charge.
const Name = "stripe"

// DefaultBaseURL is Stripe's API.
const DefaultBaseURL = "https://api.stripe.com"

const (
	// sessionLife is how long a guest has to pay. Stripe takes no less than
	// thirty minutes; the minute over keeps a slow clock from asking for
	// less.
	sessionLife = 31 * time.Minute

	// timeout bounds one call. A guest is waiting on Start.
	timeout = 15 * time.Second

	// maxBody bounds what is read of an answer.
	maxBody = 1 << 20

	// maxName is what is sent of a description as the line's name.
	maxName = 250
)

// notTaken are the currencies Stripe does not count as mez does, or counts
// by a rule of its own: its zero-decimal and three-decimal currencies, and
// the ones it calls special cases. Asked to charge one, Start refuses.
var notTaken = map[string]bool{
	"BIF": true, "CLP": true, "DJF": true, "GNF": true, "JPY": true, "KMF": true, "KRW": true, "MGA": true,
	"PYG": true, "RWF": true, "UGX": true, "VND": true, "VUV": true, "XAF": true, "XOF": true, "XPF": true,
	"BHD": true, "JOD": true, "KWD": true, "OMR": true, "TND": true,
	"ISK": true, "HUF": true, "TWD": true,
}

// Options configure the adapter. SecretKey is required.
type Options struct {
	// SecretKey is the account's secret key, sk_live_ or sk_test_, or a
	// restricted key that may write Checkout Sessions.
	SecretKey string
	// WebhookSecret is the signing secret of the webhook endpoint, whsec_,
	// from Stripe's dashboard. It is not the API's key. Empty, no webhook
	// is believed.
	WebhookSecret string
	// BaseURL is Stripe's API, or a stand-in for a test. Empty is
	// DefaultBaseURL.
	BaseURL string
	// Client makes the calls. Nil is a client with a timeout.
	Client *http.Client
	// Now is the clock a session's expiry is counted from, and a webhook's
	// age. Nil is time.Now.
	Now func() time.Time
}

// Gateway is Stripe.
type Gateway struct {
	secretKey     string
	webhookSecret string
	base          string
	client        *http.Client
	now           func() time.Time
}

// New returns the adapter. It calls nobody: a wrong key is found out by
// the first charge.
func New(o Options) (*Gateway, error) {
	if strings.TrimSpace(o.SecretKey) == "" {
		return nil, errors.New("stripe: a secret key is needed")
	}
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}
	u, err := url.Parse(o.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("stripe: %q is not an address for its API", o.BaseURL)
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: timeout}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Gateway{
		secretKey: strings.TrimSpace(o.SecretKey), webhookSecret: strings.TrimSpace(o.WebhookSecret),
		base: strings.TrimRight(o.BaseURL, "/"), client: o.Client, now: o.Now,
	}, nil
}

// Name implements gateway.Gateway.
func (g *Gateway) Name() string { return Name }

// session is a Checkout Session as Stripe sends one, the parts read here.
type session struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
	AmountTotal   int64  `json:"amount_total"`
	Currency      string `json:"currency"`
	// PaymentIntent is the intent's id, or the intent itself when it was
	// asked for expanded.
	PaymentIntent json.RawMessage `json:"payment_intent"`
}

// intent is a PaymentIntent expanded on a session, with its latest charge
// expanded in turn: where Stripe says how the guest paid.
type intent struct {
	ID           string `json:"id"`
	LatestCharge struct {
		PaymentMethodDetails struct {
			Type string `json:"type"`
		} `json:"payment_method_details"`
	} `json:"latest_charge"`
}

// Start implements gateway.Gateway: one Checkout Session, for the whole
// amount, good for half an hour.
//
// Stripe will not make a session with nowhere to send the guest back to,
// so a charge with no return address is refused here, before Stripe is
// asked. The charge's own id goes as the idempotency key: the same charge
// asked for twice is one session.
func (g *Gateway) Start(ctx context.Context, c gateway.Charge) (gateway.Checkout, error) {
	if c.Amount.Minor <= 0 {
		return gateway.Checkout{}, fmt.Errorf("%w: a charge needs an amount above zero", gateway.ErrRefused)
	}
	currency := strings.ToUpper(strings.TrimSpace(c.Amount.Currency))
	if len(currency) != 3 || notTaken[currency] || money.Exponent(currency) != 2 {
		return gateway.Checkout{}, fmt.Errorf("%w: mez's stripe adapter does not take %s", gateway.ErrRefused, currency)
	}
	if c.ReturnURL == "" {
		return gateway.Checkout{}, fmt.Errorf("%w: stripe needs an address to send a guest back to: set public_url", gateway.ErrRefused)
	}
	name := clip(c.Description, maxName)
	if name == "" {
		name = c.Reference
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", c.ReturnURL)
	form.Set("cancel_url", c.ReturnURL)
	form.Set("client_reference_id", c.Reference)
	form.Set("metadata[mez_charge]", c.Reference)
	form.Set("expires_at", strconv.FormatInt(g.now().Add(sessionLife).Unix(), 10))
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", strings.ToLower(currency))
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(c.Amount.Minor, 10))
	form.Set("line_items[0][price_data][product_data][name]", name)

	var made session
	if err := g.call(ctx, http.MethodPost, "/v1/checkout/sessions", form, c.Reference, &made); err != nil {
		return gateway.Checkout{}, err
	}
	if made.ID == "" || !strings.HasPrefix(made.URL, "https://") {
		return gateway.Checkout{}, fmt.Errorf("%w: stripe answered with no page to pay at", gateway.ErrUnavailable)
	}
	return gateway.Checkout{ID: made.ID, URL: made.URL}, nil
}

// Status implements gateway.Gateway.
//
// A session Stripe calls complete and not yet paid is pending: some ways
// of paying confirm later, and Stripe says paid when the money is there.
func (g *Gateway) Status(ctx context.Context, id string) (gateway.Status, error) {
	if id == "" {
		return gateway.Status{}, fmt.Errorf("%w: no session was named", gateway.ErrRefused)
	}
	query := url.Values{"expand[]": {"payment_intent.latest_charge"}}
	var s session
	if err := g.call(ctx, http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(id)+"?"+query.Encode(), nil, "", &s); err != nil {
		return gateway.Status{}, err
	}
	switch {
	case s.Status == "open":
		return gateway.Status{State: gateway.Pending}, nil
	case s.Status == "expired":
		return gateway.Status{State: gateway.Failed}, nil
	case s.Status == "complete" && s.PaymentStatus == "unpaid":
		return gateway.Status{State: gateway.Pending}, nil
	case s.Status == "complete" && s.PaymentStatus == "paid":
		reference, method := paidBy(s.PaymentIntent)
		if reference == "" {
			reference = s.ID
		}
		return gateway.Status{
			State:  gateway.Paid,
			Amount: money.Amount{Minor: s.AmountTotal, Currency: strings.ToUpper(s.Currency)},
			Method: method, Reference: reference,
		}, nil
	default:
		return gateway.Status{}, fmt.Errorf("%w: stripe calls session %s %q and %q, which mez does not know", gateway.ErrUnavailable, id, s.Status, s.PaymentStatus)
	}
}

// paidBy reads a session's payment intent: its id, the one a refund names,
// and how the guest paid. The intent comes as an object when expanded and
// as its bare id when not; either is read.
func paidBy(raw json.RawMessage) (string, gateway.Method) {
	var id string
	if json.Unmarshal(raw, &id) == nil {
		return id, gateway.Other
	}
	var pi intent
	if json.Unmarshal(raw, &pi) != nil {
		return "", gateway.Other
	}
	return pi.ID, methodOf(pi.LatestCharge.PaymentMethodDetails.Type)
}

// methodOf is Stripe's kind of payment method as the bill knows one. A
// wallet, a bank debit and everything else are "other" on a bill.
func methodOf(kind string) gateway.Method {
	switch kind {
	case "card", "card_present":
		return gateway.Card
	case "upi":
		return gateway.UPI
	default:
		return gateway.Other
	}
}

// apiError is how Stripe says no.
type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// call makes one request. A failure to reach Stripe, or a failure of its
// own, is gateway.ErrUnavailable; Stripe refusing the request is
// gateway.ErrRefused, with its words and never the key.
func (g *Gateway) call(ctx context.Context, method, path string, form url.Values, idempotencyKey string, into any) error {
	var reader io.Reader
	if form != nil {
		reader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+path, reader)
	if err != nil {
		return fmt.Errorf("stripe: building the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.secretKey)
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", gateway.ErrUnavailable, g.redact(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("%w: reading stripe's answer: %v", gateway.ErrUnavailable, err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if err := json.Unmarshal(raw, into); err != nil {
			return fmt.Errorf("%w: stripe's answer was not the JSON expected", gateway.ErrUnavailable)
		}
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: stripe does not accept the venue's secret key", gateway.ErrRefused)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusConflict || resp.StatusCode >= 500:
		// 409 is Stripe's for a request racing its own twin under one
		// idempotency key: asking again is right.
		return fmt.Errorf("%w: stripe answered %d", gateway.ErrUnavailable, resp.StatusCode)
	default:
		var said apiError
		_ = json.Unmarshal(raw, &said)
		words := strings.TrimSpace(said.Error.Message)
		if words == "" {
			words = fmt.Sprintf("it answered %d", resp.StatusCode)
		}
		return fmt.Errorf("%w: stripe: %s", gateway.ErrRefused, clip(g.redact(words), 300))
	}
}

// redact takes the secrets out of words, should a transport or Stripe ever
// put one there.
func (g *Gateway) redact(words string) string {
	words = strings.ReplaceAll(words, g.secretKey, "[secret]")
	if g.webhookSecret != "" {
		words = strings.ReplaceAll(words, g.webhookSecret, "[secret]")
	}
	return words
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
