package id

import (
	"strings"
	"testing"
)

// draws is the sample size the phase file asks for. A busy venue writes a few
// hundred orders a day, so a million draws is several lifetimes of one till.
const draws = 1_000_000

// TestPublicCodeCollisionSanity draws a million public codes and counts the
// repeats.
//
// The bar is deliberately not zero. A body of eight symbols is forty bits, so
// a million draws are expected to repeat about 0.45 times by the birthday
// bound: n squared over twice 2^40. Demanding zero would make this test fail
// roughly a third of the time for a perfectly good generator. What it is
// really looking for is a broken one, which does not produce a handful of
// repeats but thousands.
//
// Forty bits is not enough to make a public ID unique by itself, and this
// package does not claim it is. Uniqueness belongs to the unique index on the
// column, with a redraw on conflict. See ADR-034.
func TestPublicCodeCollisionSanity(t *testing.T) {
	const tolerated = 10

	seen := make(map[uint64]struct{}, draws)
	repeats := 0

	for range draws {
		code, err := NewPublic("ord")
		if err != nil {
			t.Fatalf("NewPublic: %v", err)
		}

		packed := pack(t, code[len("ord_"):])
		if _, ok := seen[packed]; ok {
			repeats++
		}
		seen[packed] = struct{}{}
	}

	t.Logf("%d repeats in %d draws", repeats, draws)
	if repeats > tolerated {
		t.Errorf("%d repeats in %d draws, tolerating %d: the generator is not random enough",
			repeats, draws, tolerated)
	}
}

// TestPublicCodeUsesTheWholeAlphabet is the other half of the sanity check. A
// generator that never emits some symbols, or favours the low half of the
// alphabet, would pass a collision count long before it became obvious in
// production.
func TestPublicCodeUsesTheWholeAlphabet(t *testing.T) {
	const sample = 20_000

	counts := make([][]int, PublicCodeLength)
	for i := range counts {
		counts[i] = make([]int, len(Alphabet))
	}

	for range sample {
		code, err := NewPublic("ord")
		if err != nil {
			t.Fatalf("NewPublic: %v", err)
		}
		body := code[len("ord_"):]
		for position := range len(body) {
			counts[position][strings.IndexByte(Alphabet, body[position])]++
		}
	}

	// Each symbol is expected at each position sample/32 times, 625 for this
	// sample. The band below is wide enough that a fair generator will not
	// trip it and narrow enough to catch one that ignores a symbol or leans
	// on a few.
	expected := sample / len(Alphabet)
	low, high := expected/2, expected*2

	for position, row := range counts {
		for symbol, count := range row {
			if count < low || count > high {
				t.Errorf("symbol %q appeared %d times at position %d, expected near %d",
					string(Alphabet[symbol]), count, position, expected)
			}
		}
	}
}

// pack turns a code body into the integer it stands for, so the collision test
// can hold a million of them without holding a million strings.
func pack(t *testing.T, body string) uint64 {
	t.Helper()

	var packed uint64
	for i := range len(body) {
		index := strings.IndexByte(Alphabet, body[i])
		if index < 0 {
			t.Fatalf("code body %q contains %q", body, string(body[i]))
		}
		packed = packed<<5 | uint64(index)
	}
	return packed
}
