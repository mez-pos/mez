# Design tokens

Extracted from the `claude-design` output and archived in
`docs/design/reference/`. This file is the source of truth once the reference
folder is deleted. If a colour or size is needed and not here, add it here
first, then use it.

Every value below was read out of the archived design files in Phase 00 Task 3
and counted by frequency. Where the design and this doc disagreed on a fact,
the design won and the old value is noted. Where they disagree on a *rule*,
nothing was changed: see "Unresolved conflicts" at the end.

## Ink and paper

The terminal runs on **warm ink**: dark, for ten-hour shifts under service
lighting. Back office and guest surfaces run on **paper**: light. One vermilion
carries the brand across all three, at two different values (see Accent).

### Ink surfaces (Terminal)

| Token | Hex | Use |
|---|---|---|
| `--ink` | `#0E0C09` | Page background |
| `--ink-chrome` | `#14110D` | Nav rail, right rail, header bars |
| `--ink-raised` | `#1A1712` | Cards, panels, rows |
| `--ink-raised-high` | `#221E17` | Inputs, selected rows, the layer above a card |

The design also uses `#1C1913` for cards, 31 times, which is 2/2/1 away from
`--ink-raised` and indistinguishable on screen. Treat it as the same token and
do not add it.

### Paper surfaces (Back office, Guest)

| Token | Hex | Use |
|---|---|---|
| `--paper` | `#F4F1E9` | Page background |
| `--paper-raised` | `#FFFDF8` | Cards, panels, inputs, table rows |
| `--paper-sunken` | `#F2EDE1` | Recessed wells, alternating rows |

`--paper` was `#FBF7EF` in this doc before Task 3. The archived back office
root is `background:#F4F1E9`, and `#FBF7EF` appears three times in total, so
the old value was wrong. `#FBF9F3`, `#F8F5EE` and `#EDE7DA` also appear as
one-off near-duplicates of `--paper-raised`; collapse them into it.

A caution for anyone reading the archive: the Guest file's `body` background is
`#141109`, dark. That is the prototype canvas behind a phone mockup, not a
guest surface. Guest screens are paper, inside the frame.

## Text

Percentages are the measured opacity of the text colour over its surface, given
for anyone deriving an intermediate value. Use the hex.

### On ink

| Token | Hex | % | Use |
|---|---|---|---|
| `--ink-text` | `#F3EFE6` | 100 | Body, the root text colour |
| `--ink-text-2` | `#C9C2B2` | 82 | Secondary text, item names in lists |
| `--ink-text-3` | `#A39C8C` | 65 | Tertiary, supporting detail |
| `--ink-text-muted` | `#8C8474` | 55 | Labels |
| `--ink-text-faint` | `#6E685C` | 42 | Metadata, the small-caps trail |

`#150E09` is the text colour *on* vermilion fills, not a surface text colour.

### On paper

| Token | Hex | % | Use |
|---|---|---|---|
| `--paper-text` | `#1A1712` | 100 | Body, the root text colour |
| `--paper-text-2` | `#4A4438` | 78 | Secondary text |
| `--paper-text-3` | `#6E685C` | 61 | Tertiary, supporting detail |
| `--paper-text-muted` | `#8A8274` | 49 | Labels |
| `--paper-text-faint` | `#9A9384` | 41 | Metadata, column headers |

## Hairline

| Token | Value | Use |
|---|---|---|
| `--ink-hairline` | `rgba(255,255,255,.07)` | Dividers and card borders on ink |
| `--ink-hairline-2` | `rgba(255,255,255,.10)` | Stronger separation, input borders |
| `--ink-hairline-3` | `rgba(255,255,255,.14)` | Emphasis, focus |
| `--paper-hairline` | `#E3DED2` | Dividers and card borders on paper |
| `--paper-hairline-soft` | `#EFEAE0` | Interior rules inside a card |

This doc previously said hairline was "12% of primary". Measured, it is 7% on
ink. `--paper-hairline` is a solid hex, not an alpha.

## Accent

**One accent, enforced.** Vermilion means money, action or now. It is not
decoration, not a heading colour, not a hover state on something inert. If a
screen has two vermilion elements competing, one of them is wrong.

Vermilion has two values because a single hex cannot hold contrast on both
grounds. Pick by surface, never by taste.

| Token | Hex | Use |
|---|---|---|
| `--vermilion` | `#DD5A2A` | The accent on ink |
| `--vermilion-hover` | `#F07440` | Hover and active on ink |
| `--vermilion-on` | `#150E09` | Text and icons on a vermilion fill |
| `--vermilion-paper` | `#C24D22` | The accent on paper: fills, borders, dots |
| `--vermilion-paper-text` | `#B4522F` | Vermilion as text on paper, including mono numerals |

## Status colours

| Meaning | On ink | On paper |
|---|---|---|
| Success, settled, ready | `--sage` `#7FA06B` | `--sage-paper` `#5E7A4E` |
| Warning, pending, attention | `--gold-ink` `#E0A33C` | `--gold` `#C79A3C` |
| Danger, error, refund, void | `--danger` `#D45A4E` | `--danger-paper` `#B23A2C` |
| Info, channel, reserved | `--slate` `#7E86A8` | `--slate` `#7E86A8` |

`--danger` was "to be defined" before Task 3. It is a desaturated red, clearly
separate from vermilion. Vermilion is never an error colour.

`--slate` was missing entirely. The design uses it for channel identity ("Web
and delivery"), the "Reserved" table state, and as a neutral data series in
back office charts. It is the one status colour that is the same on both
grounds.

### Status chips on paper

Chips are a triad, not a single colour:

| State | Background | Border | Text |
|---|---|---|---|
| Neutral | `#FFFDF8` | `#E3DED2` | `#6E685C` |
| Success | `#F2F5EF` | `#D6DFCE` | `#5E7A4E` |
| Warning | `#FBF1E8` | `#EDD6BF` | `#B4522F` |
| Danger | `#FCECEA` | `#EFCFCA` | `#B23A2C` |

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

## Typography

| Family | Role |
|---|---|
| **Instrument Serif** | Titles, totals, guest greetings |
| **Archivo** | All interface text |
| **JetBrains Mono** | Every number, tabular figures |

Rules:

- **Every number is monospaced and right-aligned**, with `font-variant-numeric:
  tabular-nums`, so a column of rupees reads down and decimal points align.
  Prices, quantities, totals, times, table numbers, invoice numbers. The design
  sets `font-feature-settings:'tnum' 1` on the root of all three surfaces.
- The small-caps metadata trail under list items (`TERMINAL · REGISTER`) is
  Archivo, uppercase, letter-spaced `.12em`, faint text colour, 9 to 11px.
- Instrument Serif is for the total on a screen, not for every heading in a
  form. Back office uses it sparingly.

## Touch sizing, hard constraints

| Token | Value | Rule |
|---|---|---|
| `--tap-min` | `44px` | Absolute minimum for anything tappable during service |
| `--tap-charge` | `58px` | The charge/pay button. Always |
| `--tap-gap` | `8px` | Minimum gap between adjacent tap targets |

Verified against the design: 44px and 46px are the common target heights and
nothing interactive in the terminal is below 44px. The charge button is 58px.

A 12-inch tablet, gloves, glare, a queue. No hover-only affordances. No
right-click. No drag as the only path to an action.

## Radius

| Token | Value | Use |
|---|---|---|
| `--radius-none` | `0` | Hairline dividers, full-bleed rows |
| `--radius` | `3px` | Everything: cards, chips, inputs, buttons |
| `--radius-tight` | `2px` | Small chips, tags, swatches |
| `--radius-round` | `50%` | Avatars, status dots |

This doc previously said 0 / 4 / 8 / 12. The design uses 3px 180 times and 2px
83 times; 8px and 12px are effectively unused. The house is square, not soft.

## Elevation

Elevation on ink is surface lightness, not shadow: shadows disappear on a dark
background under glare. Climb `--ink` to `--ink-chrome` to `--ink-raised` to
`--ink-raised-high`.

On paper, a single soft shadow level, plus `--paper-hairline`. The only heavy
shadow in the design is on the printed-invoice preview, which is deliberately
pretending to be paper on a desk.

## Surface-specific defaults

**Terminal**: ink background, dense, high contrast, big targets.
**Back office**: paper background, data tables, keyboard-friendly.
**Guest**: paper background, generous spacing, and a hard 60 KB gzipped bundle
budget.

## Unresolved conflicts

Three rules in this doc are contradicted by the design. They are **not**
resolved here, because each is a deliberate constraint that the prototype may
simply not be honouring, and overwriting a touch-ergonomics rule with a mock's
values would be a real regression. A human decides these.

1. **Spacing scale.** This doc said "4 / 8 / 12 / 16 / 24 / 32 / 48 / 64.
   Nothing else." The design's most common gap is `10px` (67 uses), then `8px`
   (35), `9px` (33), `12px` (33), and it freely uses 6, 7, 11, 14 and 18. The
   design is not on a 4px grid. Either the scale is wrong, or the design needs
   snapping to it during Phase 05.
2. **Body font size.** This doc said Archivo at "13 to 15px on touch". The
   design's dominant sizes are 12px (128), 11px (115), 13px (90), 10px (88) and
   9px (49). Terminal text is smaller than the stated rule allows.
3. **Guest 16px minimum.** This doc said 16px minimum body, because iOS zooms
   on focus of an input below 16px. The guest design's dominant sizes are 11,
   13 and 15px. Note the iOS behaviour applies to real `<input>` elements, and
   the prototype has none, so the design may simply not exercise the rule. The
   rule is kept as written.

Until these are settled, prefer this doc's rule over the design's measurement,
and raise it in the phase that builds the screen.

## Implementation

Phase 05 Task 1 turns this file into:

- `web/shared/tokens.css`: CSS custom properties, `[data-surface="ink"]` and
  `[data-surface="paper"]` scopes
- `tailwind.config.ts`: theme extension referencing the CSS variables, never
  raw hex

Components reference Tailwind classes only. A raw hex value in a `.tsx` file is
a bug.
