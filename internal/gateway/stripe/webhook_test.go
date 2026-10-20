package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/gateway"
)

const hookSecret = "whsec_the-endpoints-own-secret"

const completed = `{"id":"evt_1","object":"event","type":"checkout.session.completed","data":{"object":{"id":"cs_test_a1B2","object":"checkout.session","amount_total":16800,"payment_status":"paid"}}}`

func hooked(t *testing.T, secret string) *Gateway {
	t.Helper()
	g, err := New(Options{SecretKey: testKey, WebhookSecret: secret, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// sign is Stripe's signature of a body at a time, as its documentation
// has it: the hex HMAC-SHA256 of "<time>.<body>".
func sign(secret string, when time.Time, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", when.Unix(), body)
	return hex.EncodeToString(mac.Sum(nil))
}

func headerOf(value string) http.Header {
	h := http.Header{}
	if value != "" {
		h.Set("Stripe-Signature", value)
	}
	return h
}

func signed(secret string, when time.Time, body string) http.Header {
	return headerOf(fmt.Sprintf("t=%d,v1=%s", when.Unix(), sign(secret, when, body)))
}

// TestNoticeAgainstAKnownSignature: the scheme, checked against a
// signature worked out apart from Go, by Python's hmac module:
//
//	hmac.new(b"whsec_test", b"1791262800.{}", hashlib.sha256).hexdigest()
//
// so a mistake shared by this package and its tests' own signer shows.
func TestNoticeAgainstAKnownSignature(t *testing.T) {
	const want = "954b9123c32c1d0016919ca19a5a4c99c9078cb2f24beb402304324d08e888c0"
	if at.Unix() != 1791262800 {
		t.Fatalf("the clock is %d", at.Unix())
	}
	if got := sign("whsec_test", at, "{}"); got != want {
		t.Fatalf("the tests' signer makes %s", got)
	}
	g := hooked(t, "whsec_test")
	if _, err := g.Notice(headerOf("t=1791262800,v1="+want), []byte("{}")); err != nil {
		t.Errorf("a notice signed as documented was refused: %v", err)
	}
}

func TestNoticeNamesTheSession(t *testing.T) {
	g := hooked(t, hookSecret)
	id, err := g.Notice(signed(hookSecret, at, completed), []byte(completed))
	if err != nil || id != "cs_test_a1B2" {
		t.Fatalf("id %q, %v", id, err)
	}
	tests := []struct {
		name string
		body string
		want string
	}{
		{"a session expired", `{"type":"checkout.session.expired","data":{"object":{"id":"cs_test_2"}}}`, "cs_test_2"},
		{"a slow payment arrived", `{"type":"checkout.session.async_payment_succeeded","data":{"object":{"id":"cs_test_3"}}}`, "cs_test_3"},
		{"a slow payment failed", `{"type":"checkout.session.async_payment_failed","data":{"object":{"id":"cs_test_4"}}}`, "cs_test_4"},
		{"a payment intent, which mez did not ask about", `{"type":"payment_intent.succeeded","data":{"object":{"id":"pi_1"}}}`, ""},
		{"a session event that names no session", `{"type":"checkout.session.completed","data":{}}`, ""},
		{"an event with no type", `{"data":{"object":{"id":"cs_test_5"}}}`, ""},
	}
	for _, tt := range tests {
		id, err := g.Notice(signed(hookSecret, at, tt.body), []byte(tt.body))
		if err != nil || id != tt.want {
			t.Errorf("%s: id %q, %v; want %q", tt.name, id, err, tt.want)
		}
	}
}

// TestNoticeWhileASecretIsRolled: Stripe sends a signature for each secret
// then, and other schemes beside them. One good v1 is enough.
func TestNoticeWhileASecretIsRolled(t *testing.T) {
	g := hooked(t, hookSecret)
	good := sign(hookSecret, at, completed)
	other := sign("whsec_the-old-one", at, completed)
	header := fmt.Sprintf("t=%d, v1=%s, v1=%s, v0=%s", at.Unix(), other, good, strings.Repeat("0", 64))
	if id, err := g.Notice(headerOf(header), []byte(completed)); err != nil || id != "cs_test_a1B2" {
		t.Errorf("id %q, %v", id, err)
	}
}

// TestNoticeRefusesWhatIsNotStripes: a forgery, a body changed by a byte,
// the wrong secret, an old notice sent again, and none at all.
func TestNoticeRefusesWhatIsNotStripes(t *testing.T) {
	g := hooked(t, hookSecret)
	good := sign(hookSecret, at, completed)
	stamp := fmt.Sprint(at.Unix())
	old := at.Add(-tolerance - time.Second)
	ahead := at.Add(tolerance + time.Second)
	tests := []struct {
		name   string
		header string
		body   string
	}{
		{"no signature", "", completed},
		{"a time and no signature", "t=" + stamp, completed},
		{"a signature and no time", "v1=" + good, completed},
		{"a signature that is not hex", "t=" + stamp + ",v1=not-hex", completed},
		{"a made-up signature", "t=" + stamp + ",v1=" + strings.Repeat("ab", 32), completed},
		{"only the scheme mez does not read", "t=" + stamp + ",v0=" + good, completed},
		{"the right signature on a body changed by a byte", "t=" + stamp + ",v1=" + good, strings.Replace(completed, "16800", "16801", 1)},
		{"the right signature under another time", fmt.Sprintf("t=%d,v1=%s", at.Unix()+1, good), completed},
		{"a time that is not a number", "t=now,v1=" + good, completed},
		{"signed with the API's key, not the endpoint's", fmt.Sprintf("t=%s,v1=%s", stamp, sign(testKey, at, completed)), completed},
		{"a signature cut short", "t=" + stamp + ",v1=" + good[:32], completed},
		{"rightly signed, and too old", fmt.Sprintf("t=%d,v1=%s", old.Unix(), sign(hookSecret, old, completed)), completed},
		{"rightly signed, from a clock far ahead", fmt.Sprintf("t=%d,v1=%s", ahead.Unix(), sign(hookSecret, ahead, completed)), completed},
		{"signed, and not JSON", fmt.Sprintf("t=%s,v1=%s", stamp, sign(hookSecret, at, "hello")), "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := g.Notice(headerOf(tt.header), []byte(tt.body))
			if !errors.Is(err, gateway.ErrRefused) || id != "" {
				t.Fatalf("id %q, err %v; want it refused", id, err)
			}
			if strings.Contains(err.Error(), hookSecret) || strings.Contains(err.Error(), testKey) {
				t.Errorf("a secret is in the error: %v", err)
			}
		})
	}
}

// TestNoticeAtTheEdgeOfItsAge: five minutes old is taken, a second more is
// not.
func TestNoticeAtTheEdgeOfItsAge(t *testing.T) {
	g := hooked(t, hookSecret)
	edge := at.Add(-tolerance)
	if _, err := g.Notice(signed(hookSecret, edge, completed), []byte(completed)); err != nil {
		t.Errorf("a notice exactly five minutes old: %v", err)
	}
}

// TestNoticeWithNoSecretSet: nothing can be told from a forgery, so
// nothing is believed, however it is signed.
func TestNoticeWithNoSecretSet(t *testing.T) {
	g := hooked(t, "")
	_, err := g.Notice(signed("", at, completed), []byte(completed))
	if !errors.Is(err, gateway.ErrRefused) {
		t.Errorf("err = %v", err)
	}
}

// The adapter is both of the shapes mez uses a gateway through.
var (
	_ gateway.Gateway  = (*Gateway)(nil)
	_ gateway.Notified = (*Gateway)(nil)
)
