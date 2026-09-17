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
- Explicit columns. `SELECT *` is banned.
- Every multi-statement write is in an explicit transaction with a deferred
  rollback.
- Foreign keys on, `PRAGMA foreign_keys = ON` at connection setup.
- WAL mode, `busy_timeout=5000`, `synchronous=NORMAL`.
- Migrations are goose, numbered, forward-only in practice. Never edit a
  migration that has shipped — add a new one.

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
- API types are **generated**. Never hand-write a type that mirrors a Go struct.
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
