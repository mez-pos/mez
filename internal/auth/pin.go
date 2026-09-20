package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// PIN length bounds. Four digits is what staff will choose; eight is the most
// anyone types with a queue watching.
const (
	MinPINLength = 4
	MaxPINLength = 8
)

// argonParams are Argon2id's cost settings. They are written into every hash,
// so raising them later affects new PINs only and old ones still verify.
type argonParams struct {
	memoryKiB uint32
	time      uint32
	threads   uint8
}

// pinParams is OWASP's minimum for Argon2id: 19 MiB, two passes, one lane.
// On a two-core venue PC that is tens of milliseconds, which is invisible at
// a PIN pad, and enough to make a stolen database's four-digit PINs cost
// something to recover rather than nothing.
var pinParams = argonParams{memoryKiB: 19 * 1024, time: 2, threads: 1}

const (
	saltBytes = 16
	keyBytes  = 32
)

var (
	// ErrPINFormat means a PIN that is not 4 to 8 digits.
	ErrPINFormat = fmt.Errorf("a PIN is %d to %d digits", MinPINLength, MaxPINLength)

	// errHashFormat means a stored hash this package cannot read.
	errHashFormat = errors.New("auth: stored PIN hash is not an argon2id hash")
)

// checkPIN refuses anything but 4 to 8 ASCII digits.
func checkPIN(pin string) error {
	if len(pin) < MinPINLength || len(pin) > MaxPINLength {
		return ErrPINFormat
	}
	for i := range len(pin) {
		if pin[i] < '0' || pin[i] > '9' {
			return ErrPINFormat
		}
	}
	return nil
}

// HashPIN hashes a PIN for staff.pin_hash, as a PHC string:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>.
func HashPIN(pin string) (string, error) {
	if err := checkPIN(pin); err != nil {
		return "", err
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: drawing a salt: %w", err)
	}
	return encodeHash(pinParams, salt, derive(pin, salt, pinParams)), nil
}

// verifyPIN reports whether pin matches a stored hash, comparing in constant
// time. A malformed hash is an error, never a match.
func verifyPIN(stored, pin string) (bool, error) {
	p, salt, want, err := decodeHash(stored)
	if err != nil {
		return false, err
	}
	got := derive(pin, salt, p)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func derive(pin string, salt []byte, p argonParams) []byte {
	return argon2.IDKey([]byte(pin), salt, p.time, p.memoryKiB, p.threads, keyBytes)
}

func encodeHash(p argonParams, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memoryKiB, p.time, p.threads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

// maxMemoryKiB bounds the memory a stored hash may ask for. A hash is read
// from the database, and one written with absurd parameters must not be able
// to take the machine down at the next sign-in.
const maxMemoryKiB = 256 * 1024

func decodeHash(s string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errHashFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, errHashFormat
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryKiB, &p.time, &p.threads); err != nil {
		return argonParams{}, nil, nil, errHashFormat
	}
	if p.memoryKiB < 8 || p.memoryKiB > maxMemoryKiB || p.time < 1 || p.time > 16 || p.threads < 1 {
		return argonParams{}, nil, nil, errHashFormat
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return argonParams{}, nil, nil, errHashFormat
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 {
		return argonParams{}, nil, nil, errHashFormat
	}
	return p, salt, key, nil
}

// dummyHash is compared against when the staff member asked for does not
// exist or cannot sign in, so that answer takes as long as a wrong PIN and
// the timing says nothing about who works here.
var dummyHash = sync.OnceValue(func() string {
	h, err := HashPIN("00000000")
	if err != nil {
		// crypto/rand failing is not something to carry on from.
		panic(fmt.Sprintf("auth: building the dummy PIN hash: %v", err))
	}
	return h
})
