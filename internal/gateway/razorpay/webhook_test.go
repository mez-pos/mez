package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/gateway"
)

const hookSecret = "whsec-that-must-never-be-seen"

// paidEvent is a payment_link.paid webhook in Razorpay's documented shape,
// cut to what matters, with fields this adapter never reads.
const paidEvent = `{"entity":"event","account_id":"acc_BFQ7uQEaa7j2z7","event":"payment_link.paid",
"contains":["payment_link","order","payment"],
"payload":{"payment_link":{"entity":{"id":"plink_QflcnnZqCekuvL","reference_id":"chg_7K3M9QX2","status":"paid","amount":16800,"amount_paid":16800}},
"payment":{"entity":{"id":"pay_Qfldmt5StKZFCB","method":"upi","amount":16800}}},
"created_at":1748500000}`

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func hooked(t *testing.T, secret string) *Gateway {
	t.Helper()
	g, err := New(Options{KeyID: testKeyID, KeySecret: testSecret, WebhookSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func headerWith(signature string) http.Header {
	h := http.Header{}
	if signature != "" {
		h.Set("X-Razorpay-Signature", signature)
	}
	return h
}

func TestNotice(t *testing.T) {
	g := hooked(t, hookSecret)
	id, err := g.Notice(headerWith(sign(hookSecret, paidEvent)), []byte(paidEvent))
	if err != nil || id != "plink_QflcnnZqCekuvL" {
		t.Fatalf("id %q, %v", id, err)
	}
	// Upper case hex, and spaces around it, are the same signature.
	if _, err := g.Notice(headerWith("  "+strings.ToUpper(sign(hookSecret, paidEvent))+" "), []byte(paidEvent)); err != nil {
		t.Errorf("an upper case signature: %v", err)
	}
}

func TestNoticeForOtherEvents(t *testing.T) {
	g := hooked(t, hookSecret)
	tests := []struct {
		name string
		body string
		want string
	}{
		{"a link expired", `{"event":"payment_link.expired","payload":{"payment_link":{"entity":{"id":"plink_1"}}}}`, "plink_1"},
		{"a link cancelled", `{"event":"payment_link.cancelled","payload":{"payment_link":{"entity":{"id":"plink_2"}}}}`, "plink_2"},
		{"a payment, of something mez did not ask for", `{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_1"}}}}`, ""},
		{"a link event that names no link", `{"event":"payment_link.paid","payload":{}}`, ""},
		{"an event with no name", `{"payload":{"payment_link":{"entity":{"id":"plink_3"}}}}`, ""},
	}
	for _, tt := range tests {
		id, err := g.Notice(headerWith(sign(hookSecret, tt.body)), []byte(tt.body))
		if err != nil || id != tt.want {
			t.Errorf("%s: id %q, %v; want %q", tt.name, id, err, tt.want)
		}
	}
}

// TestNoticeRefusesWhatIsNotRazorpays: a forgery, a body changed by one
// byte, a signature made with the API's secret, and none at all.
func TestNoticeRefusesWhatIsNotRazorpays(t *testing.T) {
	g := hooked(t, hookSecret)
	good := sign(hookSecret, paidEvent)
	tests := []struct {
		name      string
		signature string
		body      string
	}{
		{"no signature", "", paidEvent},
		{"a signature that is not hex", "not-hex-at-all", paidEvent},
		{"a made-up signature", strings.Repeat("ab", 32), paidEvent},
		{"the right signature on a body changed by a byte", good, strings.Replace(paidEvent, "16800", "16801", 1)},
		{"a body with a space added", good, paidEvent + " "},
		{"signed with the API's secret, not the webhook's", sign(testSecret, paidEvent), paidEvent},
		{"a signature cut short", good[:32], paidEvent},
		{"signed, and not JSON", sign(hookSecret, "hello"), "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := g.Notice(headerWith(tt.signature), []byte(tt.body))
			if !errors.Is(err, gateway.ErrRefused) || id != "" {
				t.Fatalf("id %q, err %v; want it refused", id, err)
			}
			if strings.Contains(err.Error(), hookSecret) || strings.Contains(err.Error(), testSecret) {
				t.Errorf("a secret is in the error: %v", err)
			}
		})
	}
}

// TestNoticeWithNoSecretSetBelievesNothing: without the webhook's secret
// there is no telling a notice from a forgery, so none is taken.
func TestNoticeWithNoSecretSetBelievesNothing(t *testing.T) {
	g := hooked(t, "")
	for _, signature := range []string{"", sign("", paidEvent), sign(testSecret, paidEvent)} {
		if _, err := g.Notice(headerWith(signature), []byte(paidEvent)); !errors.Is(err, gateway.ErrRefused) {
			t.Errorf("signature %q: %v, want it refused", signature, err)
		}
	}
}
