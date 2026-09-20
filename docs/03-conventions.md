# Conventions

Read this before writing code. It exists so that a session in month four
produces code that looks like the session in week one.

## Go

**Layout.** Packages by capability, not by layer-within-feature. `internal/domain`
holds pure logic and imports nothing from `db` or `api`. Enforce with a test that
walks imports.

**Errors.** Wrap with `fmt.Errorf("doing x: %w", err)`. Domain errors are typed
sentinel values (`var ErrInvalidTransition = errors.New(...)`) so handlers can
map them to status codes. Never `panic` in request paths.

**Context.** Every function that touches IO takes `ctx context.Context` first.

**Logging.** `log/slog`, structured, to stdout. Fields: `venue_id`, `device_id`,
`order_id`, `actor_id` where relevant. No `fmt.Println` anywhere.

**Time.** Inject a clock (`type Clock interface{ Now() time.Time }`) into domain
code. Never call `time.Now()` inside `internal/domain`. Store UTC, render in the
venue's timezone.

**Concurrency.** Goroutines are owned by a struct with a `Close()`. No naked
`go func()` in handlers. Every long-running loop selects on `ctx.Done()`.

**Naming.** `GetX` is banned; use `X()` or `FindX`. Constructors are `New...`.
Test files are table-driven with a `tests := []struct{ name string; ... }`.

## SQL

- Queries live in `internal/db/queries/*.sql` with sqlc annotations.
- Explicit columns. `SELECT *` is banned, and so is `RETURNING *`: both change
  shape silently the next time a migration adds a column.

**Adding a query.** Write it in `internal/db/queries/<table>.sql` with a
`-- name: XByY :one|:many|:exec` annotation, run `make generate`, and commit
the regenerated `internal/db/gen`. The output is committed so that a clone
builds without sqlc installed. Then:

- Reads go through `database.Queries()`, which is bound to the read-only pool.
- Writes go through `database.WriteTx(ctx, func(q *gen.Queries) error {...})`,
  which runs them in one transaction.
- `database.Tx` and `database.Writer()` are for SQL sqlc cannot generate.
- Query names follow the Go rule: no `GetX`. `VenueByID`, `ListVenues`,
  `CreateStaff`, `ActiveStaffForVenue`.

A query naming a column that does not exist fails `make generate`, not a
request during service, because sqlc reads the migrations as its schema.
- Every multi-statement write is in an explicit transaction with a deferred
  rollback.
- Foreign keys on, `PRAGMA foreign_keys = ON` at connection setup.
- WAL mode, `busy_timeout=5000`, `synchronous=NORMAL`.
- Migrations are goose, numbered, forward-only in practice. Never edit a
  migration that has shipped — add a new one.
- `make migrate-new NAME=add_orders` scaffolds the next file. Numbering is
  sequential, not timestamped: one line of history, so the number is also the
  order it ran in. They are embedded, and the server applies them on every
  startup; `make migrate` is only for migrating a database without starting one.
- **Declare the type you want in Go.** SQLite ignores the declared type beyond
  affinity, but sqlc and the driver both read it, so it is the contract:
  `TIMESTAMP` gives `time.Time`, `BOOLEAN` gives `bool`, `INTEGER` gives
  `int64`, `TEXT` gives `string`. A timestamp declared `TEXT` cannot scan into
  a `time.Time` at all. See ADR-043.
- Timestamps are UTC, stored as `2026-09-18 04:05:06+00:00`, which is what the
  driver writes and what SQLite's `date()` and `datetime()` read. A `GLOB`
  check refuses anything else, a local offset included. No SQL defaults on
  them: the clock is injected in Go, and a column defaulting to `now()` goes
  around it.
- A time read back has a zero offset but no zone name, not `time.UTC`. Call
  `.UTC()` before rendering one, or it serialises as `+00:00` where the rest
  of the API says `Z`.
- Money columns carry `CHECK (typeof(x) = 'integer')`. INTEGER affinity is not
  a type — SQLite stores `12.5` in an INTEGER column as a REAL rather than
  refusing it, and the column then holds half a paisa forever.
- Booleans are `INTEGER` 0 or 1 with a check. SQLite will store the text
  `'false'`, which every truthiness test then reads as set.
- Anything with a public ID gets a unique index on it. See ADR-034.

## HTTP API

- `/api/v1/...`, JSON, snake_case fields.
- Handlers: decode → validate → call domain/service → encode. No business logic
  in a handler, ever.
- Errors: `{"error": {"code": "invalid_transition", "message": "...", "field": "..."}}`
  with a proper status code. `code` is stable and machine-readable.
- Every mutating endpoint accepts an `Idempotency-Key` header. A terminal on
  flaky wifi will retry a payment; charging twice is unacceptable.
- Lists are cursor-paginated. No offset pagination on order history.

## Frontend

- React function components, TypeScript strict, no `any`.
- **React Query** for all server state. No Redux, no Zustand, no context for
  data. Local UI state uses `useState`.
- Tailwind only, driving the CSS variables in `docs/design/tokens.md`. No
  arbitrary hex values in components — if a colour is missing, add a token.
- File naming: `PascalCase.tsx` for components, `kebab-case.ts` for everything
  else.
- API types are **generated**. Never hand-write a type that mirrors a Go
  struct. See "Generated API types" below.
- Terminal and back office may use shadcn/ui. The guest app may not — it has a
  60 KB gzipped budget.

## Terminal UI rules — these are hard constraints, not preferences

From the design system. A task that breaks one of these is not done.

- Nothing under **44px** is tappable. The charge button is **58px**.
- Every number is **monospaced and right-aligned**, so a column of rupees reads
  down.
- Status is **colour plus a word**. Never colour alone.
- **One accent.** Vermilion means money, action or now — nothing else.
- The guest never sees a screen the staff cannot explain.
- Assume gloves, glare and a queue. No hover-dependent interactions, no
  right-click menus, no drag as the only way to do something.

## Generated API types

`web/shared/api-types.ts` is generated from the Go request and response
structs, and committed. A frontend imports from it and never restates a
shape the server owns.

The loop:

1. Change a request or response struct in `internal/api`, a hub message in
   `internal/hub`, or a domain type they carry.
2. Run `make generate`. It rewrites `web/shared/api-types.ts`.
3. Commit the Go change and the generated file together.

`make check` runs the generator again in memory and fails if the committed
file is not what the structs produce, so a struct changed without step 2
fails the build rather than reaching a screen as a type that lies. The
generator is `internal/api/typescript_test.go` over `internal/codegen/ts`.

Two things it needs from you:

- **A vocabulary is a named Go type.** `Status order.Status` generates
  `status: OrderStatus`, a union of the real values; `Status string`
  generates `status: string`. Name the type, add it to the union list in the
  generator, and every client gets the check.
- **A new event type needs a line in `eventTypes`.** `order.Event` has a
  custom JSON shape, so its TypeScript union is assembled by hand from that
  list. A test compares the list with `order.Types()`, so a forgotten line
  fails rather than producing a union missing an arm.

## Testing

- `internal/domain`: near-total coverage. Money, projections, invoice series and
  split payment get adversarial cases — zero, negative, currency mismatch,
  rounding boundaries, concurrent allocation.
- `internal/db`: integration tests against a temp file SQLite. No mocks.
- `internal/api`: `httptest` round-trips on the happy path and each error shape.
- Frontend: Vitest for logic. No snapshot tests of whole screens — they rot.

## Commits and workflow

- `phase-N/task-M: imperative description`
- One task per commit. If a session produces two commits, the task was too big;
  say so in `STATE.md`.
- `make check` must pass before committing: `gofmt -l`, `go vet`, `go test ./...`,
  `tsc --noEmit`, `go build`.

## Dependencies

Adding a Go dependency requires a line in `DECISIONS.md`. The bar is high — every
dependency is a thing a cafe owner's antivirus might flag and a thing you must
maintain. Prefer stdlib. Current allowed set:

`modernc.org/sqlite`, `pressly/goose`, `coder/websocket`, `kardianos/service`,
`golang.org/x/crypto` (argon2), `google/uuid` (or a hand-rolled ID package).

## Things that are never acceptable

- A float in a money path.
- A `SELECT` that computes a historical total from current menu prices.
- `MAX(n)+1` for an invoice number.
- A feature that requires the internet to complete a sale.
- A dependency added without an ADR entry.
- A UI element under 44px that staff must hit during service.
