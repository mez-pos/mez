package id

import (
	"slices"
	"strings"
)

// blockedWords is the built-in English list a pickup code is checked against.
//
// Every entry is uppercase, letters only, and no longer than a pickup code,
// because anything else can never match. Words needing a U are absent for the
// same reason: the alphabet has no U and no digit reads as one, so it cannot
// spell them.
//
// The list is deliberately short and covers what a venue would be embarrassed
// to have called across a counter: obscenity and slurs. It is not a general
// content filter. A venue that needs more, or needs another language, passes
// extra words to NewBlocklist.
var blockedWords = []string{
	"ANAL", "ARSE", "ASS", "COCK", "COON", "CRAP", "DAMN", "DICK", "DYKE",
	"FAG", "FART", "GOOK", "HELL", "HOMO", "JAP", "JIZZ", "KIKE", "KKK",
	"NAZI", "PAKI", "PEDO", "PISS", "PORN", "RAPE", "SEX", "SHAG", "SHIT",
	"SLAG", "SPIC", "TIT", "TURD", "TWAT", "WANK", "WOG",
}

// lookalikes maps each digit to the letters it is read as. This is the same
// confusion the alphabet was designed around, used in the other direction: a
// code is filtered on how it can be read, not on the characters it holds.
var lookalikes = map[byte][]byte{
	'0': {'O'},
	'1': {'I', 'L'},
	'2': {'Z'},
	'3': {'E'},
	'4': {'A'},
	'5': {'S'},
	'6': {'G'},
	'7': {'T'},
	'8': {'B'},
	'9': {'G'},
}

// maxReadings caps the expansion in readings. A four character code produces
// at most sixteen, so the cap is unreachable in use and exists only so that a
// long string handed to Blocks cannot cost an unbounded amount of work.
const maxReadings = 64

// Blocklist decides whether a pickup code is one to keep.
type Blocklist struct {
	words []string
}

// NewBlocklist returns the built-in English list plus any extra words, which
// is where a venue puts the ones its own language needs. Called with no
// arguments it is the default list.
//
// Extra words are uppercased and trimmed. Words that could never match a
// pickup code are dropped rather than kept and compared forever: anything
// longer than a code, and anything that is not plain letters, since a reading
// is always letters.
func NewBlocklist(extra ...string) *Blocklist {
	words := make([]string, 0, len(blockedWords)+len(extra))
	words = append(words, blockedWords...)

	for _, word := range extra {
		word = strings.ToUpper(strings.TrimSpace(word))
		if usableWord(word) && !slices.Contains(words, word) {
			words = append(words, word)
		}
	}

	return &Blocklist{words: words}
}

// Blocks reports whether any way of reading the code contains a blocked word.
//
// It matches anywhere in the code, not just the whole of it. A code is heard
// as one sound when it is called out, and nobody hears where the word stops.
func (b *Blocklist) Blocks(code string) bool {
	for _, reading := range readings(strings.ToUpper(code)) {
		for _, word := range b.words {
			if strings.Contains(reading, word) {
				return true
			}
		}
	}
	return false
}

// readings returns every way the code can be read aloud, with each digit
// replaced by the letters it resembles. 5H1T reads as SHIT and SHLT, and one
// of those is enough to redraw.
func readings(code string) []string {
	out := []string{""}

	for i := range len(code) {
		options, ok := lookalikes[code[i]]
		if !ok {
			options = []byte{code[i]}
		}

		next := make([]string, 0, len(out)*len(options))
		for _, prefix := range out {
			for _, option := range options {
				if len(next) >= maxReadings {
					break
				}
				next = append(next, prefix+string(option))
			}
		}
		out = next
	}

	return out
}

// usableWord reports whether a word could ever appear in a reading of a pickup
// code: uppercase letters only, at least one of them, and no longer than a
// code.
func usableWord(word string) bool {
	if word == "" || len(word) > PickupCodeLength {
		return false
	}
	for i := range len(word) {
		if word[i] < 'A' || word[i] > 'Z' {
			return false
		}
	}
	return true
}
