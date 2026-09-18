package id

import "fmt"

// PickupCodeLength is how many symbols a pickup code has. Four is short enough
// to call across a room and long enough that two open orders rarely clash.
const PickupCodeLength = 4

// pickupAttempts is how many blocked codes in a row are tolerated before the
// generator gives up. With the built-in list, a draw is rejected roughly once
// in ten thousand, so reaching this many in a row means the entropy source is
// broken, not that the venue is unlucky.
const pickupAttempts = 100

// NewPickupCode returns a four symbol code for calling an order out, using
// crypto/rand and the built-in blocklist.
//
// A venue with its own language passes extra words:
//
//	NewGenerator(nil).PickupCode(NewBlocklist(venueWords...))
//
// The code is not unique by itself. It only has to be unambiguous among the
// orders waiting at one counter, which is the caller's business: draw, try to
// claim it, draw again if it is taken.
func NewPickupCode() (string, error) {
	return NewGenerator(nil).PickupCode(NewBlocklist())
}

// PickupCode draws a code from this generator's entropy source, redrawing past
// anything the blocklist rejects.
//
// It returns an error rather than an unfiltered code when it cannot find one.
// A code that gets as far as being printed is a code that gets called out, and
// there is no recovering from that in a room full of customers.
func (g *Generator) PickupCode(blocked *Blocklist) (string, error) {
	for range pickupAttempts {
		code, err := g.symbols(PickupCodeLength)
		if err != nil {
			return "", err
		}
		if !blocked.Blocks(code) {
			return code, nil
		}
	}

	return "", fmt.Errorf("id: %d blocked codes in a row: %w", pickupAttempts, ErrEntropy)
}
