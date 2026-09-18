package id

import (
	"fmt"
	"strings"
)

// Alphabet is Crockford base-32: the ten digits and the uppercase letters,
// less I, L, O and U.
//
// I and L look like 1, O looks like 0, and U is dropped so that the alphabet
// cannot spell the shorter English obscenities by accident. Staff read these
// codes aloud across a room with a grinder running, so the alphabet is chosen
// for ears and eyes rather than for density.
const Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// symbolMask takes the low five bits of a byte. Thirty two divides two hundred
// and fifty six exactly, so every symbol is equally likely and there is no
// modulo bias to correct for.
const symbolMask = 0x1f

// Normalize canonicalises a code somebody typed or read back over the phone:
// it uppercases, resolves the substitutions the excluded letters exist to
// survive, and drops the hyphens and spaces people add to group digits.
//
// Call it on anything that arrives from outside before comparing it with a
// stored code. Normalizing twice gives the same answer as normalizing once, so
// it is safe to apply at more than one boundary.
func Normalize(code string) (string, error) {
	var out strings.Builder
	out.Grow(len(code))

	for i := range len(code) {
		c := code[i]

		if c == '-' || c == ' ' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		switch c {
		case 'O':
			c = '0'
		case 'I', 'L':
			c = '1'
		}

		if strings.IndexByte(Alphabet, c) < 0 {
			return "", fmt.Errorf("id: %q: %q has no reading in this alphabet: %w", code, string(code[i]), ErrInvalidCode)
		}
		out.WriteByte(c)
	}

	if out.Len() == 0 {
		return "", fmt.Errorf("id: %q: no code characters: %w", code, ErrInvalidCode)
	}
	return out.String(), nil
}
