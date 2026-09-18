package id

import (
	"fmt"
	"strings"
)

// PublicCodeLength is how many symbols follow the prefix in a public ID.
// Eight symbols is forty bits.
const PublicCodeLength = 8

// maxPrefixLength keeps a public ID short enough to read aloud. The convention
// is three letters: ord, inv, dev, pay.
const maxPrefixLength = 8

// NewPublic returns a public ID such as ord_7K3M9QX2, using crypto/rand.
//
// Public IDs are what leaves the building: order codes on a receipt, invoice
// references, device pairing codes. They are random rather than sequential so
// that one customer cannot count to another customer's order, and they carry a
// prefix so that a code in a support email says what it is.
//
// Forty bits is not enough to make these unique on its own. The unique index
// on the column is what guarantees that, with a redraw on conflict; see
// ADR-034 before using one as a key.
func NewPublic(prefix string) (string, error) {
	return NewGenerator(nil).Public(prefix)
}

// Public returns a public ID drawn from this generator's entropy source.
func (g *Generator) Public(prefix string) (string, error) {
	if err := checkPrefix(prefix); err != nil {
		return "", err
	}

	code, err := g.symbols(PublicCodeLength)
	if err != nil {
		return "", err
	}
	return prefix + "_" + code, nil
}

// ParsePublic reads a public ID back, tolerating everything a human does to
// one between the receipt and the keyboard: any case, the letter O for zero,
// I or L for one, and hyphens or spaces used to group the symbols.
//
// It returns the prefix and the canonical code, which is what a lookup
// compares against.
func ParsePublic(s string) (prefix, code string, err error) {
	prefix, code, found := strings.Cut(strings.TrimSpace(s), "_")
	if !found {
		return "", "", fmt.Errorf("id: %q: no prefix separator: %w", s, ErrInvalidCode)
	}

	prefix = strings.ToLower(prefix)
	if err := checkPrefix(prefix); err != nil {
		return "", "", fmt.Errorf("id: %q: %w", s, err)
	}

	code, err = Normalize(code)
	if err != nil {
		return "", "", err
	}
	if len(code) != PublicCodeLength {
		return "", "", fmt.Errorf("id: %q: %d symbols after the prefix, want %d: %w",
			s, len(code), PublicCodeLength, ErrInvalidCode)
	}

	return prefix, code, nil
}

// checkPrefix reports whether a prefix is one to eight lowercase ASCII
// letters. Lowercase only, so that the prefix can never be confused with the
// code, which is always uppercase.
func checkPrefix(prefix string) error {
	if len(prefix) == 0 || len(prefix) > maxPrefixLength {
		return fmt.Errorf("id: prefix %q is %d characters: %w", prefix, len(prefix), ErrInvalidPrefix)
	}
	for i := range len(prefix) {
		if prefix[i] < 'a' || prefix[i] > 'z' {
			return fmt.Errorf("id: prefix %q contains %q: %w", prefix, string(prefix[i]), ErrInvalidPrefix)
		}
	}
	return nil
}
