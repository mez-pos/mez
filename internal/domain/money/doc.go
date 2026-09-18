// Package money is the amount type the rest of the system counts in.
//
// An Amount is a whole number of a currency's minor units: paise, cents, and
// nothing smaller. There is no float in this package and there must never be
// one, because a float cannot hold 0.1 exactly and a till that is a paisa out
// at close is a real problem for a real cafe. docs/02-domain.md is the rule;
// this is the implementation of it.
//
// # Every operation can fail
//
// Add, Sub, Mul, Neg and Compare all return an error. That is more typing than
// a+b, and it buys one guarantee worth the cost: nothing in this package can
// quietly produce a wrong number. Mixed currencies fail, and results that do
// not fit in an int64 fail, rather than wrapping around into a total that
// looks plausible.
//
// # The zero value is not an identity
//
// Amount{} has no currency, so adding it to rupees is a mismatch, not a no-op.
// A zero-valued Amount in a total almost always means a field was never
// filled in, and treating it as "zero of whatever you like" is exactly how a
// dollar line ends up inside a rupee total. Start a running total with
// Sum(currency) or New(0, currency) instead.
//
// # Rounding
//
// Rounding is half to even, and it happens once, at the end. Mul rounds the
// product of an amount and a basis-point rate; RoundToNearest applies a
// venue's cash-rounding step. Both are integer arithmetic throughout. The
// per-venue rounding setting from docs/02-domain.md is not read here: this
// package provides the operation, and a later phase decides when to call it.
//
// # Currencies
//
// A currency is three uppercase ASCII letters, checked for shape but not
// against a list of the world's currencies. Maintaining that list would be a
// standing chore and would reject a currency the day it is introduced. What is
// kept is a short table of the currencies whose minor unit is not one
// hundredth, so that a yen is never printed as 12.34 when it means 1234.
//
// # Splitting
//
// SplitEvenly is the split-payment invariant in code: the parts sum to exactly
// the total, deterministically, every time. The extra minor unit goes to the
// earliest parts.
package money
