package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// keyPrefix says what a venue key is when it turns up in a support ticket or
// a secret scanner.
const keyPrefix = "mzr_"

// MinSecretLen is the shortest signing secret the relay will start with.
const MinSecretLen = 32

// keyIDBytes is the random part of a key. It is not a secret: it names the
// key in logs, and it is what a revocation lists.
const keyIDBytes = 6

// Errors a venue key can fail with. The agent endpoint answers all of them
// the same way; the difference is for the log.
var (
	ErrKeyMalformed = errors.New("relay: venue key is malformed")
	ErrKeyInvalid   = errors.New("relay: venue key is not one this relay issued")
	ErrKeyRevoked   = errors.New("relay: venue key is revoked")
)

// Keys issues and checks venue keys.
//
// The relay stores nothing (ADR-008), so a key is not looked up: it is
// "mzr_<venue>.<key id>.<mac>", where the mac is an HMAC over the other two
// under the relay's one secret. Whoever holds the secret can issue a key, and
// the relay can check one, with no table of venues anywhere.
type Keys struct {
	secret  []byte
	revoked map[string]struct{}
}

// NewKeys builds a key checker. revoked lists venue ids and key ids that are
// refused although their mac is good: the one way to turn a key off.
func NewKeys(secret string, revoked []string) (*Keys, error) {
	if len(secret) < MinSecretLen {
		return nil, fmt.Errorf("relay: secret is %d characters; it must be at least %d", len(secret), MinSecretLen)
	}
	set := make(map[string]struct{}, len(revoked))
	for _, r := range revoked {
		if r = strings.TrimSpace(r); r != "" {
			set[r] = struct{}{}
		}
	}
	return &Keys{secret: []byte(secret), revoked: set}, nil
}

// scopeRemote is the one scope a key can carry beyond guests: the venue's
// back office may cross the relay too (ADR-306). It is signed into the key,
// so a venue cannot add it to a key it was given.
const scopeRemote = "remote"

// Key is what a verified key says: whose it is, which one it is, and whether
// the venue may be reached beyond the guest app.
type Key struct {
	VenueID string
	KeyID   string
	// Remote is true for a key issued with the remote scope: the relay then
	// forwards /backoffice/ and /api/ for this venue. For every other key
	// those paths are answered 404 at the relay.
	Remote bool
}

// Issue draws a new key for a venue. random is crypto/rand.Reader outside
// tests. remote adds the scope that lets the venue's back office through.
func (k *Keys) Issue(random io.Reader, venueID string, remote bool) (string, error) {
	if Reserved(venueID) {
		return "", fmt.Errorf("relay: venue id %q is reserved: it would be a hostname the relay or its operator needs", venueID)
	}
	if !ValidVenueID(venueID) {
		return "", fmt.Errorf("relay: venue id %q: use 1 to 63 of a-z, 0-9 and '-', not starting with '-'", venueID)
	}
	var b [keyIDBytes]byte
	if _, err := io.ReadFull(random, b[:]); err != nil {
		return "", fmt.Errorf("relay: drawing a key id: %w", err)
	}
	keyID := hex.EncodeToString(b[:])
	scope := ""
	if remote {
		scope = scopeRemote
	}
	key := keyPrefix + venueID + "." + keyID + "."
	if scope != "" {
		key += scope + "."
	}
	return key + k.mac(venueID, keyID, scope), nil
}

// Verify checks a presented key and returns the venue it speaks for and the
// key's id. Check says the rest.
func (k *Keys) Verify(key string) (venueID, keyID string, err error) {
	c, err := k.Check(key)
	return c.VenueID, c.KeyID, err
}

// Check verifies a presented key: "mzr_<venue>.<key id>.<mac>" is a guests-only
// key, "mzr_<venue>.<key id>.remote.<mac>" one whose venue may have its back
// office carried too. The mac covers the scope, so a scope written into a key
// after it was issued fails as any other forgery does.
func (k *Keys) Check(key string) (Key, error) {
	body, ok := strings.CutPrefix(key, keyPrefix)
	if !ok {
		return Key{}, ErrKeyMalformed
	}
	parts := strings.Split(body, ".")
	scope := ""
	switch len(parts) {
	case 3:
	case 4:
		if parts[2] != scopeRemote {
			return Key{}, ErrKeyMalformed
		}
		scope = scopeRemote
	default:
		return Key{}, ErrKeyMalformed
	}
	if !ValidVenueID(parts[0]) || len(parts[1]) != keyIDBytes*2 {
		return Key{}, ErrKeyMalformed
	}
	venueID, keyID, mac := parts[0], parts[1], parts[len(parts)-1]
	if !hmac.Equal([]byte(mac), []byte(k.mac(venueID, keyID, scope))) {
		return Key{}, ErrKeyInvalid
	}
	// Checked after the mac, so the answer never says which ids are listed
	// to someone who cannot sign.
	if _, gone := k.revoked[venueID]; gone {
		return Key{}, ErrKeyRevoked
	}
	if _, gone := k.revoked[keyID]; gone {
		return Key{}, ErrKeyRevoked
	}
	return Key{VenueID: venueID, KeyID: keyID, Remote: scope == scopeRemote}, nil
}

func (k *Keys) mac(venueID, keyID, scope string) string {
	h := hmac.New(sha256.New, k.secret)
	// The label keeps this secret's macs from meaning anything elsewhere, and
	// the newline keeps "ab"+"c" from signing the same bytes as "a"+"bc". A
	// guests-only key signs exactly what it did before the scope existed, so
	// every key issued until now still verifies.
	h.Write([]byte("mez-relay-key-v1\n" + venueID + "\n" + keyID))
	if scope != "" {
		h.Write([]byte("\n" + scope))
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// reserved are names no venue may have. With a hostname per venue a venue id
// is a label under the operator's domain, and these are the ones the relay
// itself, or anyone running a domain, expects to mean something else.
var reserved = map[string]struct{}{
	"relay": {}, "www": {}, "api": {}, "admin": {}, "app": {}, "mail": {},
	"smtp": {}, "ns1": {}, "ns2": {}, "status": {}, "support": {}, "order": {},
}

// Reserved reports whether id is a name kept back from venues.
func Reserved(id string) bool {
	_, ok := reserved[id]
	return ok
}

// ValidVenueID reports whether id can name a venue on the relay. It is a URL
// path segment and a field in every log line, so it is kept to what needs no
// escaping in either.
func ValidVenueID(id string) bool {
	if id == "" || len(id) > 63 || id[0] == '-' {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}
