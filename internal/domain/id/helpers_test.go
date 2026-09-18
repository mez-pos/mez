package id

import (
	"errors"
	"io"
)

// repeatingReader is an entropy source that hands out a fixed byte pattern
// over and over. It makes generation deterministic, so a test can assert the
// exact code that a known set of bytes produces, and can force a generator to
// draw the same blocked pickup code every time.
type repeatingReader struct {
	pattern []byte
	next    int
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.pattern[r.next%len(r.pattern)]
		r.next++
	}
	return len(p), nil
}

// symbolBytes turns code characters into the bytes a repeatingReader must
// produce to generate them, so tests name the code they want rather than a
// list of magic numbers.
func symbolBytes(code string) []byte {
	out := make([]byte, len(code))
	for i := range len(code) {
		index := -1
		for j := range len(Alphabet) {
			if Alphabet[j] == code[i] {
				index = j
				break
			}
		}
		if index < 0 {
			panic("symbolBytes: " + string(code[i]) + " is not a code character")
		}
		out[i] = byte(index)
	}
	return out
}

// errReader is an entropy source that has failed, which is the one IO error
// this package has to survive.
type errReader struct{}

var errSourceFailed = errors.New("entropy source failed")

func (errReader) Read([]byte) (int, error) { return 0, errSourceFailed }

// shortReader returns fewer bytes than asked for and then stops, which is what
// a truncated or exhausted source looks like.
type shortReader struct{ left int }

func (r *shortReader) Read(p []byte) (int, error) {
	if r.left <= 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.left)
	for i := range n {
		p[i] = 0
	}
	r.left -= n
	return n, nil
}
