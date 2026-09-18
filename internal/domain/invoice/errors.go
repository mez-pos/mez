package invoice

import (
	"errors"
	"fmt"
)

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrInvalidFYRule means a financial-year start is not a real month, or
	// a day that every year has (1 to 28).
	ErrInvalidFYRule = errors.New("invalid financial-year rule")

	// ErrNoLocation means a financial year was asked for without the venue's
	// time zone. The boundary is local midnight, so UTC alone cannot say
	// which year a sale at 00:30 on 1 April belongs to.
	ErrNoLocation = errors.New("no venue time zone")

	// ErrInvalidTemplate means a number format is unparseable, names a token
	// that does not exist, or does not hold exactly one {seq}.
	ErrInvalidTemplate = errors.New("invalid invoice number template")

	// ErrInvalidSeries means a series configuration is incomplete or
	// inconsistent.
	ErrInvalidSeries = errors.New("invalid invoice series")

	// ErrTooLong means a formatted number is longer than the series allows.
	// GST caps an invoice number at 16 characters.
	ErrTooLong = errors.New("invoice number too long")

	// ErrInvalidKey means an allocation was asked for with no venue, no
	// series or no financial year.
	ErrInvalidKey = errors.New("invalid series key")

	// ErrInvalidSeq means an allocator returned a number below 1, which no
	// correct allocator can. The caller must roll back.
	ErrInvalidSeq = errors.New("allocator returned an invalid number")

	// ErrGap means a series is missing a number: a compliance problem, and
	// in this codebase a production incident.
	ErrGap = errors.New("gap in invoice series")

	// ErrDuplicate means a number was issued twice in one series and year.
	ErrDuplicate = errors.New("duplicate invoice number")
)

// AuditError reports the first fault Audit found: which series, and which
// number is missing or repeated.
type AuditError struct {
	Key Key
	Seq int64 // the missing or repeated number
	Err error // ErrGap or ErrDuplicate
}

func (e *AuditError) Error() string {
	return fmt.Sprintf("venue %d series %q FY %d: number %d: %v", e.Key.VenueID, e.Key.Series, e.Key.FYStart, e.Seq, e.Err)
}

// Unwrap lets errors.Is match ErrGap or ErrDuplicate.
func (e *AuditError) Unwrap() error { return e.Err }
