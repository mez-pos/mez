package cloud

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

const assertedVenue = "ven_TEST0001"

var assertedAt = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func signingKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func assertion(t *testing.T) Assertion {
	t.Helper()
	a, err := NewAssertion(assertedVenue, assertedAt, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// sealed signs a payload as it is given, right or wrong, so a test can put
// a good signature under something SignAssertion would refuse to sign.
func sealed(priv ed25519.PrivateKey, payload string) string {
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := ed25519.Sign(priv, []byte(assertionContext+body))
	return assertionVersion + "." + body + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestAssertionRoundTrip(t *testing.T) {
	pub, priv := signingKey(t)
	want := assertion(t)
	signed, err := SignAssertion(priv, want)
	if err != nil {
		t.Fatalf("SignAssertion: %v", err)
	}
	got, err := VerifyAssertion(pub, signed, assertedVenue, assertedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}
	if got != want {
		t.Errorf("verified %+v, signed %+v", got, want)
	}
	if keep := got.KeepUntil(); !keep.Equal(assertedAt.Add(AssertionLife + AssertionLeeway)) {
		t.Errorf("KeepUntil = %s", keep)
	}
}

func TestAssertionNoncesDiffer(t *testing.T) {
	if a, b := assertion(t), assertion(t); a.Nonce == b.Nonce {
		t.Errorf("two assertions drew the nonce %q", a.Nonce)
	}
}

func TestVerifyAssertionTime(t *testing.T) {
	pub, priv := signingKey(t)
	signed, err := SignAssertion(priv, assertion(t))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		now  time.Time
		want error
	}{
		{"as it is issued", assertedAt, nil},
		{"a venue's clock a minute behind", assertedAt.Add(-AssertionLeeway), nil},
		{"a venue's clock more than a minute behind", assertedAt.Add(-AssertionLeeway - time.Second), ErrAssertionTime},
		{"in its last second", assertedAt.Add(AssertionLife - time.Second), nil},
		{"inside the grace after it", assertedAt.Add(AssertionLife + AssertionLeeway - time.Second), nil},
		{"at the end of the grace", assertedAt.Add(AssertionLife + AssertionLeeway), ErrAssertionTime},
		{"an hour on", assertedAt.Add(time.Hour), ErrAssertionTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyAssertion(pub, signed, assertedVenue, tt.now)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyAssertionRefuses(t *testing.T) {
	pub, priv := signingKey(t)
	_, other := signingKey(t)
	good, err := SignAssertion(priv, assertion(t))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, ".")
	byOther, err := SignAssertion(other, assertion(t))
	if err != nil {
		t.Fatal(err)
	}
	// A signature this key made, with no context in front of it: what a key
	// reused to sign something else would produce.
	bare := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(parts[1])))
	iat, exp := assertedAt.Unix(), assertedAt.Add(AssertionLife).Unix()
	payload := func(aud, sub string, iat, exp int64, nonce, extra string) string {
		return `{"aud":"` + aud + `","sub":"` + sub + `","iat":` + itoa(iat) + `,"exp":` + itoa(exp) + `,"nonce":"` + nonce + `"` + extra + `}`
	}
	const nonce = "abcdefghijklmnopqrstuvwx"

	tests := []struct {
		name      string
		presented string
		venue     string
	}{
		{"nothing", "", assertedVenue},
		{"not three parts", parts[0] + "." + parts[1], assertedVenue},
		{"another version", "v2." + parts[1] + "." + parts[2], assertedVenue},
		{"another key's", byOther, assertedVenue},
		{"a payload changed after signing", parts[0] + "." + parts[1] + "A." + parts[2], assertedVenue},
		{"a signature that is not base64url", parts[0] + "." + parts[1] + ".!!!", assertedVenue},
		{"a line break in the signature", parts[0] + "." + parts[1] + "." + parts[2][:40] + "\n" + parts[2][40:], assertedVenue},
		{"a line break after it", good + "\n", assertedVenue},
		{"the same signature spelt another way", respelt(good), assertedVenue},
		{"a signature with no context", bare, assertedVenue},
		{"for another venue", good, "ven_OTHER001"},
		{"no venue to be for", good, ""},
		{"longer than an assertion is", good + strings.Repeat("A", maxAssertion), assertedVenue},
		{"signed, for a staff member by id", sealed(priv, payload(assertedVenue, "stf_ABCDEFGH", iat, exp, nonce, "")), assertedVenue},
		{"signed, for nobody", sealed(priv, payload(assertedVenue, "", iat, exp, nonce, "")), assertedVenue},
		{"signed, good for an hour", sealed(priv, payload(assertedVenue, SubjectOwner, iat, iat+3600, nonce, "")), assertedVenue},
		{"signed, expiring before it is issued", sealed(priv, payload(assertedVenue, SubjectOwner, exp, iat, nonce, "")), assertedVenue},
		{"signed, a short nonce", sealed(priv, payload(assertedVenue, SubjectOwner, iat, exp, "short", "")), assertedVenue},
		{"signed, a nonce that is not url-safe", sealed(priv, payload(assertedVenue, SubjectOwner, iat, exp, "abcdefghijklmnop/+==", "")), assertedVenue},
		{"signed, with a claim this build does not know", sealed(priv, payload(assertedVenue, SubjectOwner, iat, exp, nonce, `,"role":"owner"`)), assertedVenue},
		{"signed, and not JSON", sealed(priv, "owner please"), assertedVenue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyAssertion(pub, tt.presented, tt.venue, assertedAt.Add(time.Second))
			if !errors.Is(err, ErrAssertion) {
				t.Errorf("err = %v, want ErrAssertion", err)
			}
		})
	}

	t.Run("a key that is not one", func(t *testing.T) {
		if _, err := VerifyAssertion(pub[:8], good, assertedVenue, assertedAt); !errors.Is(err, ErrAssertion) {
			t.Errorf("err = %v, want ErrAssertion", err)
		}
	})
}

func TestSignAssertionRefuses(t *testing.T) {
	_, priv := signingKey(t)
	tests := []struct {
		name   string
		key    ed25519.PrivateKey
		change func(*Assertion)
	}{
		{"a key that is not one", priv[:10], func(*Assertion) {}},
		{"no venue", priv, func(a *Assertion) { a.Audience = "" }},
		{"someone who is not the owner", priv, func(a *Assertion) { a.Subject = "stf_ABCDEFGH" }},
		{"longer than five minutes", priv, func(a *Assertion) { a.ExpiresAt = a.IssuedAt.Add(AssertionLife + time.Second) }},
		{"no nonce", priv, func(a *Assertion) { a.Nonce = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assertion(t)
			tt.change(&a)
			if signed, err := SignAssertion(tt.key, a); err == nil {
				t.Errorf("signed %q", signed)
			}
		})
	}
}

// respelt changes the last character of an assertion for another that
// decodes, loosely, to the same bytes: the stray bits base64 leaves over.
func respelt(signed string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, signed[len(signed)-1])
	return signed[:len(signed)-1] + string(alphabet[last^1])
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
