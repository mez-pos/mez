package cloud

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// An assertion is mez Cloud vouching, to one venue, that the account's owner
// has signed in to the cloud just now (ADR-312, ADR-323). The venue checks it
// with the public key its pairing holds and lets the owner into the back
// office with no PIN typed.
//
// It reads "v1.<payload>.<signature>", the last two in base64url with no
// padding. It is not a JWT: there is one algorithm and one issuer, so there
// is nothing in it to choose and nothing to choose wrongly.
const (
	// AssertionScheme is the Authorization scheme one is carried under.
	AssertionScheme = "Mez-Cloud"

	// AssertionLife is the longest an assertion may say it is good for, and
	// AssertionLeeway how far either clock may be out. A cafe's computer
	// keeps worse time than a server does.
	AssertionLife   = 5 * time.Minute
	AssertionLeeway = time.Minute

	// SubjectOwner is the only subject there is: the one person the cloud
	// may be at a venue. It is a word and not a staff id. The venue knows
	// who it means, and the cloud never learns the id of anyone there.
	SubjectOwner = "owner"

	assertionVersion = "v1"
	// assertionContext is signed in front of the payload, so that a
	// signature the same key made over anything else is not one of these.
	assertionContext = "mez-cloud-assertion-v1\n"

	// maxAssertion bounds what is read of one before anything is decoded.
	maxAssertion = 1024

	minNonce = 16
	maxNonce = 64
)

// assertionEncoding is base64url with no padding, and no second way to
// write the same bytes.
var assertionEncoding = base64.RawURLEncoding.Strict()

var (
	// ErrAssertion is an assertion that is not accepted: not one at all,
	// signed by another key, for another venue or for someone else. Which
	// is not said to whoever presented it.
	ErrAssertion = errors.New("the assertion is not accepted")

	// ErrAssertionTime is an assertion that is rightly signed and outside
	// its five minutes: held too long, or one of the two clocks is wrong.
	// It is told apart because the remedy is the owner's: this computer's
	// clock.
	ErrAssertionTime = errors.New("the assertion is outside the time it is good for")
)

// Assertion is what the cloud vouches for.
type Assertion struct {
	// Audience is the venue it is for: the venue's public id, as the venue
	// sent it when it was paired (ADR-318).
	Audience string
	// Subject is who is vouched for. SubjectOwner, always.
	Subject string
	// IssuedAt and ExpiresAt are the time it is good for, to the second.
	IssuedAt  time.Time
	ExpiresAt time.Time
	// Nonce is drawn afresh for each one. The venue accepts a nonce once.
	Nonce string
}

// KeepUntil is how long the venue must remember the nonce: until the
// assertion would be refused for its age anyway.
func (a Assertion) KeepUntil() time.Time { return a.ExpiresAt.Add(AssertionLeeway) }

// assertionWire is the payload as it is signed.
type assertionWire struct {
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
	Nonce    string `json:"nonce"`
}

// NewAssertion is an assertion for the owner of the venue with this public
// id, good from now for AssertionLife, with a nonce drawn from entropy.
func NewAssertion(venuePublicID string, now time.Time, entropy io.Reader) (Assertion, error) {
	var raw [18]byte
	if _, err := io.ReadFull(entropy, raw[:]); err != nil {
		return Assertion{}, fmt.Errorf("cloud: drawing a nonce: %w", err)
	}
	now = now.UTC().Truncate(time.Second)
	return Assertion{
		Audience: venuePublicID, Subject: SubjectOwner,
		IssuedAt: now, ExpiresAt: now.Add(AssertionLife),
		Nonce: base64.RawURLEncoding.EncodeToString(raw[:]),
	}, nil
}

// SignAssertion signs an assertion as mez Cloud does. The venue's binary
// never calls it: it is here so that the service that does, which imports
// this module, and the venue that checks, cannot come to disagree.
func SignAssertion(key ed25519.PrivateKey, a Assertion) (string, error) {
	if len(key) != ed25519.PrivateKeySize {
		return "", errors.New("cloud: the signing key is not an Ed25519 private key")
	}
	if err := a.check(); err != nil {
		return "", fmt.Errorf("cloud: %w", err)
	}
	raw, err := json.Marshal(assertionWire{
		Audience: a.Audience, Subject: a.Subject,
		IssuedAt: a.IssuedAt.Unix(), Expires: a.ExpiresAt.Unix(), Nonce: a.Nonce,
	})
	if err != nil {
		return "", fmt.Errorf("cloud: encoding an assertion: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := ed25519.Sign(key, []byte(assertionContext+payload))
	return assertionVersion + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyAssertion checks an assertion as presented: that it was signed by
// key, or by the newest key the links presented with it lead to from key
// (ADR-345); that it is for the venue with this public id and for the
// owner; and that now is inside the time it is good for. The errors are
// ErrAssertion and ErrAssertionTime.
//
// It returns the key that signed it. One that is not key is the cloud's
// next, vouched for by key, and the venue keeps it once the sign-in is
// done.
//
// It does not know whether the assertion has been presented before. The
// caller spends the Nonce, and keeps it until KeepUntil.
func VerifyAssertion(key ed25519.PublicKey, presented, venuePublicID string, now time.Time) (Assertion, ed25519.PublicKey, error) {
	if len(key) != ed25519.PublicKeySize || venuePublicID == "" || len(presented) > maxPresented {
		return Assertion{}, nil, ErrAssertion
	}
	// Strictly: the decoder otherwise passes over a line break and over
	// stray bits in the last character, and one assertion would have many
	// spellings. Nothing here tells them apart but the nonce, and whatever
	// is one day keyed on the text as presented should not have to know.
	if strings.ContainsAny(presented, "\r\n") {
		return Assertion{}, nil, ErrAssertion
	}
	signed, chain, chained := strings.Cut(presented, chainSeparator)
	signer := key
	if chained {
		var err error
		if signer, err = followChain(key, strings.Split(chain, chainSeparator)); err != nil {
			return Assertion{}, nil, err
		}
	}
	a, err := verifySigned(signer, signed, venuePublicID, now)
	if err != nil {
		return Assertion{}, nil, err
	}
	return a, signer, nil
}

// verifySigned checks an assertion alone, with the one key that must have
// signed it.
func verifySigned(key ed25519.PublicKey, presented, venuePublicID string, now time.Time) (Assertion, error) {
	if len(presented) > maxAssertion {
		return Assertion{}, ErrAssertion
	}
	parts := strings.Split(presented, ".")
	if len(parts) != 3 || parts[0] != assertionVersion {
		return Assertion{}, ErrAssertion
	}
	sig, err := assertionEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return Assertion{}, ErrAssertion
	}
	// The signature first, over the payload as it arrived: nothing unsigned
	// is decoded, let alone believed.
	if !ed25519.Verify(key, []byte(assertionContext+parts[1]), sig) {
		return Assertion{}, ErrAssertion
	}
	raw, err := assertionEncoding.DecodeString(parts[1])
	if err != nil {
		return Assertion{}, ErrAssertion
	}
	var wire assertionWire
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return Assertion{}, ErrAssertion
	}
	a := Assertion{
		Audience: wire.Audience, Subject: wire.Subject,
		IssuedAt: time.Unix(wire.IssuedAt, 0).UTC(), ExpiresAt: time.Unix(wire.Expires, 0).UTC(),
		Nonce: wire.Nonce,
	}
	if err := a.check(); err != nil || a.Audience != venuePublicID {
		return Assertion{}, ErrAssertion
	}
	if now.Before(a.IssuedAt.Add(-AssertionLeeway)) || !now.Before(a.KeepUntil()) {
		return Assertion{}, ErrAssertionTime
	}
	return a, nil
}

// check is what any assertion must be, whoever it is for and whenever it is
// read: one this build would sign and one it would accept.
func (a Assertion) check() error {
	switch {
	case a.Audience == "":
		return errors.New("an assertion names the venue it is for")
	case a.Subject != SubjectOwner:
		return fmt.Errorf("an assertion is for the %s and nobody else", SubjectOwner)
	case len(a.Nonce) < minNonce || len(a.Nonce) > maxNonce || !urlSafe(a.Nonce):
		return fmt.Errorf("a nonce is %d to %d letters, digits, - and _", minNonce, maxNonce)
	case !a.ExpiresAt.After(a.IssuedAt) || a.ExpiresAt.Sub(a.IssuedAt) > AssertionLife:
		return fmt.Errorf("an assertion is good for %s at most", AssertionLife)
	}
	return nil
}

func urlSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
