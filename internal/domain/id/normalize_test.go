package id

import (
	"errors"
	"strings"
	"testing"
)

func TestAlphabetIsCrockford(t *testing.T) {
	if len(Alphabet) != 32 {
		t.Fatalf("alphabet has %d symbols, want 32", len(Alphabet))
	}

	// The four letters Crockford leaves out are the ones that are misheard or
	// misread across a noisy room, which is the whole reason for this
	// alphabet.
	for _, excluded := range []string{"I", "L", "O", "U"} {
		if strings.Contains(Alphabet, excluded) {
			t.Errorf("alphabet contains %q, which Crockford base-32 excludes", excluded)
		}
	}

	seen := map[byte]bool{}
	for i := range len(Alphabet) {
		c := Alphabet[i]
		if seen[c] {
			t.Errorf("alphabet repeats %q", string(c))
		}
		seen[c] = true
		if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'Z') {
			t.Errorf("alphabet contains %q, which is not an uppercase digit or letter", string(c))
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "already canonical", in: "7K3M9QX2", want: "7K3M9QX2"},
		{name: "lowercase", in: "7k3m9qx2", want: "7K3M9QX2"},
		{name: "mixed case", in: "7k3M9qX2", want: "7K3M9QX2"},

		// The substitutions staff actually make when reading a code off a
		// receipt and typing it back in.
		{name: "letter O for zero", in: "7K3MOQX2", want: "7K3M0QX2"},
		{name: "lowercase o for zero", in: "7k3moqx2", want: "7K3M0QX2"},
		{name: "letter I for one", in: "7K3MIQX2", want: "7K3M1QX2"},
		{name: "letter L for one", in: "7K3MLQX2", want: "7K3M1QX2"},
		{name: "lowercase l for one", in: "7k3mlqx2", want: "7K3M1QX2"},
		{name: "every substitution at once", in: "oil", want: "011"},

		// Crockford ignores hyphens, and staff add spaces when reading aloud.
		{name: "hyphens are ignored", in: "7K3M-9QX2", want: "7K3M9QX2"},
		{name: "spaces are ignored", in: "7K3M 9QX2", want: "7K3M9QX2"},
		{name: "surrounding space", in: "  7K3M9QX2  ", want: "7K3M9QX2"},

		{name: "empty", in: "", wantErr: ErrInvalidCode},
		{name: "only separators", in: " - ", wantErr: ErrInvalidCode},
		{name: "an excluded letter has no reading", in: "7K3M9QXU", wantErr: ErrInvalidCode},
		{name: "punctuation", in: "7K3M9QX!", wantErr: ErrInvalidCode},
		{name: "not ascii", in: "7K3M9QXé", wantErr: ErrInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Normalize(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestNormalizeIsIdempotent checks the property a lookup depends on: whatever
// a human typed, normalising once is enough to compare against what is stored.
func TestNormalizeIsIdempotent(t *testing.T) {
	inputs := []string{"7K3M9QX2", "7k3m-9qx2", "oil", "ILO", "0123456789", "abcdefgh"}

	for _, in := range inputs {
		once, err := Normalize(in)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", in, err)
		}
		twice, err := Normalize(once)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", once, err)
		}
		if once != twice {
			t.Errorf("Normalize(%q) = %q, but normalising that gives %q", in, once, twice)
		}
	}
}
