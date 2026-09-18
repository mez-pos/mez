// Package tax computes tax on a bill. Tax is pluggable from the first day:
// a TaxProfile decides how tax is computed, what an invoice must print, and
// what the tax register needs, and nothing outside a profile knows which
// regime a venue is under. docs/02-domain.md is the rule.
//
// Three profiles ship: GST for India, VAT for the UK and EU, and US sales
// tax. They share one engine, which rounds each line half to even and then
// reconciles each rate's total at the order level, so the lines always add
// up to the rate-wise summary and an inclusive price is never a paisa out.
//
// Rates are in thousandths of a percent, not the basis points money uses:
// see Rate and ADR-049. Nothing here is a float.
package tax
