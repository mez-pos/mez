package tax

import "errors"

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrInvalidRate means a rate is below 0% or above 100%, or cannot be
	// split the way the profile needs.
	ErrInvalidRate = errors.New("invalid tax rate")

	// ErrInvalidLine means a line has a negative amount, is in the wrong
	// currency, or lacks something its profile needs.
	ErrInvalidLine = errors.New("invalid tax line")

	// ErrMissingHSN means a GST line has no valid HSN or SAC code. GST
	// invoices must print one per line.
	ErrMissingHSN = errors.New("line has no valid HSN or SAC code")

	// ErrInvalidContext means the venue context is missing what the profile
	// needs: a GSTIN, a state code, a VAT number, a jurisdiction.
	ErrInvalidContext = errors.New("invalid venue tax context")

	// ErrPricingNotSupported means the profile cannot price that way. US
	// sales tax is always added at the till.
	ErrPricingNotSupported = errors.New("pricing mode not supported by this tax profile")
)
