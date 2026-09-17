# Web

Three separate Vite builds, not one app with three routes. The terminal and
back office run on a machine in the building and can afford to be fat. The
guest app runs on a stranger's phone on 3G in a basement.

| Bundle | Surface | shadcn/ui | Budget |
|---|---|---|---|
| `terminal/` | Staff counter and floor | Allowed | None stated |
| `backoffice/` | Owner and manager | Allowed | None stated |
| `guest/` | Guest phone, QR, kiosk | **Never** | 60 KB gzipped, hard |

Never let a shared component library drag shadcn into the guest bundle. If a
component is needed on all three, it is written twice or written dependency
free.

API types are generated from the Go structs. Never hand-write a type that
mirrors one. See `docs/03-conventions.md`.
