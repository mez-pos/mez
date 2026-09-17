# Architecture

## The shape

```
                          ┌──────────────────────────────┐
                          │  relay.<domain>  (optional)  │
                          │  stateless, routes by venue  │
                          └──────────────┬───────────────┘
                                         │  outbound WSS, initiated by venue
                                         │
┌────────────────────────────────────────┴─────────────────────────────┐
│  VENUE PC  —  nofiltr.exe  (Windows Service)                         │
│                                                                      │
│   HTTP + WebSocket on :7777        SQLite (WAL)  ──> Litestream ──>  │
│   embedded SPA bundles             the source of truth       backup  │
│   ESC/POS print queue                                                │
└───────┬───────────────┬───────────────┬──────────────────────────────┘
        │ LAN           │ LAN           │ TCP:9100
   Terminal         Kitchen         Thermal printers
   (kiosk browser)  display          + cash drawer
```

## The one rule everything follows

**The venue's machine owns the data.** Every other device is a client. There is
no sync engine, no CRDT, no conflict resolution, because there is only ever one
writer. Offline is the default state, not a degraded mode.

The consequence: `nofiltr.exe` on a counter PC and the same binary in Docker on
a VPS are the same artifact. Windows install and self-host are not two products.

## Stack

| Layer | Choice |
|---|---|
| Server | Go 1.23+, single static binary, `CGO_ENABLED=0` |
| Database | SQLite, WAL mode, `modernc.org/sqlite` |
| Queries | `sqlc` — SQL in, type-safe Go out |
| Migrations | `goose`, embedded via `embed.FS` |
| HTTP | stdlib `net/http` + `ServeMux` (Go 1.22 routing) |
| Realtime | `github.com/coder/websocket` |
| Frontend | React 18 + TypeScript + Vite + Tailwind |
| UI shell | Edge/Chrome `--app= --kiosk` (ADR-007) |
| Types | Go structs → TypeScript, generated at build |
| Printing | ESC/POS over TCP:9100; Windows RAW spooler fallback |
| Backup | Litestream → S3 / R2 / local folder |
| Service | `kardianos/service` for Windows Service registration |
| Installer | Inno Setup |

## Process layout

One process. Inside it:

- **HTTP server** — API, static SPA bundles, WebSocket upgrade
- **Hub** — fan-out of domain events to subscribed devices
- **Print queue worker** — durable, retries, survives restart
- **Tunnel client** — outbound connection to the relay, reconnects with backoff
- **Scheduler** — nightly rollups, Litestream health, log rotation

No external message broker. No Redis. Goroutines and channels are enough at this
scale, and every extra service is a support ticket from a cafe owner.

## Repository layout

```
/cmd
  nofiltr/          server entrypoint
  relay/            the optional cloud relay
/internal
  config/           env + file config, sane defaults
  db/               sqlc output, migrations, connection setup
  domain/           PURE GO. no http, no sql. events, projections, money, tax
  api/              http handlers, request/response types
  hub/              websocket fan-out
  device/           escpos, drawer, printer discovery
  tunnel/           relay client
  service/          windows service wrapper
/web
  terminal/         staff SPA        (React, Tailwind, shadcn ok)
  backoffice/       owner SPA        (React, Tailwind, shadcn ok)
  guest/            guest SPA        (React, Tailwind, NO shadcn, <60KB gz)
/docs
/build              inno setup, docker, goreleaser
```

`internal/domain` must import nothing from `internal/db` or `internal/api`.
That boundary is what makes the money logic testable and reviewable.

## Three bundles, not one

The terminal and back office run on a machine in the building and can afford to
be fat. The guest app runs on a stranger's phone on 3G in a basement. Separate
Vite builds, separate budgets. Never let a shared component library drag shadcn
into the guest bundle.

## Realtime

WebSocket per device. On connect, the client sends its last seen event sequence
number; the server replays the gap from SQLite then streams live. This makes a
dropped wifi connection on a kitchen tablet a non-event.

Message envelope: `{ seq, type, venue_id, payload }`. Types mirror domain
events.

## The relay, in detail

The relay stores nothing. On startup the venue server dials
`wss://relay.<domain>/agent` with a venue key. The relay maps
`venue_id → open socket`. A guest hitting `order.<domain>/t/<table>` resolves
the venue, forwards the HTTP request as a framed message down the socket, and
streams the response back.

Self-hosters can run their own relay, or point `cloudflared` at their machine
and skip it entirely. This is deliberate — the POS must be fully functional with
zero dependency on any service you operate.

## Security posture

- LAN is treated as semi-trusted. Every device pairs once with a code, gets a
  long-lived device token, and every request is authenticated.
- Staff PINs are short by necessity. Rate-limit hard, lock after failures, and
  store as Argon2id — never as plain or fast hashes.
- The guest surface is anonymous and rate-limited per table.
- The back office is the only surface allowed to touch prices, tax settings or
  void history.

## What we are deliberately not doing

No microservices. No Kubernetes. No GraphQL. No ORM. No state management library
beyond React Query plus local component state. Every one of those would add
operational or cognitive cost that a cafe POS does not repay.
