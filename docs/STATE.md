# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 03 - Domain core: order events, projections, invoice series, tax**
File: `docs/phases/phase-03.md`. Phase 02 is complete.

## Last completed task

**Phase 03, Task 1 - event types.** `internal/domain/order` has all 15 events
from `docs/02-domain.md` as payload types, an `Event` envelope (`Seq`, `Type`,
`At`, `ActorID`, `DeviceID`, `Payload`) and its JSON form. `NewEvent` takes the
type from the payload and normalises `At` to UTC. 100% coverage. ADR-045.

What Task 2 must know: a line's ID and a payment's ID are the `Seq` of the
event that created them. An unknown event type fails decoding with
`*UnknownEventTypeError`, and a stream with one fails whole. Unknown fields
are ignored, so anything that changes a total must be a new event type.
Payloads are values; no validation beyond shape happens here (qty > 0, a void
of a missing line, and so on belong to the projection).

## Next task

**Phase 03, Task 2 - the projection.** `Project(events []Event) (State,
error)`, pure. Read `docs/02-domain.md` in full first. The phase notes ask the
human to review the event list and projection before Task 3.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the real server on `:7777`, writing to the default
  `data_dir`. See `docs/running.md`. `make migrate` needs the goose CLI; the
  server migrates itself on startup, so it is only for a database with no
  server.
- `go test -race` needs cgo, which is banned, so `make check` runs without it.
  Now racing untested: the serve goroutine, the log mutex, the health prober.
- Every binary ships placeholder frontends. Phase 05 must add `-tags frontend`
  to `make dist`; it fails today because `web/*/dist` does not exist.
- `make build` writes `bin/nofiltr` with no `.exe`, so a stray server on Windows
  is killed with `taskkill /F /IM nofiltr`, without the extension.
- The domain import test checks direct imports only. See the cost note on
  ADR-016.
- `make generate` needs sqlc installed: `go install
  github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`. The generated code is committed,
  so only someone editing SQL needs it.
- `docs/02-domain.md` asks for a lint rule that fails the build on `float64` in
  `internal/domain`. No phase task owns it and it does not exist yet.
- `line.added` has no tax snapshot yet. Task 5 decides its shape; until then
  `invoice.issued.tax_snapshot` is raw JSON. No events are persisted, so
  adding it then is free.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [ ] 03 — Domain core: order events, projections, invoice series, tax
- [ ] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [ ] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [ ] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
