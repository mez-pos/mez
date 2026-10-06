package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/gateway"
)

// Stripe's webhooks (phase 11, Task 4f6a, ADR-230; the hook is ADR-224's).
// Stripe calls the venue when a Checkout Session is completed or expires,
// and signs what it sends. The Stripe-Signature header carries a time and
// one or more signatures:
//
//	Stripe-Signature: t=1492774577,v1=5257a869...,v1=...
//
// Each v1 is the hex HMAC-SHA256 of the time, a dot and the raw body,
// keyed with the endpoint's own signing secret, which is not the API's
// key. There is more than one while a secret is being rolled.
//
// A notice is believed about one thing: which session to go and ask about.
// What it says of the session's state is not read.

// signatureHeader carries Stripe's time and signatures.
const signatureHeader = "Stripe-Signature"

// tolerance is how old, or how far ahead, a notice's time may be. It is
// Stripe's own recommendation, and bounds how long a captured notice can
// be sent again.
const tolerance = 5 * time.Minute

// event is a webhook's body, the parts read here.
type event struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
	} `json:"data"`
}

// Notice implements gateway.Notified: it checks a webhook is Stripe's and
// answers the id of the session it is about, or "" for an event about
// nothing mez asked for.
//
// The signature is checked over the body exactly as it arrived, before any
// of it is read, and compared in constant time.
func (g *Gateway) Notice(header http.Header, body []byte) (string, error) {
	if g.webhookSecret == "" {
		return "", fmt.Errorf("%w: no webhook secret is set, so no notice can be told from a forgery", gateway.ErrRefused)
	}
	stamp, given := parseSignature(header.Get(signatureHeader))
	if stamp == "" || len(given) == 0 {
		return "", fmt.Errorf("%w: the notice carries no signature", gateway.ErrRefused)
	}
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: the notice's time is not one", gateway.ErrRefused)
	}

	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	mac.Write([]byte(stamp))
	mac.Write([]byte{'.'})
	mac.Write(body)
	want := mac.Sum(nil)
	signed := false
	for _, sig := range given {
		// Every one is compared, so how long this takes says nothing of
		// which matched.
		if hmac.Equal(sig, want) {
			signed = true
		}
	}
	if !signed {
		return "", fmt.Errorf("%w: the notice's signature is not stripe's", gateway.ErrRefused)
	}
	// The time is under the signature, so it is Stripe's own once that
	// holds, and only then worth reading.
	if age := g.now().Sub(time.Unix(seconds, 0)); age > tolerance || age < -tolerance {
		return "", fmt.Errorf("%w: the notice is too old, or from a clock far from this one", gateway.ErrRefused)
	}

	var e event
	if err := json.Unmarshal(body, &e); err != nil {
		return "", fmt.Errorf("%w: a signed notice that is not the JSON expected", gateway.ErrRefused)
	}
	if !strings.HasPrefix(e.Type, "checkout.session.") {
		return "", nil
	}
	return e.Data.Object.ID, nil
}

// parseSignature reads a Stripe-Signature header: its time, as written,
// and every v1 signature that is hex. Other schemes are passed over.
func parseSignature(header string) (string, [][]byte) {
	var stamp string
	var sigs [][]byte
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			stamp = value
		case "v1":
			if raw, err := hex.DecodeString(value); err == nil && len(raw) > 0 {
				sigs = append(sigs, raw)
			}
		}
	}
	return stamp, sigs
}
