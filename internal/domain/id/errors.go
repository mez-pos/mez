package id

import "errors"

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrInvalidCode means a string was not a code in this alphabet: it was
	// empty, the wrong length, or held a character with no reading.
	ErrInvalidCode = errors.New("not a valid code")

	// ErrInvalidPrefix means a public ID prefix was not one to eight
	// lowercase ASCII letters.
	ErrInvalidPrefix = errors.New("prefix must be one to eight lowercase letters")

	// ErrEntropy means the randomness source failed, returned less than was
	// asked of it, or kept producing codes the blocklist rejects. It always
	// wraps the underlying cause when there was one.
	ErrEntropy = errors.New("could not draw a code")
)
