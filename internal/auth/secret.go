package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// deviceTokenPrefix marks a device token, so one pasted into a support
// ticket or caught by a secret scanner says what it is.
const deviceTokenPrefix = "nfd_"

// deviceTokenBytes is the entropy in a device token: 256 bits.
const deviceTokenBytes = 32

// newDeviceToken draws a device token and returns it with its hash. The
// token goes to the device once; the hash is what is stored.
func newDeviceToken(src io.Reader) (token, hash string, err error) {
	var b [deviceTokenBytes]byte
	if _, err := io.ReadFull(src, b[:]); err != nil {
		return "", "", fmt.Errorf("auth: drawing a device token: %w", err)
	}
	token = deviceTokenPrefix + base64.RawURLEncoding.EncodeToString(b[:])
	return token, hashSecret(token), nil
}

// deviceTokenHash checks a presented token's shape and returns its hash. A
// token of the wrong shape is refused before it reaches the database.
func deviceTokenHash(token string) (string, bool) {
	body, ok := strings.CutPrefix(token, deviceTokenPrefix)
	if !ok {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) != deviceTokenBytes {
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
