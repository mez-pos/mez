# Contributing to mez

Thank you for looking. mez is a point of sale: a bug here can charge a guest
twice, skip an invoice number, or lose a night's takings. So the bar is
correctness first, and the rules below exist because each one has a real cafe
on the other side of it.

- [Before you start](#before-you-start)
- [Set up](#set-up)
- [Run it while you work](#run-it-while-you-work)
- [Where things are](#where-things-are)
- [The rules that are never broken](#the-rules-that-are-never-broken)
- [Writing code](#writing-code)
- [Tests](#tests)
- [Changing the database](#changing-the-database)
- [Changing the API](#changing-the-api)
- [Recording a decision](#recording-a-decision)
- [Sending a change](#sending-a-change)
- [Reporting a bug or a security issue](#reporting-a-bug-or-a-security-issue)

## Before you start

- **Small fixes** (a bug, a typo, a test, a doc): open a pull request.
- **Anything larger** (a feature, a new dependency, a change of approach):
  open an issue first and say what you want to do. The build follows a
  phased plan in [`docs/phases/`](docs/phases/), and
  [`docs/STATE.md`](docs/STATE.md) says where it is; work that fits the plan
  is far more likely to be merged.
- **The stack is settled**: Go, SQLite, sqlc, goose, `net/http`, React,
  TypeScript, Vite, Tailwind. If you think a choice is wrong, make the case in
  an issue rather than in a pull request that switches it.
- **Out of scope until phase 10+ is done**: loyalty, gift cards, purchasing,
  timesheets, memberships, delivery aggregators.

By contributing you agree that your contribution is licensed under the
project's licence, AGPL-3.0 (see [LICENSE](LICENSE)).

## Set up

You need:

| Tool | Version | For |
|---|---|---|
| Go | 1.23 or newer | The server |
| Node | 22.12 or newer | The three web apps |
| GNU Make and a POSIX shell | any | The build. On Windows: Git Bash plus GNU Make, or WSL |
| sqlc | 1.31 | Only if you change SQL (`make generate`) |

```sh
git clone https://github.com/byjoellobo/mez.git
cd mez
make check        # gofmt, go vet, Go tests, TypeScript typecheck, build bin/mez
make build-web    # Prettier check, vitest, production builds, guest size budget
```

Both must pass before you start, and before you send anything. CI runs exactly
these two.

| Target | Does |
|---|---|
| `make check` | The Go gate: fmt, vet, test, typecheck, build |
| `make build-web` | The web gate: format check, tests, build, the guest's 60 KB budget |
| `make fmt-web` | Formats the TypeScript with Prettier |
| `make generate` | Regenerates sqlc code and `web/shared/api-types.ts` |
| `make migrate-new NAME=...` | Scaffolds the next migration |
| `make run` | Runs the server |
| `make dev` | Dev venue, server and the web apps with hot reload |
| `make dist` | Release binaries for Windows, Linux and macOS |
| `make help` | Lists everything |

## Run it while you work

`make dev` does the rest of this section in one go. It builds the binary,
makes a demo cafe in `.dev-data` the first time (with a pairing code for the
till, the back office and the kitchen display, and the demo staff's PINs),
starts the server on `:7791`, and starts the terminal and back office under
Vite with hot reload. Ctrl+C stops all three. `DEV_DATA` and `DEV_PORT` change
the directory and the port; delete `.dev-data` to start the venue again. A
fresh code for a browser that is not paired yet is
`bin/mez pair -data-dir .dev-data -device-kind terminal`.

To do it by hand, create a throwaway venue with a sample menu, pairing the
back office first, then start the server:

```sh
go run ./cmd/mez setup -data-dir ./data -name "Dev Cafe" -owner Dev -owner-pin 1234 \
   -timezone Asia/Kolkata -device-kind back_office -device-name "Dev office" -demo-menu
MEZ_DATA_DIR=./data make run
```

`./data` is git-ignored. Delete it to start again.

For the web apps, run their dev servers beside it. They proxy `/api` and the
live stream to the server on `:7777`, so the code is the same as in
production:

```sh
cd web
npm run dev -w backoffice   # http://localhost:5174/backoffice/
npm run dev -w terminal     # http://localhost:5173/
npm run dev -w guest        # http://localhost:5175/order/
```

Enter the pairing code `setup` printed in the back office, sign in with PIN
`1234`, and pair a till from **Devices**. [`web/README.md`](web/README.md) has
more on the three apps.

A receipt printer is not needed: `./mez --test-print <ip> -save test.bin`
writes the bytes to a file, and the receipt tests decode the layout without
one.

## Where things are

```
cmd/mez/              the binary: serve, setup, pair, --test-print
internal/
  domain/             pure business rules, no IO: money, order events and
                      projection, invoice series, tax, shifts, stock, tips...
  service/            application services, one package per capability:
                      order, menus, shift, stock, settings, reports...
  api/                HTTP handlers: decode, call one service method, encode
  db/                 SQLite, migrations (goose), queries (sqlc) and gen/
  auth/               devices, pairing, staff PINs, roles and permissions
  hub/                the live WebSocket stream to every device
  device/             ESC/POS printing and the durable print queue
  config/ logging/    settings from file, env and flags; structured logs
  codegen/            Go structs to TypeScript types
web/
  terminal/           the till and the kitchen display (React)
  backoffice/         the owner's back office (React)
  guest/              guest ordering, Preact, 60 KB budget
  shared/             UI primitives, API client, generated api-types.ts
build/                Dockerfile and compose
docs/                 product, architecture, domain, conventions, ADRs, plan
```

`internal/domain` imports nothing from `db` or `api`, and a test fails the
build if it does.

## The rules that are never broken

These are the invariants from [`CLAUDE.md`](CLAUDE.md). A change that breaks
one is not merged, however good the rest is.

1. **Money is `int64` minor units** (paise, cents). Never a float, never a
   decimal string. Only rendering converts.
2. **Orders are append-only events.** The current state is a projection. To
   change an order line, append a corrective event; never `UPDATE` it.
3. **No ORM.** SQL is written by hand and compiled with sqlc.
4. **No cgo.** The build cross-compiles with `CGO_ENABLED=0`.
5. **No external services.** No Postgres, Redis or Docker needed to run.
6. **Offline is normal.** Nothing in the till, kitchen display or back office
   may need the internet.
7. **Writes that matter are transactions.** Invoice numbers, payments and shift
   closes above all.
8. **The guest app stays tiny.** 60 KB gzipped, no shadcn, no heavy
   dependencies. `make build-web` fails over budget or if React's own runtime
   gets in.

## Writing code

[`docs/03-conventions.md`](docs/03-conventions.md) is the full guide. In
short:

**Go**
- `gofmt` clean; `go vet` clean.
- Wrap errors with context: `fmt.Errorf("reading menu %d: %w", id, err)`.
  Domain errors are sentinel values, so handlers can map them to statuses.
- `ctx context.Context` first in anything that does IO.
- `log/slog`, structured. No `fmt.Println`.
- No `time.Now()` in `internal/domain`: inject the clock.
- No naked goroutines in handlers; a goroutine has an owner with `Close()`.
- No `GetX` names: `X()` or `FindX`. Constructors are `New...`.
- No `panic` in a request path.

**TypeScript**
- Strict, no `any`. Prettier-formatted (`make fmt-web`).
- Server state through React Query only; local state with `useState`.
- Tailwind with the design tokens only; no raw hex colours.
- API types come from `web/shared/api-types.ts`, which is generated. Never
  hand-write a type that mirrors a Go struct.
- Components `PascalCase.tsx`, everything else `kebab-case.ts`.

**Screens** (the terminal especially, see
[`docs/design/tokens.md`](docs/design/tokens.md)): nothing tappable under
44px; numbers monospaced and right-aligned; status is colour plus a word; one
accent colour, for money and action; no hover-only or drag-only interactions.

**Size and shape.** Files stay under about 600 lines and functions under about
50; split by capability, not by layer. If you write the same block a third
time, extract it. Prefer clarity to cleverness: a cafe owner's nephew may be
the one reading it.

## Tests

- **Domain code ships with tests in the same change.** Money, invoice series,
  split payments and the order projection are where a bug costs a venue real
  money; they are covered close to completely.
- Go tests are table-driven: `tests := []struct{ name string; ... }`.
- **No mocks against SQLite.** Use a real database in a temp file; the helpers
  in the existing tests show how.
- Web tests (vitest) cover logic, not DOM snapshots. Put a screen's decisions
  in a `*-logic.ts` file and test that.
- Tests run in the `Asia/Kolkata` timezone (`web/vitest.config.ts`), so a date
  test gives the same answer on every machine.
- A screen change is tried in a browser before it is called done. Say in the
  pull request what you tried.

```sh
go test ./internal/domain/...        # one area
go test -run TestInvoice ./...       # one test
cd web && npx vitest run backoffice  # one app's web tests
```

## Changing the database

1. `make migrate-new NAME=add_something` creates the next numbered file in
   `internal/db/migrations/`.
2. Write it forward-only. **Never edit a migration that has been merged**;
   add another.
3. Declare the type you mean (`TIMESTAMP`, `BOOLEAN`, `INTEGER`, `TEXT`).
   Money columns carry `CHECK (typeof(x) = 'integer')`; booleans are 0 or 1
   with a check; timestamps are UTC with no SQL default.
4. Queries go in `internal/db/queries/<table>.sql` with explicit columns.
   `SELECT *` and `RETURNING *` are banned.
5. `make generate`, and commit the regenerated `internal/db/gen/` with it.

Reads go through `database.Queries()`; writes through
`database.WriteTx(ctx, func(q *gen.Queries) error { ... })`.

## Changing the API

- Routes are `/api/v1/...`, JSON with snake_case fields.
- Handlers decode, validate, call one service method and encode. No business
  logic in a handler.
- Errors are `{"error": {"code", "message", "field"}}`; `code` is stable.
- After changing a request or response struct, run `make generate` and commit
  `web/shared/api-types.ts` with it. `make check` fails if they disagree.

## Recording a decision

If you choose something a later reader would ask "why?" about, add an ADR to
the end of [`docs/DECISIONS.md`](docs/DECISIONS.md), numbered after the last:

```
## ADR-NNN - A title that states the decision
YYYY-MM · Status: accepted.
Decision: what was decided, concretely.
Because: the reason, and what it was weighed against.
Cost: what this gives up, or what is now known to be missing.
---
```

## Sending a change

1. Branch from `main`.
2. Keep it focused: one change, its tests, its docs.
3. Run `make check` and `make build-web`. Both pass.
4. Commit messages:
   - work on a planned task: `phase-11/task-2: short description`;
   - anything else, conventional commits: `fix: ...`, `feat: ...`,
     `refactor: ...`, `docs: ...`, `test: ...`, `style: ...`, `ci: ...`,
     `chore: ...`.
5. Open a pull request that says what changed, why, and how you tested it
   (which tests, and what you tried in a browser or on a printer).

Before asking for review, check:

- [ ] `make check` and `make build-web` pass
- [ ] New domain logic has tests
- [ ] No float touches money; no order row is updated in place
- [ ] A migration, if any, is new rather than an edit
- [ ] `make generate` output is committed if SQL or API structs changed
- [ ] A non-obvious choice has an ADR
- [ ] `docs/STATE.md` notes anything left broken or untried

## Reporting a bug or a security issue

**A bug:** open an issue with what you did, what you expected, what happened,
the version (`version` and `commit` from `/api/v1/health`, also on the first
line the server logs) and the
relevant lines from `logs/mez.log` in the data directory. For a printing bug,
add the printer model and paper width.

**A security issue:** do not open a public issue. Report it privately through
the repository's **Security** tab ("Report a vulnerability"), with the steps
to reproduce it. You will get an answer before anything is disclosed.
