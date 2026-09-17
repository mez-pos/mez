# Phase 05 — Frontend shell

## Goal
Three Vite apps, the token system in real CSS, the primitive components, routing,
and a working staff sign-in. No business screens yet.

## Depends on
Phase 04. Read `docs/design/tokens.md` and `docs/design/components.md`.

## Tasks

- [ ] **Task 1 — Tokens into code.**
  `web/shared/tokens.css`: every token from `docs/design/tokens.md` as a CSS
  custom property, scoped by `[data-surface="ink"]` and `[data-surface="paper"]`.
  `web/shared/tailwind.preset.ts` referencing the variables — never raw hex.
  Fonts self-hosted (Instrument Serif, Archivo, JetBrains Mono), subset, with
  `font-display: swap` and a real fallback stack. A single demo page rendering
  every token so the human can eyeball it.

- [ ] **Task 2 — Three Vite apps.**
  `web/terminal`, `web/backoffice`, `web/guest`. React 18, TS strict, Tailwind
  with the shared preset. Correct base paths matching Phase 01 Task 4. Dev proxy
  to `:7777`. `make build-web` builds all three into the paths `embed.FS`
  expects. Add a bundle-size check that **fails the build if the guest bundle
  exceeds 60 KB gzipped**.

- [ ] **Task 3 — API client and realtime hook.**
  `web/shared/api.ts` using the generated types. React Query set up per app.
  A `useRealtime()` hook wrapping the WebSocket: tracks last `seq`, reconnects
  with backoff, exposes a connection state. A visible, honest connection
  indicator component — staff must be able to tell at a glance whether the
  terminal is talking to the server.

- [ ] **Task 4 — Primitives, part one.**
  `Button`, `Money`, `Qty`, `StatusChip`, `Card`, `Meta`. `Money` is the only
  path by which money is ever rendered — mono, tabular, right-aligned. `Button`
  sizes include `tap` (44px) and `charge` (58px). `StatusChip` takes a status
  enum and always renders colour **and** word. A component gallery route in the
  terminal app, dev-only.

- [ ] **Task 5 — Primitives, part two.**
  `Sheet`, `Modal`, `NumPad`, `SearchField`, `Toast`, `EmptyState`, `DataTable`,
  `Timeline`. `NumPad` keys are 44px minimum and usable one-handed. Modals close
  on Escape and have a visible close control. Added to the gallery.

- [ ] **Task 6 — Device pairing and staff lock.**
  First-run pairing screen redeeming a code for a device token, stored durably.
  The staff lock screen: PIN entry on the NumPad, lockout feedback, clear error
  states. Route guards. Back office sign-in. This is the first screen the human
  will judge — get it right, and write its spec into
  `docs/design/screens/terminal-lock.md` as you build it.

## Done when
`make build-web && make build` produces one binary serving all three apps. A
device pairs, a staff member signs in with a PIN, and the gallery shows every
primitive on both ink and paper.
