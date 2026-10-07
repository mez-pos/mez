package firstrun

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TokenFile is where the setup token is kept, in the data directory, while
// the venue is still to be made.
const TokenFile = "setup-token"

// tokenBytes is the token's length before it is written as hex: 128 bits,
// which nobody guesses at ten tries a minute.
const tokenBytes = 16

// Token is the setup token (ADR-270): what a browser must hold, besides
// being on the server's own machine, to make the venue over HTTP.
//
// Being on the machine was all ADR-266 asked, and on a machine that is any
// user and any program. The token is in the data directory, which only an
// administrator and the service's own account can read (ADR-259, ADR-264),
// so holding it is what running `mez setup` there was: proof of being
// someone who may. It is drawn once, kept across restarts so a link
// already handed out still works, and deleted when the venue exists.
type Token struct {
	path string

	mu    sync.Mutex
	value string // "" once spent, or when no setup is needed
}

// OpenToken readies the token in dir. With a venue still to be made it is
// the one already there, or a new one, written before this returns; with
// none needed, any file left behind is removed and no token opens anything.
func OpenToken(dir string, needed bool) (*Token, error) {
	t := &Token{path: filepath.Join(dir, TokenFile)}
	if !needed {
		return t, t.remove()
	}
	if have, err := ReadToken(dir); err == nil {
		t.value = have
		return t, nil
	}
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("firstrun: drawing the setup token: %w", err)
	}
	value := hex.EncodeToString(raw)
	// Owner-only where modes mean something. On Windows the directory's
	// ACL is what keeps it, and the directory is restricted before this.
	if err := os.WriteFile(t.path, []byte(value+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("firstrun: writing the setup token: %w", err)
	}
	t.value = value
	return t, nil
}

// ErrNoToken means there is no setup token to read: the venue exists, the
// server has not been started since the data directory was made, or the
// file is not one this wrote.
var ErrNoToken = errors.New("there is no setup token")

// ReadToken reads the token kept in dir. Reading it is the test of being
// allowed it: the directory's permissions decide.
func ReadToken(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, TokenFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoToken
	}
	if err != nil {
		return "", fmt.Errorf("firstrun: reading the setup token: %w", err)
	}
	value := strings.TrimSpace(string(raw))
	if !wellFormed(value) {
		return "", ErrNoToken
	}
	return value, nil
}

// wellFormed reports whether a value is a token as OpenToken writes one. A
// file that is anything else is replaced, never trusted: an empty file must
// not make the empty string a token.
func wellFormed(value string) bool {
	if len(value) != hex.EncodedLen(tokenBytes) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

// Valid reports whether candidate is the token, in constant time. Nothing
// is the token once it is spent.
func (t *Token) Valid(candidate string) bool {
	t.mu.Lock()
	value := t.value
	t.mu.Unlock()
	if value == "" || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(candidate)) == 1
}

// Spend ends the token: the venue exists, and nothing opens setup again.
func (t *Token) Spend() error {
	t.mu.Lock()
	t.value = ""
	t.mu.Unlock()
	return t.remove()
}

func (t *Token) remove() error {
	if err := os.Remove(t.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("firstrun: removing the setup token: %w", err)
	}
	return nil
}

// SetupLink is the address that opens the wizard holding the token. The
// token is in the fragment, the part of an address a browser keeps to
// itself: it is not sent to the server, so it reaches no access log.
func SetupLink(base, token string) string {
	return strings.TrimSuffix(base, "/") + "/setup#token=" + token
}
