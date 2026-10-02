package invoice

import (
	"errors"
	"fmt"
)

// ErrNumbering means a venue's invoice and credit note series, each fine on
// its own, would still issue numbers that repeat or run together.
var ErrNumbering = errors.New("invoice numbering would repeat a number")

// NumberingError is the refusal of CheckNumbering, and the part of the
// configuration it is about: "template", "prefix", "series", "credit_prefix"
// or "credit_series".
type NumberingError struct {
	Field  string
	Reason string
}

func (e *NumberingError) Error() string { return fmt.Sprintf("%v: %s", ErrNumbering, e.Reason) }

// Unwrap lets errors.Is find ErrNumbering.
func (e *NumberingError) Unwrap() error { return ErrNumbering }

// CheckNumbering checks the venue's invoice series and its credit note
// series together, against the three ways a configuration that parses can
// still issue the same number twice:
//
//   - A template with no financial year restarts at 1 every year, and the
//     second year repeats the first's numbers. Counters restart per
//     financial year, so the year must be in the number.
//   - A template with no prefix writes the same number for an invoice and a
//     credit note with the same sequence, the two being counted apart.
//   - The same prefix on both does the same.
//
// Both series share one template (ADR-083), so each rule is checked once.
func CheckNumbering(invoices, credits Series) error {
	switch {
	case invoices.Code == credits.Code:
		return &NumberingError{"credit_series", fmt.Sprintf("credit notes and invoices both use series %q", invoices.Code)}
	case !invoices.Template.UsesFY():
		return &NumberingError{"template", "the number must include the financial year, {fy} or {fy:short}: numbering restarts every year, and the same number would come round again"}
	case !invoices.Template.UsesPrefix():
		return &NumberingError{"template", "the number must include {prefix}, so an invoice and a credit note never share a number"}
	case invoices.Prefix == credits.Prefix:
		return &NumberingError{"credit_prefix", fmt.Sprintf("invoices and credit notes both use the prefix %q: an invoice and a credit note would share a number", invoices.Prefix)}
	}
	return nil
}
