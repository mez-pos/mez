# Web

Three separate Vite builds, not one app with three routes. The terminal and
back office run on a machine in the building and can afford to be fat. The
guest app runs on a stranger's phone on 3G in a basement.

| Bundle | Surface | shadcn/ui | Budget |
|---|---|---|---|
| `terminal/` | Staff counter and floor | Allowed | None stated |
| `backoffice/` | Owner and manager | Allowed | None stated |
| `guest/` | Guest phone, QR, kiosk | **Never** | 60 KB gzipped, hard; its fonts 72 KB, hard (ADR-397) |

Guest is written in React but bundled with Preact: `shared/vite-app.ts`
aliases `react` and `react-dom` to `preact/compat` for guest only (ADR-064).
The same components run in all three apps. Anything React-only (a library
that reaches into React internals) will not work in guest; `make build-web`
fails if React's runtime appears in the guest bundle.

Never let a shared component library drag shadcn into the guest bundle. If a
component is needed on all three, it is written twice or written dependency
free.

API types are generated from the Go structs. Never hand-write a type that
mirrors one. See `docs/03-conventions.md`.

## Working on the frontends

Node 22.12 or later. One npm workspace, one lockfile, in `web/`.

```sh
make build-web          # Prettier check, vitest, build all three, check the guest budget
                        # (tsc and eslint run in `make check`)
make build              # then embeds them: one binary serving all three apps
make run                # the server on :7777, which the dev servers proxy to

cd web && npm ci
npm run dev -w terminal     # http://localhost:5173/
npm run dev -w backoffice   # http://localhost:5174/backoffice/
npm run dev -w guest        # http://localhost:5175/order/
```

The dev servers proxy `/api` (HTTP and the WebSocket stream) to `:7777`, so
start the Go server first. Base paths match the mounts in `web.go`; a Vite
base that disagrees with its mount serves `index.html` for every script.

`make build` embeds the real bundles only when all three `dist/index.html`
exist, and says which it used. `make clean` removes them. `make dist` always
rebuilds them, so a release cannot ship a stale one.

Every app shares `shared/vite-app.ts`, `shared/tokens.css` and
`shared/tailwind.preset.ts`. The surface is set once, on `<html>` in each
`index.html`: `data-surface="ink"` for the terminal, `"paper"` for the others.
