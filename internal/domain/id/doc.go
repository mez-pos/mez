// Package id makes the identifiers that leave the building.
//
// Internal primary keys are the database's own int64 autoincrements and are
// not this package's business. What is here is everything a person sees, says
// or types: order codes on a receipt, invoice references, device pairing
// codes, and the pickup code called across the counter.
//
// # The alphabet
//
// Crockford base-32: digits and uppercase letters, less I, L, O and U. Staff
// read these aloud over a grinder and type them back in with one hand, so
// Normalize resolves the confusions the alphabet exists to survive, accepting
// O for zero and I or L for one, in either case, with hyphens and spaces
// ignored. Normalize whatever arrives from outside before comparing it with
// anything stored.
//
// # Public IDs
//
// NewPublic("ord") gives ord_7K3M9QX2: a prefix saying what the thing is, and
// eight random symbols. Random rather than sequential, so that a customer
// holding one receipt cannot count to somebody else's order.
//
// Eight symbols is forty bits, which is not enough to be unique on its own.
// The unique index on the column is what makes it unique, with a redraw on
// conflict. ADR-034 has the arithmetic.
//
// # Pickup codes
//
// NewPickupCode gives four symbols, filtered so that what gets called across
// the counter is not a word anybody has to apologise for. The filter reads a
// code rather than matching it literally: 0 reads as O and 1 reads as I or L,
// so 5H1T and C0CK are both codes a naive generator would hand out. Pass extra
// words to NewBlocklist for the venue's own language.
//
// # Randomness
//
// The entropy source is injected, the way the clock is elsewhere in
// internal/domain: a package that reaches for randomness on its own cannot be
// tested for what it does with a given draw. NewPublic and NewPickupCode use
// crypto/rand, which is what production wants, and a Generator over a fixed
// sequence is what tests want.
package id
