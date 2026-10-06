package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/byjoellobo/mez/internal/gateway"
)

// Razorpay's webhooks (phase 11, Task 4f5, ADR-224). Razorpay calls the
// venue when a payment link is paid, expires or is cancelled, and signs
// what it sends: X-Razorpay-Signature is the hex HMAC-SHA256 of the raw
// body, keyed with the webhook's own secret, which is not the API's.
//
// A notice is believed about one thing: which link to go and ask about.
// What it says of the link's state is not read.

// signatureHeader carries Razorpay's signature of the body.
const signatureHeader = "X-Razorpay-Signature"

// event is a webhook's body, the parts read here.
type event struct {
	Event   string `json:"event"`
	Payload struct {
		PaymentLink struct {
			Entity struct {
				ID string `json:"id"`
			} `json:"entity"`
		} `json:"payment_link"`
	} `json:"payload"`
}

// Notice implements gateway.Notified: it checks a webhook is Razorpay's and
// answers the id of the link it is about, or "" for an event about nothing
// mez asked for.
//
// The signature is checked over the body exactly as it arrived, before any
// of it is read, and compared in constant time.
func (g *Gateway) Notice(header http.Header, body []byte) (string, error) {
	if g.webhookSecret == "" {
		return "", fmt.Errorf("%w: no webhook secret is set, so no notice can be told from a forgery", gateway.ErrRefused)
	}
	given, err := hex.DecodeString(strings.TrimSpace(header.Get(signatureHeader)))
	if err != nil || len(given) == 0 {
		return "", fmt.Errorf("%w: the notice carries no signature", gateway.ErrRefused)
	}
	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	mac.Write(body)
	if !hmac.Equal(given, mac.Sum(nil)) {
		return "", fmt.Errorf("%w: the notice's signature is not razorpay's", gateway.ErrRefused)
	}

	var e event
	if err := json.Unmarshal(body, &e); err != nil {
		return "", fmt.Errorf("%w: a signed notice that is not the JSON expected", gateway.ErrRefused)
	}
	if !strings.HasPrefix(e.Event, "payment_link.") {
		return "", nil
	}
	return e.Payload.PaymentLink.Entity.ID, nil
}
