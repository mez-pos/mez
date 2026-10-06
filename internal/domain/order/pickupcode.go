package order

import (
	"fmt"
	"io"
	"slices"
)

// A pickup code is what is called out across a counter when an order to
// collect is ready (docs/02-domain.md, phase 11 Task 5f1b, ADR-249): three
// characters, or four when three are running short.
//
// It has no vowels. The domain asks for a profanity list in English and in
// the venue's language; a code with no vowel in it spells almost nothing in
// any language, which is a filter that needs no list kept up. The few bare
// consonant runs that still read as something are refused by name.
//
// It has no 0 or 1 either, and none of the letters read aloud as each other
// are both in it, so "B 7 K" said over a coffee machine is one code.

// pickupAlphabet is what a code is drawn from: digits and consonants, with
// 0, 1 and the vowels left out.
const pickupAlphabet = "23456789BCDFGHJKMNPQRSTVWXZ"

// PickupCodeLength is how long a code is, and PickupCodeLong how long it is
// when the short ones are nearly all out.
const (
	PickupCodeLength = 3
	PickupCodeLong   = 4
)

// refusedPickupCodes are the runs of consonants that are still read as a
// word or a slur with no vowel in them.
var refusedPickupCodes = []string{"FCK", "FKN", "FKR", "SHT", "CNT", "DCK", "CCK", "KKK", "NGR", "NGG", "FGT", "BTCH", "DMN", "WTF", "STD", "XXX"}

// NewPickupCode draws a code of the given length from random. It is
// unbiased, and never one of the refused.
func NewPickupCode(random io.Reader, length int) (string, error) {
	if length != PickupCodeLength && length != PickupCodeLong {
		return "", fmt.Errorf("a pickup code is %d or %d characters, not %d", PickupCodeLength, PickupCodeLong, length)
	}
	// The largest multiple of the alphabet that fits a byte: a byte at or
	// over it is thrown away, or the first letters would come up more often.
	limit := byte(256 - 256%len(pickupAlphabet))
	buf := make([]byte, 1)
	for {
		out := make([]byte, 0, length)
		for len(out) < length {
			if _, err := io.ReadFull(random, buf); err != nil {
				return "", fmt.Errorf("drawing a pickup code: %w", err)
			}
			if buf[0] >= limit {
				continue
			}
			out = append(out, pickupAlphabet[int(buf[0])%len(pickupAlphabet)])
		}
		if code := string(out); PickupCodeOK(code) {
			return code, nil
		}
	}
}

// PickupCodeOK reports whether s is a code that may be given: the right
// length, from the alphabet, and not one of the refused, whole or as the
// start or end of a longer one.
func PickupCodeOK(s string) bool {
	if len(s) != PickupCodeLength && len(s) != PickupCodeLong {
		return false
	}
	for i := range len(s) {
		if !containsByte(pickupAlphabet, s[i]) {
			return false
		}
	}
	return !slices.ContainsFunc(refusedPickupCodes, func(bad string) bool {
		return len(bad) <= len(s) && (s[:len(bad)] == bad || s[len(s)-len(bad):] == bad)
	})
}

func containsByte(s string, b byte) bool {
	for i := range len(s) {
		if s[i] == b {
			return true
		}
	}
	return false
}
