# Design tokens

Extracted from the `claude-design` output and archived in
`docs/design/reference/`. This file is the source of truth once the reference
folder is deleted. If a colour or size is needed and not here, add it here
first, then use it.

## Ink and paper

The terminal runs on **warm ink** — dark, for ten-hour shifts under service
lighting. Back office and guest surfaces run on **paper** — light. One vermilion
carries the brand across all three.

| Token | Hex | Use |
|---|---|---|
| `--ink` | `#0E0C09` | Terminal background |
| `--ink-raised` | `#1A1712` | Cards, panels, raised surfaces on ink |
| `--vermilion` | `#DD5A2A` | The single accent. Money, action, now |
| `--gold` | `#C79A3C` | Warning, pending, attention |
| `--sage` | `#7FA06B` | Success, settled, ready |
| `--paper` | `#FBF7EF` | Back office and guest background |

Derived, to be defined in `tokens.css` during Phase 05:

| Token | Meaning |
|---|---|
| `--text-primary` | `--paper` on ink surfaces, `--ink` on paper surfaces |
| `--text-secondary` | 70% of primary |
| `--text-muted` | 45% of primary — labels, metadata, the small-caps trail |
| `--hairline` | 12% of primary — borders, dividers |
| `--danger` | A red distinct from vermilion. Vermilion is not an error colour |

**One accent, enforced.** Vermilion means money, action or now. It is not
decoration, not a heading colour, not a hover state on something inert. If a
screen has two vermilion elements competing, one of them is wrong.

## Typography

| Family | Role |
|---|---|
| **Instrument Serif** | Titles, totals, guest greetings |
| **Archivo** | All interface text, 13–15px on touch |
| **JetBrains Mono** | Every number, tabular figures |

Rules:

- **Every number is monospaced and right-aligned**, with `font-variant-numeric:
  tabular-nums`, so a column of rupees reads down and decimal points align.
  Prices, quantities, totals, times, table numbers, invoice numbers.
- The small-caps metadata trail under list items (`TERMINAL · REGISTER`) is
  Archivo, uppercase, letter-spaced, `--text-muted`, ~11px.
- Instrument Serif is for the total on a screen, not for every heading in a
  form. Back office uses it sparingly.

## Touch sizing — hard constraints

| Token | Value | Rule |
|---|---|---|
| `--tap-min` | `44px` | Absolute minimum for anything tappable during service |
| `--tap-charge` | `58px` | The charge/pay button. Always |
| `--tap-gap` | `8px` | Minimum gap between adjacent tap targets |

A 12-inch tablet, gloves, glare, a queue. No hover-only affordances. No
right-click. No drag as the only path to an action.

## Spacing scale

4 / 8 / 12 / 16 / 24 / 32 / 48 / 64. Nothing else. Tailwind's default scale maps
to this; do not use arbitrary values.

## Radius and elevation

Radius: 0 (hairline dividers), 4 (inputs, chips), 8 (cards), 12 (modals).
Elevation on ink is surface lightness, not shadow — shadows disappear on a dark
background under glare. On paper, a single soft shadow level.

## Status colour rules

**Status is colour plus a word. Never colour alone.** A chip reading "Ready" in
sage is correct; a sage dot alone is not. Kitchen staff work at an angle, in
steam, and some of them are colour-blind.

| State | Colour | Word |
|---|---|---|
| Queued | muted | Queued |
| Fired / in progress | gold | Firing |
| Ready | sage | Ready |
| Late / attention | vermilion | Late |
| Voided | muted + strikethrough | Void |

## Surface-specific defaults

**Terminal** — ink background, dense, 15px body, high contrast, big targets.
**Back office** — paper background, 14px body, data tables, keyboard-friendly.
**Guest** — paper background, generous spacing, 16px minimum body (iOS zooms
below 16px on input focus), and a hard 60 KB gzipped bundle budget.

## Implementation

Phase 05 Task 1 turns this file into:

- `web/shared/tokens.css` — CSS custom properties, `[data-surface="ink"]` and
  `[data-surface="paper"]` scopes
- `tailwind.config.ts` — theme extension referencing the CSS variables, never
  raw hex

Components reference Tailwind classes only. A raw hex value in a `.tsx` file is
a bug.
