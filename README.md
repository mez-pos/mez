# Nofiltr POS

An open-source, self-hostable Point of Sale for cafes, bars and small
restaurants. One binary on the venue's own computer. Keeps selling with no
internet.

**One ticket, wherever it starts.** Counter, table QR, web, kiosk or aggregator —
the same ticket, the same kitchen display, the same invoice series, the same
night's Z-report.

Status: pre-alpha. Nothing is built yet.

## Docs

| File | What |
|---|---|
| `CLAUDE.md` | How an agent session works. Read first. |
| `docs/STATE.md` | Where the build currently is |
| `docs/DECISIONS.md` | Why things are the way they are |
| `docs/00-product.md` | Scope and non-goals |
| `docs/01-architecture.md` | The system |
| `docs/02-domain.md` | Money, orders, invoices, tax |
| `docs/03-conventions.md` | How to write code here |
| `docs/design/` | Tokens, components, archived reference |
| `docs/phases/` | The build plan |
| `docs/prompts/` | Copy-paste session starters |

## Licence

The server and terminal are licensed under **AGPL-3.0**. See [LICENSE](LICENSE).

The guest ordering app (`web/guest/`, arriving in Phase 11) will be **MIT**: it
is embedded in other people's pages and should be frictionless. It will carry
its own `LICENSE` file when that directory is created.
