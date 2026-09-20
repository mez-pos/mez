package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Token prefixes, so a token pasted into a support ticket or caught by a
// secret scanner says what it is.
const (
	deviceTokenPrefix  = "nfd_"
	sessionTokenPrefix = "nfs_"
)

// tokenBytes is the entropy in a token: 256 bits.
const tokenBytes = 32

// newToken draws a token with a prefix and returns it with its hash. The
// token is handed out once; the hash is what is stored.
func newToken(src io.Reader, prefix string) (token, hash string, err error) {
	var b [tokenBytes]byte
	if _, err := io.ReadFull(src, b[:]); err != nil {
		return "", "", fmt.Errorf("auth: drawing a token: %w", err)
	}
	token = prefix + base64.RawURLEncoding.EncodeToString(b[:])
	return token, hashSecret(token), nil
}

// tokenHash checks a presented token's shape and returns its hash. A token
// of the wrong shape is refused before it reaches the database.
func tokenHash(token, prefix string) (string, bool) {
	body, ok := strings.CutPrefix(token, prefix)
	if !ok {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) != tokenBytes {
		return "", false
	}
	return hashSecret(token), true
}

// hashSecret is SHA-256 in hex. It is right for a secret with enough entropy
// that guessing it is hopeless however fast the hash, and wrong for a PIN.
func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
