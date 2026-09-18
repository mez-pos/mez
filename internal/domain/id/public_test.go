package id

import (
	"errors"
	"strings"
	"testing"
)

func TestNewPublicShape(t *testing.T) {
	code, err := NewPublic("ord")
	if err != nil {
		t.Fatalf("NewPublic: %v", err)
	}

	prefix, body, found := strings.Cut(code, "_")
	if !found {
		t.Fatalf("NewPublic = %q, want a prefix and a body separated by _", code)
	}
	if prefix != "ord" {
		t.Errorf("prefix = %q, want %q", prefix, "ord")
	}
	if len(body) != PublicCodeLength {
		t.Errorf("body %q is %d characters, want %d", body, len(body), PublicCodeLength)
	}
	for i := range len(body) {
		if !strings.ContainsRune(Alphabet, rune(body[i])) {
			t.Errorf("body %q contains %q, which is not a code character", body, string(body[i]))
		}
	}
}

func TestPublicUsesTheEntropyItIsGiven(t *testing.T) {
	// Each byte of entropy becomes one symbol from its low five bits. Thirty
	// two divides two hundred and fifty six exactly, so masking is uniform and
	// there is no modulo bias to correct for.
	source := &repeatingReader{pattern: symbolBytes("7K3M9QX2")}

	got, err := NewGenerator(source).Public("ord")
	if err != nil {
		t.Fatalf("Public: %v", err)
	}
	if got != "ord_7K3M9QX2" {
		t.Errorf("Public = %q, want %q", got, "ord_7K3M9QX2")
	}
}

func TestPublicUsesOnlyTheLowFiveBits(t *testing.T) {
	// The high three bits of every byte are set. If they leaked into the
	// symbol choice the result would be out of range rather than the same
	// code as the low bits alone.
	plain := &repeatingReader{pattern: symbolBytes("7K3M9QX2")}
	noisy := &repeatingReader{pattern: withHighBits(symbolBytes("7K3M9QX2"))}

	want, err := NewGenerator(plain).Public("ord")
	if err != nil {
		t.Fatalf("Public: %v", err)
	}
	got, err := NewGenerator(noisy).Public("ord")
	if err != nil {
		t.Fatalf("Public: %v", err)
	}
	if got != want {
		t.Errorf("Public with the high bits set = %q, want %q", got, want)
	}
}

func withHighBits(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c | 0xe0
	}
	return out
}

func TestNewPublicPrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr error
	}{
		{name: "an order", prefix: "ord"},
		{name: "an invoice", prefix: "inv"},
		{name: "one letter", prefix: "o"},
		{name: "eight letters", prefix: "abcdefgh"},
		{name: "empty", prefix: "", wantErr: ErrInvalidPrefix},
		{name: "too long", prefix: "abcdefghi", wantErr: ErrInvalidPrefix},
		{name: "uppercase", prefix: "ORD", wantErr: ErrInvalidPrefix},
		{name: "digits", prefix: "or1", wantErr: ErrInvalidPrefix},
		{name: "the separator itself", prefix: "or_d", wantErr: ErrInvalidPrefix},
		{name: "a hyphen", prefix: "or-d", wantErr: ErrInvalidPrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPublic(tt.prefix)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewPublic(%q) error = %v, want %v", tt.prefix, err, tt.wantErr)
			}
			if tt.wantErr == nil && !strings.HasPrefix(got, tt.prefix+"_") {
				t.Errorf("NewPublic(%q) = %q, which does not start with the prefix", tt.prefix, got)
			}
		})
	}
}

func TestPublicReportsAFailingEntropySource(t *testing.T) {
	if _, err := NewGenerator(errReader{}).Public("ord"); !errors.Is(err, ErrEntropy) {
		t.Errorf("Public with a failed source: error = %v, want %v", err, ErrEntropy)
	}
	if _, err := NewGenerator(errReader{}).Public("ord"); !errors.Is(err, errSourceFailed) {
		t.Error("Public dropped the underlying source error")
	}
	if _, err := NewGenerator(&shortReader{left: 3}).Public("ord"); !errors.Is(err, ErrEntropy) {
		t.Errorf("Public with a truncated source: error = %v, want %v", err, ErrEntropy)
	}
}

func TestParsePublic(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantPrefix string
		wantCode   string
		wantErr    error
	}{
		{name: "canonical", in: "ord_7K3M9QX2", wantPrefix: "ord", wantCode: "7K3M9QX2"},
		{name: "typed in lowercase", in: "ord_7k3m9qx2", wantPrefix: "ord", wantCode: "7K3M9QX2"},
		{name: "shouted in uppercase", in: "ORD_7K3M9QX2", wantPrefix: "ord", wantCode: "7K3M9QX2"},
		{name: "read aloud with the letter O", in: "ord_7K3MOQX2", wantPrefix: "ord", wantCode: "7K3M0QX2"},
		{name: "read aloud with the letter L", in: "ord_7K3MLQX2", wantPrefix: "ord", wantCode: "7K3M1QX2"},
		{name: "grouped with a hyphen", in: "ord_7K3M-9QX2", wantPrefix: "ord", wantCode: "7K3M9QX2"},
		{name: "pasted with space around it", in: "  ord_7K3M9QX2 ", wantPrefix: "ord", wantCode: "7K3M9QX2"},

		{name: "no separator", in: "ord7K3M9QX2", wantErr: ErrInvalidCode},
		{name: "no prefix", in: "_7K3M9QX2", wantErr: ErrInvalidPrefix},
		{name: "no code", in: "ord_", wantErr: ErrInvalidCode},
		{name: "too short", in: "ord_7K3M9QX", wantErr: ErrInvalidCode},
		{name: "too long", in: "ord_7K3M9QX22", wantErr: ErrInvalidCode},
		{name: "an excluded letter", in: "ord_7K3M9QXU", wantErr: ErrInvalidCode},
		{name: "empty", in: "", wantErr: ErrInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, code, err := ParsePublic(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParsePublic(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if prefix != tt.wantPrefix {
				t.Errorf("ParsePublic(%q) prefix = %q, want %q", tt.in, prefix, tt.wantPrefix)
			}
			if code != tt.wantCode {
				t.Errorf("ParsePublic(%q) code = %q, want %q", tt.in, code, tt.wantCode)
			}
		})
	}
}

// TestParsePublicRoundTripsNewPublic is the pair that matters in production:
// an ID this package generated is one it can read back.
func TestParsePublicRoundTripsNewPublic(t *testing.T) {
	for range 100 {
		want, err := NewPublic("ord")
		if err != nil {
			t.Fatalf("NewPublic: %v", err)
		}

		prefix, code, err := ParsePublic(want)
		if err != nil {
			t.Fatalf("ParsePublic(%q): %v", want, err)
		}
		if got := prefix + "_" + code; got != want {
			t.Errorf("ParsePublic(%q) rebuilt as %q", want, got)
		}
	}
}
