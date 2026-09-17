# Design tokens

Extracted from the `claude-design` output and archived in
`docs/design/reference/`. This file is the source of truth once the reference
folder is deleted. If a colour or size is needed and not here, add it here
first, then use it.

Every value below was read out of the archived design files in Phase 00 Task 3
and counted by frequency. Where the design and this doc disagreed on a fact,
the design won and the old value is noted. Where they disagreed on a *rule*,
each case was argued on its merits and settled; see "Where this doc overrules
the design" at the end. Nothing here is left open.

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

### Type scale

Sizes are named by role, not by number. A component picks the role; it does not
pick a pixel value.

| Token | Size | Role |
|---|---|---|
| `--text-meta` | `11px` | Uppercase letter-spaced `.12em` metadata only. The floor. |
| `--text-sm` | `13px` | Secondary text, table cells, dense back office rows |
| `--text-base` | `15px` | Body, item names, every interactive label |
| `--text-lg` | `17px` | Sub-headings, emphasised rows |
| `--text-xl` | `21px` | Section titles |
| `--text-total` | `32px` | Instrument Serif, the one total on a screen |

### The 11px floor

**Nothing renders below 11px, and 11px is only available to uppercase,
letter-spaced metadata.** All sentence-case text is 13px or larger.

The archived design does not honour this. It sets invoice column headers at
9px, and the split-bill controls ("Evenly", "By item", "By seat") at 10px, and
the ticket-rail actions ("Modify", "Void", "Discount", "Note") at 11px
sentence-case. Uppercase at 9px is defensible because cap-height and tracking
carry it; an 11px sentence-case **Void** button sitting next to **Modify**, read
at arm's length through glare by someone wearing gloves, is not. The cost of
misreading it is a destroyed line on a live ticket.

So: metadata rises 9px to `--text-meta`, and every interactive label rises to
`--text-base`. This is a deliberate, recorded deviation from the mock, not an
oversight. See ADR-014.

**Consequence for Phase 06:** the terminal's right rail is 392px in the design
and its line lengths assume the smaller text. At `--text-base` some labels will
wrap. Re-check rail width and truncation when building the register, and widen
the rail rather than dropping back below the floor.

### Rules

- **Every number is monospaced and right-aligned**, with `font-variant-numeric:
  tabular-nums`, so a column of rupees reads down and decimal points align.
  Prices, quantities, totals, times, table numbers, invoice numbers. The design
  sets `font-feature-settings:'tnum' 1` on the root of all three surfaces.
- The small-caps metadata trail under list items (`TERMINAL · REGISTER`) is
  Archivo, uppercase, letter-spaced `.12em`, faint text colour, `--text-meta`.
- Instrument Serif is for the total on a screen, not for every heading in a
  form. Back office uses it sparingly.
- **Form controls are 16px minimum on every surface**, no exceptions. Mobile
  Safari zooms the viewport when an `input`, `select` or `textarea` below 16px
  takes focus, and on the guest surface that zoom strands the guest mid-order
  with no way back. This is a rule about controls, not about body text. The
  archived design contains no real form elements, so it neither confirms nor
  contradicts it.

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

## Spacing scale

**2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 32, 48, 64. Nothing else.**

This doc previously said "4 / 8 / 12 / 16 / 24 / 32 / 48 / 64. Nothing else."
The design does not obey that and never did: its most common gap is 10px, used
67 times, which a 4px grid cannot express.

The scale above is a 2px base through 16px, then 4px and wider. Two properties
make it the right answer rather than a capitulation:

- It already covers 71% of the design's spacing untouched, including the 10px
  gap that carries the terminal's density. Forcing 10px to 8px or 12px would
  restyle every dense list in the app to fix a rule, which is backwards.
- Every value is a **native Tailwind class**, so nothing needs an arbitrary
  value: 2px is `gap-0.5`, 6px is `gap-1.5`, 10px is `gap-2.5`, 14px is
  `gap-3.5`. Tailwind's `--spacing` is `0.25rem` and numeric utilities multiply
  it, and the 0.5/1.5/2.5/3.5 micro-steps have shipped by default since v2. The
  "no arbitrary values" rule survives intact.

The remaining 29% snaps to the nearest step: 7 and 9 go to 8, 11 goes to 12,
18 goes to 16 or 20 by eye. Those four account for 73 uses across 35 screens
and none of them is load-bearing.

Be clear about who is imposing this. Tailwind v4 resolves *any* multiple of
`--spacing`, so `gap-4.5` would render 18px perfectly well. The scale is not a
limit the framework sets; it is a limit we set, so that 35 screens built in 12
phases by different sessions land on the same rhythm. A reviewer should be able
to read a diff and see a step, not a pixel someone liked.

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

## Where this doc overrules the design

Three rules here are contradicted by the archived design. All three are settled
(ADR-014); none is open. The principle used to settle them:

> A measurement is evidence about a fact. It is not evidence about a
> constraint. Where the design and a constraint disagree, the constraint has to
> justify itself on its own merits, and then one of the two changes.

| Conflict | Settled as | Who moved |
|---|---|---|
| Spacing scale | 2px base: 2/4/6/8/10/12/14/16/20/24/32/48/64 | **The doc moved.** The old 4px grid could not express the 10px gap the design leans on 67 times, and the new scale is entirely native Tailwind. |
| Body font size | 11px floor, metadata only; 13px sentence-case minimum; 15px for every interactive label | **The design moves.** 9px uppercase is defensible; an 11px sentence-case "Void" next to "Modify" under glare is not. |
| Guest 16px minimum | Scoped to form controls on every surface, not body text | **Neither.** The conflict was not real. The Safari zoom-on-focus behaviour applies to `input`, `select` and `textarea`, and the mock contains none. |

Two of the three moved the doc, not the design, which is the honest outcome: the
old spacing scale and the old flat "13 to 15px body" rule were both written
before anyone measured anything. The one place the design gives way is the one
place a human can be hurt by it.

## Implementation

Phase 05 Task 1 turns this file into:

- `web/shared/tokens.css`: CSS custom properties, `[data-surface="ink"]` and
  `[data-surface="paper"]` scopes
- `tailwind.config.ts`: theme extension referencing the CSS variables, never
  raw hex

Components reference Tailwind classes only. A raw hex value in a `.tsx` file is
a bug.
