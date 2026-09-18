// Package invoice is the invoice series: how numbers are formatted, which
// financial year a sale falls in, and the contract for handing numbers out.
//
// docs/02-domain.md is the rule. A series is monotonic and gapless, per
// venue, per series, per financial year. A number is never reused, never
// reassigned, and never issued for a sale that fails to commit. A gap is a
// compliance problem in most jurisdictions, and here it is a production
// incident.
//
// The domain cannot allocate a number by itself: that takes a row lock in
// the same transaction as the sale. So this package defines the Allocator
// contract and the format, the database layer implements the allocator, and
// invoicetest.Conformance is the suite that holds it to the contract. Audit
// states the guarantee as a checkable property.
//
// Financial years are judged in the venue's time zone, passed in, never
// read from a clock. ADR-048.
package invoice
