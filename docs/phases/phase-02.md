# Phase 02 — Data foundation

## Goal
SQLite is wired, migrations run on startup, sqlc generates typed queries, and the
money and ID primitives exist with full tests.

## Depends on
Phase 01. Read `docs/02-domain.md` before starting.

## Tasks

- [ ] **Task 1 — The money package.**
  `internal/domain/money`. `Amount{Minor int64; Currency string}`. Add, Sub,
  Mul by basis points, Neg, IsZero, Compare, Parse, String. Currency mismatch
  returns a typed error. Banker's rounding helper. `SplitEvenly(total, n)`
  returning parts that sum exactly to the total by largest remainder.
  JSON marshals as `{"minor":123400,"currency":"INR"}` — never as a float.
  Adversarial tests: zero, negative, mismatch, rounding boundaries, a
  three-way split of a prime number of paise.

- [ ] **Task 2 — The ID package.**
  `internal/domain/id`. `int64` internal keys are just the DB's. Public IDs:
  Crockford base-32 without I, L, O, U. `NewPublic(prefix)` →`ord_7K3M9QX2`.
  `NewPickupCode()` → 4 chars, filtered against a profanity list (English +
  a configurable extra list). Decoding tolerates case and the common
  substitutions (0/O, 1/I/L). Tests including collision sanity over 1e6 draws.

- [ ] **Task 3 — Database connection.**
  `internal/db`. Open with `modernc.org/sqlite`. Set WAL, `foreign_keys=ON`,
  `busy_timeout=5000`, `synchronous=NORMAL`. A single writer connection and a
  read pool. `Tx(ctx, fn)` helper with deferred rollback and the correct error
  wrapping. Health check that reports db status into `/api/v1/health`.

- [ ] **Task 4 — Migrations.**
  goose, migrations in `internal/db/migrations/*.sql`, embedded. Run
  automatically on startup, logged, and fail loudly on error. First migration:
  `venues`, `settings`, `devices`, `staff`, `roles`. Use the money convention:
  every monetary column is `INTEGER` minor units with a sibling currency column
  or a venue-level currency. A `make migrate-new NAME=x` target.

- [ ] **Task 5 — sqlc wired.**
  `sqlc.yaml` generating into `internal/db/gen`. Write the first queries for
  venues, settings and staff. `make generate` regenerates. An integration test
  against a temp file DB that migrates up, inserts a venue and reads it back.
  Document the add-a-query loop in `docs/03-conventions.md` if anything is
  non-obvious.

## Done when
A temp DB migrates, sqlc-generated code compiles and round-trips a venue, and
the money package has adversarial tests passing. `make check` passes.

## Notes
The money package is load-bearing for the entire product. Take the whole session
on Task 1 if needed.
