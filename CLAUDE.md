# Nofiltr POS — Agent Instructions

> Product name: **Nofiltr POS**. Binary: `nofiltr`.
> Go module path: `github.com/byjoellobo/nofiltr-pos` (set in `go.mod`).

You are building an open-source, self-hostable Point of Sale system for cafes,
bars and small restaurants. It runs as a single binary on the venue's own
computer and must keep selling with no internet connection.

## Session protocol — follow this exactly

1. Read `docs/STATE.md`. It names the current phase and the next task.
2. Read `docs/phases/phase-NN.md` for that phase. **Read only that phase file.**
3. Do **one task**. The next unchecked `[ ]` item. Do not start the one after it.
4. Run `make check` (fmt, vet, test, build). It must pass.
5. Update `docs/STATE.md`: tick the task, set the next one, note anything broken.
6. Append to `docs/DECISIONS.md` if you made a non-obvious choice.
7. Commit: `phase-N/task-M: short description`.
8. Stop. End the session.

If a task turns out to be larger than one session: **do not push through.**
Split it into sub-tasks inside the phase file, commit that edit, and stop.

## What to read, and what not to

| Read | When |
|---|---|
| `docs/STATE.md` | Every session, first |
| `docs/phases/phase-NN.md` | The current phase only |
| `docs/02-domain.md` | Any task touching money, orders, invoices, tax |
| `docs/03-conventions.md` | Any task writing code |
| `docs/design/tokens.md` | Any task writing UI |
| `docs/design/screens/<screen>.md` | Only the screen you are building |
| `docs/design/reference/` | Only when a task explicitly says to |

Do **not** read all phase files. Do **not** read the whole `design/reference`
archive to "get context". Context spent on files you don't need is the main way
this build fails.

## Invariants — never violate these

- **Money is `int64` minor units** (paise / cents). Never float. Never decimal
  strings. A rendering helper converts for display only.
- **Orders are append-only events.** Current state is a projection. Never
  `UPDATE` an order line to change it — append a corrective event.
- **No ORM.** Hand-written SQL through `sqlc`. No GORM, no ent.
- **No cgo.** Use `modernc.org/sqlite`. The build must cross-compile with
  `CGO_ENABLED=0`.
- **Boots with zero external services.** No Postgres, no Redis, no Docker
  required. `./nofiltr` on a bare machine must work.
- **Offline is the default state, not an error path.** Nothing in the terminal,
  kitchen display or back office may require the internet.
- **Every DB write that matters is in a transaction.** Invoice numbers,
  payments and shift closes especially.
- **The guest app is a separate, tiny bundle.** It loads on 3G. No shadcn, no
  heavy dependencies, hard budget of 60 KB gzipped.

## Stack — settled, do not relitigate

Go 1.23+ · SQLite (WAL, `modernc.org/sqlite`) · sqlc · goose · stdlib `net/http`
· `coder/websocket` · React + TypeScript + Vite + Tailwind · `embed.FS` ·
ESC/POS over TCP:9100 · Litestream for backups.

Rationale is in `docs/01-architecture.md`. If you believe a choice is wrong,
write the case in `docs/DECISIONS.md` and stop — do not switch unilaterally.

## Scope discipline

The v1 spine is: **register → order → kitchen display → payment → invoice →
Z-report.** Loyalty, gift cards, purchasing, timesheets, memberships and
aggregator channels are out of scope until phase 10+. If a task tempts you into
one of them, don't.

## Tests

Domain code (`internal/domain/...`) ships with tests in the same task. Money,
invoice series, split payment and order projection logic are the places a bug
costs a real cafe real money. Table-driven tests, no mocks against SQLite —
use a temp file DB.
