// Package razorpay is the Razorpay adapter (phase 11, Task 4f2, ADR-221):
// gateway.Gateway over Razorpay's Payment Links API.
//
// A charge is a payment link: Razorpay's own page, at a short address, for
// one amount, paid once. Start makes one and Status reads it back. The key
// id and secret go to Razorpay as HTTP basic auth and nowhere else: not
// into an error, a log or anything this package returns.
//
// The API as used here, from Razorpay's documentation:
//
//	POST /v1/payment_links/        amount in paise, currency, reference_id,
//	                               description, callback_url, expire_by
//	GET  /v1/payment_links/{id}    status: created, partially_paid, paid,
//	                               expired, cancelled; amount_paid; and
//	                               payments[] of captured payments, each
//	                               with payment_id and method
package razorpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
)

// Name is how this provider is named in configuration and on a charge.
const Name = "razorpay"

// DefaultBaseURL is Razorpay's API.
const DefaultBaseURL = "https://api.razorpay.com"

const (
	// linkLife is how long a guest has to pay a link. Razorpay asks for at
	// least fifteen minutes; a guest at a table needs no more than this.
	linkLife = 30 * time.Minute

	// timeout bounds one call. A guest is waiting on Start.
	timeout = 15 * time.Second

	// maxBody bounds what is read of an answer.
	maxBody = 1 << 20

	// maxDescription is Razorpay's limit on a link's description.
	maxDescription = 2048
)

// Options configure the adapter. KeyID and KeySecret are required.
type Options struct {
	KeyID     string
	KeySecret string
	// WebhookSecret is the secret Razorpay signs its webhooks with, set
	// where the webhook is, in Razorpay's dashboard. It is not the API's
	// secret. Empty, no webhook is believed.
	WebhookSecret string
	// BaseURL is Razorpay's API, or a stand-in for a test. Empty is
	// DefaultBaseURL.
	BaseURL string
	// Client makes the calls. Nil is a client with a timeout.
	Client *http.Client
	// Now is the clock a link's expiry is counted from. Nil is time.Now.
	Now func() time.Time
}

// Gateway is Razorpay.
type Gateway struct {
	keyID, keySecret string
	webhookSecret    string
	base             string
	client           *http.Client
	now              func() time.Time
}

// New returns the adapter. It calls nobody: wrong keys are found out by the
// first charge.
func New(o Options) (*Gateway, error) {
	if strings.TrimSpace(o.KeyID) == "" || strings.TrimSpace(o.KeySecret) == "" {
		return nil, errors.New("razorpay: a key id and a key secret are both needed")
	}
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}
	u, err := url.Parse(o.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("razorpay: %q is not an address for its API", o.BaseURL)
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: timeout}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Gateway{
		keyID: strings.TrimSpace(o.KeyID), keySecret: strings.TrimSpace(o.KeySecret),
		webhookSecret: strings.TrimSpace(o.WebhookSecret),
		base:          strings.TrimRight(o.BaseURL, "/"), client: o.Client, now: o.Now,
	}, nil
}

// Name implements gateway.Gateway.
func (g *Gateway) Name() string { return Name }

// link is a payment link as Razorpay sends one, the parts read here.
type link struct {
	ID         string    `json:"id"`
	Amount     int64     `json:"amount"`
	AmountPaid int64     `json:"amount_paid"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	ShortURL   string    `json:"short_url"`
	Payments   []payment `json:"payments"`
}

type payment struct {
	PaymentID string `json:"payment_id"`
	Amount    int64  `json:"amount"`
	Method    string `json:"method"`
	Status    string `json:"status"`
}

// newLink is what Start asks for.
type newLink struct {
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	AcceptPartial  bool   `json:"accept_partial"`
	ReferenceID    string `json:"reference_id"`
	Description    string `json:"description,omitempty"`
	ExpireBy       int64  `json:"expire_by"`
	CallbackURL    string `json:"callback_url,omitempty"`
	CallbackMethod string `json:"callback_method,omitempty"`
	// Razorpay would otherwise text and email a guest mez has no number or
	// address for, and remind them.
	Notify         notify `json:"notify"`
	ReminderEnable bool   `json:"reminder_enable"`
}

type notify struct {
	SMS   bool `json:"sms"`
	Email bool `json:"email"`
}

// Start implements gateway.Gateway: one payment link, for the whole amount
// and nothing less, good for half an hour.
func (g *Gateway) Start(ctx context.Context, c gateway.Charge) (gateway.Checkout, error) {
	if c.Amount.Minor <= 0 {
		return gateway.Checkout{}, fmt.Errorf("%w: a charge needs an amount above zero", gateway.ErrRefused)
	}
	body := newLink{
		Amount: c.Amount.Minor, Currency: c.Amount.Currency, ReferenceID: c.Reference,
		Description: clip(c.Description, maxDescription), ExpireBy: g.now().Add(linkLife).Unix(),
	}
	if c.ReturnURL != "" {
		body.CallbackURL, body.CallbackMethod = c.ReturnURL, "get"
	}
	var made link
	if err := g.call(ctx, http.MethodPost, "/v1/payment_links/", body, &made); err != nil {
		return gateway.Checkout{}, err
	}
	if made.ID == "" || !strings.HasPrefix(made.ShortURL, "https://") {
		return gateway.Checkout{}, fmt.Errorf("%w: razorpay answered with no link to pay", gateway.ErrUnavailable)
	}
	return gateway.Checkout{ID: made.ID, URL: made.ShortURL}, nil
}

// Status implements gateway.Gateway.
//
// A link Razorpay calls partially_paid is answered as paid, for the part
// that was: partial payment is turned off on every link made here, so it
// should not happen, and if it does there is money at Razorpay that the
// caller has to hear about and not wait on.
func (g *Gateway) Status(ctx context.Context, id string) (gateway.Status, error) {
	if id == "" {
		return gateway.Status{}, fmt.Errorf("%w: no link was named", gateway.ErrRefused)
	}
	var l link
	if err := g.call(ctx, http.MethodGet, "/v1/payment_links/"+url.PathEscape(id), nil, &l); err != nil {
		return gateway.Status{}, err
	}
	switch l.Status {
	case "created":
		return gateway.Status{State: gateway.Pending}, nil
	case "expired", "cancelled":
		return gateway.Status{State: gateway.Failed}, nil
	case "paid", "partially_paid":
		paid := captured(l.Payments)
		st := gateway.Status{
			State: gateway.Paid, Amount: money.Amount{Minor: l.AmountPaid, Currency: l.Currency},
			Method: methodOf(paid.Method), Reference: paid.PaymentID,
		}
		return st, nil
	default:
		return gateway.Status{}, fmt.Errorf("%w: razorpay calls link %s %q, which mez does not know", gateway.ErrUnavailable, id, l.Status)
	}
}

// captured is the payment that paid a link: the last one captured. A link
// is paid once here, so it is the only one. A payment that says no status
// is not taken for captured: Razorpay always says, and what it says is the
// only ground for naming a method and a reference on the bill.
func captured(payments []payment) payment {
	for i := len(payments) - 1; i >= 0; i-- {
		if payments[i].Status == "captured" {
			return payments[i]
		}
	}
	return payment{}
}

// methodOf is Razorpay's method as the bill knows one. Net banking, a
// wallet, EMI and a bank transfer are all "other" on a bill.
func methodOf(m string) gateway.Method {
	switch m {
	case "upi":
		return gateway.UPI
	case "card":
		return gateway.Card
	default:
		return gateway.Other
	}
}

// apiError is how Razorpay says no.
type apiError struct {
	Error struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"error"`
}

// call makes one request. A failure to reach Razorpay, or a failure of
// its own, is gateway.ErrUnavailable; Razorpay refusing the request is
// gateway.ErrRefused, with its words and never the keys.
func (g *Gateway) call(ctx context.Context, method, path string, body, into any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("razorpay: encoding the request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+path, reader)
	if err != nil {
		return fmt.Errorf("razorpay: building the request: %w", err)
	}
	req.SetBasicAuth(g.keyID, g.keySecret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		// The error of a failed request names its address, which holds no
		// key: basic auth travels in a header.
		return fmt.Errorf("%w: %v", gateway.ErrUnavailable, redact(err, g.keySecret))
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("%w: reading razorpay's answer: %v", gateway.ErrUnavailable, err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if err := json.Unmarshal(raw, into); err != nil {
			return fmt.Errorf("%w: razorpay's answer was not the JSON expected", gateway.ErrUnavailable)
		}
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: razorpay does not accept the venue's key id and secret", gateway.ErrRefused)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: razorpay answered %d", gateway.ErrUnavailable, resp.StatusCode)
	default:
		var said apiError
		_ = json.Unmarshal(raw, &said)
		words := strings.TrimSpace(said.Error.Description)
		if words == "" {
			words = fmt.Sprintf("it answered %d", resp.StatusCode)
		}
		return fmt.Errorf("%w: razorpay: %s", gateway.ErrRefused, clip(words, 300))
	}
}

// redact takes a secret out of an error's words, should a transport ever
// put it there.
func redact(err error, secret string) string {
	return strings.ReplaceAll(err.Error(), secret, "[secret]")
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
