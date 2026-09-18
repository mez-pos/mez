package id

import (
	"errors"
	"strings"
	"testing"
)

func TestNewPickupCodeShape(t *testing.T) {
	for range 1000 {
		code, err := NewPickupCode()
		if err != nil {
			t.Fatalf("NewPickupCode: %v", err)
		}
		if len(code) != PickupCodeLength {
			t.Fatalf("NewPickupCode = %q, which is %d characters, want %d", code, len(code), PickupCodeLength)
		}
		for i := range len(code) {
			if !strings.ContainsRune(Alphabet, rune(code[i])) {
				t.Fatalf("NewPickupCode = %q, which contains %q", code, string(code[i]))
			}
		}
		if NewBlocklist().Blocks(code) {
			t.Fatalf("NewPickupCode returned %q, which the blocklist rejects", code)
		}
	}
}

func TestPickupCodeRedrawsPastABlockedCode(t *testing.T) {
	// The source offers a blocked code first and an innocuous one second, so
	// the generator must discard the first and hand back the second.
	source := &repeatingReader{pattern: append(symbolBytes("5H1T"), symbolBytes("7K3M")...)}

	got, err := NewGenerator(source).PickupCode(NewBlocklist())
	if err != nil {
		t.Fatalf("PickupCode: %v", err)
	}
	if got != "7K3M" {
		t.Errorf("PickupCode = %q, want %q: the blocked code was not redrawn", got, "7K3M")
	}
}

func TestPickupCodeGivesUpRatherThanReturningABlockedCode(t *testing.T) {
	// A source stuck on one blocked code is a broken source. Failing is the
	// only honest answer; returning the code anyway would put it on a receipt.
	source := &repeatingReader{pattern: symbolBytes("5H1T")}

	got, err := NewGenerator(source).PickupCode(NewBlocklist())
	if !errors.Is(err, ErrEntropy) {
		t.Fatalf("PickupCode = %q, error = %v, want %v", got, err, ErrEntropy)
	}
}

func TestPickupCodeReportsAFailingEntropySource(t *testing.T) {
	if _, err := NewGenerator(errReader{}).PickupCode(NewBlocklist()); !errors.Is(err, ErrEntropy) {
		t.Errorf("PickupCode with a failed source: error = %v, want %v", err, ErrEntropy)
	}
}

func TestBlocklist(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		block bool
	}{
		{name: "an ordinary code", code: "7K3M", block: false},
		{name: "another ordinary code", code: "QZ84", block: false},
		{name: "all digits", code: "4821", block: false},

		// The alphabet has no I, L, O or U, so these words can only be spelled
		// with the digits that stand in for them. That is exactly why the
		// filter reads a code rather than matching it literally.
		{name: "one for the letter I", code: "5H1T", block: true},
		{name: "zero for the letter O", code: "C0CK", block: true},
		{name: "one for the letter I again", code: "D1CK", block: true},
		{name: "one for the letter L", code: "HE11", block: true},
		{name: "five for the letter S", code: "A55X", block: true},
		{name: "four for the letter A", code: "F4G7", block: true},
		{name: "three for the letter E", code: "5EX3", block: true},
		{name: "zero for O twice", code: "C00N", block: true},

		// A blocked word anywhere in the code is enough: it is read aloud as
		// a whole, and nobody hears the boundary.
		{name: "at the start", code: "ASSQ", block: true},
		{name: "at the end", code: "QASS", block: true},
		{name: "spelled plainly", code: "TWAT", block: true},

		{name: "lowercase is still blocked", code: "5h1t", block: true},

		// A word the alphabet cannot spell at all, because U is excluded and
		// no digit reads as U.
		{name: "unspellable without a U", code: "F0CK", block: false},
	}

	blocked := NewBlocklist()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blocked.Blocks(tt.code); got != tt.block {
				t.Errorf("Blocks(%q) = %v, want %v", tt.code, got, tt.block)
			}
		})
	}
}

func TestBlocklistTakesExtraWords(t *testing.T) {
	// The venue's own language. "kaka" is childish in several, and the built
	// in list is English only.
	blocked := NewBlocklist("kaka", "  gaand  ")

	tests := []struct {
		code  string
		block bool
	}{
		{code: "KAKA", block: true},
		{code: "K4KA", block: true},
		{code: "GAAN", block: false},
		{code: "7K3M", block: false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := blocked.Blocks(tt.code); got != tt.block {
				t.Errorf("Blocks(%q) = %v, want %v", tt.code, got, tt.block)
			}
		})
	}

	// Extra words are added to the built in list, never instead of it.
	if !blocked.Blocks("5H1T") {
		t.Error("extra words replaced the built in list instead of extending it")
	}
}

func TestBlocklistIgnoresWordsItCanNeverMatch(t *testing.T) {
	// A pickup code is four characters, so a longer word can never appear in
	// one. Keeping it would cost a comparison per draw forever and block
	// nothing.
	//
	// A word written in leetspeak goes the same way, which is worth knowing
	// before extending the list: matching happens against readings, and a
	// reading is always letters. A venue that wants K4KA blocked adds KAKA.
	blocked := NewBlocklist("verylongword", "", "   ", "k4ka", "wtf!")

	if got, want := len(blocked.words), len(NewBlocklist().words); got != want {
		t.Errorf("blocklist holds %d words, want %d: unusable words were kept", got, want)
	}
}

func TestBuiltInBlocklistIsWellFormed(t *testing.T) {
	for _, word := range NewBlocklist().words {
		if len(word) > PickupCodeLength {
			t.Errorf("%q is longer than a pickup code, so it can never match", word)
		}
		if word == "" {
			t.Error("the blocklist holds an empty word, which would block every code")
		}
		if word != strings.ToUpper(word) {
			t.Errorf("%q is not uppercase, so it can never match a reading", word)
		}
		for i := range len(word) {
			if word[i] < 'A' || word[i] > 'Z' {
				t.Errorf("%q contains %q; blocked words are letters, since readings are letters", word, string(word[i]))
			}
		}
	}
}
