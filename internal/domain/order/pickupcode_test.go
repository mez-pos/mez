package order

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPickupCodeOK(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"three from the alphabet", "B7K", true},
		{"four from the alphabet", "B7KM", true},
		{"too short", "B7", false},
		{"too long", "B7KMN", false},
		{"nothing", "", false},
		{"a vowel", "BAK", false},
		{"a zero, read as O", "B0K", false},
		{"a one, read as I or L", "B1K", false},
		{"lower case", "b7k", false},
		{"a space", "B K", false},
		{"one of the refused", "FCK", false},
		{"one of the refused at the start of four", "FCK7", false},
		{"one of the refused at the end of four", "7FCK", false},
		{"a refused one of four", "BTCH", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PickupCodeOK(tt.code); got != tt.want {
				t.Errorf("PickupCodeOK(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
	// Every refused code is itself made of the alphabet: one that is not
	// could never be drawn, and would be a line that guards nothing.
	for _, bad := range refusedPickupCodes {
		for i := range len(bad) {
			if !strings.ContainsRune(pickupAlphabet, rune(bad[i])) {
				t.Errorf("the refused code %q has %q in it, which is never drawn", bad, bad[i])
			}
		}
	}
	if strings.ContainsAny(pickupAlphabet, "AEIOU01") {
		t.Errorf("the alphabet %q has a vowel, a 0 or a 1 in it", pickupAlphabet)
	}
}

func TestNewPickupCode(t *testing.T) {
	seen := map[string]bool{}
	for range 5000 {
		for _, length := range []int{PickupCodeLength, PickupCodeLong} {
			code, err := NewPickupCode(rand.Reader, length)
			if err != nil {
				t.Fatal(err)
			}
			if len(code) != length || !PickupCodeOK(code) {
				t.Fatalf("drew %q for a length of %d", code, length)
			}
			seen[code] = true
		}
	}
	if len(seen) < 4000 {
		t.Errorf("only %d different codes in 10000 draws", len(seen))
	}

	t.Run("a refused code is drawn past", func(t *testing.T) {
		// Bytes that spell FCK, then bytes that spell B7K.
		at := func(c byte) byte { return byte(strings.IndexByte(pickupAlphabet, c)) }
		src := bytes.NewReader([]byte{at('F'), at('C'), at('K'), at('B'), at('7'), at('K')})
		code, err := NewPickupCode(src, PickupCodeLength)
		if err != nil || code != "B7K" {
			t.Errorf("code = %q, %v; want B7K", code, err)
		}
	})
	t.Run("a byte past the alphabet's multiple is thrown away", func(t *testing.T) {
		code, err := NewPickupCode(bytes.NewReader([]byte{255, 0, 255, 1, 2}), PickupCodeLength)
		if err != nil || code != pickupAlphabet[:3] {
			t.Errorf("code = %q, %v; want %q", code, err, pickupAlphabet[:3])
		}
	})
	t.Run("a source that runs dry is an error", func(t *testing.T) {
		if _, err := NewPickupCode(bytes.NewReader([]byte{0}), PickupCodeLength); !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a length that is not a code's", func(t *testing.T) {
		if _, err := NewPickupCode(rand.Reader, 5); err == nil {
			t.Error("a code of five was drawn")
		}
	})
}
