package id

import (
	"crypto/rand"
	"fmt"
	"io"
)

// Generator draws codes from an entropy source.
//
// The source is injected for the same reason the clock is in the rest of
// internal/domain, per docs/03-conventions.md: a package that reaches for
// randomness on its own cannot be tested for what it does with a given draw.
// The package-level NewPublic and NewPickupCode use crypto/rand, which is what
// production wants; tests construct a Generator over a fixed sequence.
type Generator struct {
	src io.Reader
}

// NewGenerator returns a generator reading from src. A nil src means
// crypto/rand, the only source that belongs in production: a public ID that
// can be guessed is a way to read somebody else's order.
func NewGenerator(src io.Reader) *Generator {
	if src == nil {
		src = rand.Reader
	}
	return &Generator{src: src}
}

// symbols draws n symbols from the alphabet, one per byte of entropy.
//
// Three of every eight bits are discarded. That is wasteful and it is the
// right trade: taking five bits at a time would mean stitching bits across
// byte boundaries for no benefit a till will ever notice.
func (g *Generator) symbols(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(g.src, buf); err != nil {
		return "", fmt.Errorf("id: drawing %d symbols: %w: %w", n, ErrEntropy, err)
	}

	for i, b := range buf {
		buf[i] = Alphabet[b&symbolMask]
	}
	return string(buf), nil
}
