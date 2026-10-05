# Design tokens

The mez design language, read out of `docs/design/reference/mez/` (the
design system file first, then the five screen files). This file is the
source of truth. If a colour or size is needed and not here, add it here
first, then use it. Never introduce a colour outside this sheet.

The language before the rename is kept in `tokens-nofiltr.md`, as history.
ADR-128 records the change.

## The language in one paragraph

A beautifully printed menu, brought to life. Parchment ground and ivory
cards, floated on warm hairlines and low brown shadows, never grey. Forest
green commits. Terracotta asks for attention. Serif headings in a maroon
ink, a neutral sans for everything functional. Status is a pastel fill, a
saturated dot and a word, in a pill. Round, soft shapes.

## Surfaces

Two surfaces. `data-surface="paper"` is every screen but one: till, back
office, guest. `data-surface="ink"` is **espresso night**, the kitchen
display, where a bright screen over a hot pass is the wrong tool.

### Paper

| Token | Hex | Role | Use |
|---|---|---|---|
| `--parchment` | `#F8F3E9` | `canvas` | Page ground |
| `--card` | `#FFFDF8` | `raised`, `chrome` | Panels, cards, header, navigation |
| `--card-hover` | `#F6EFE2` | `raised-high` | Hover and pressed fills on a card |
| `--well` | `#F2EADB` | `sunken` | Inputs, wells, keys |

### Espresso night

| Token | Hex | Role |
|---|---|---|
| `--night` | `#241C17` | `canvas` |
| `--night-chrome` | `#2E241E` | `chrome` |
| `--night-raised` | `#332821` | `raised` (the ticket) |
| `--night-raised-high` | `#3D3028` | `raised-high` (derived) |

## Text

| Token | Hex | Role | On parchment |
|---|---|---|---|
| `--heading` | `#5B1F1F` | `fg-heading` | 12.4:1. Serif titles and display prices |
| `--body` | `#3A2E2A` | `fg` | 11.1:1. Body |
| `--body-2` | `#5F5850` | `fg-2` | Secondary text |
| `--muted` | `#7A6E66` | `fg-3`, `fg-muted` | 4.6:1. The floor for sentence text, 13px minimum |
| `--faint` | `#8B7F72` | `fg-faint` | Uppercase metadata only |

On night: `--night-text` `#F5EADB`, `--night-text-2` `#E4D6C4`,
`--night-text-3` `#BCA995`, `--night-text-muted` `#A8968A`.

## Rules

| Token | Hex | Role |
|---|---|---|
| `--hairline` | `#EAE0D0` | `line`. Card borders, dividers |
| `--hairline-soft` | `#F4EDE0` | `line-soft`. Row dividers inside a table |
| `--hairline-strong` | `#DCCDB4` | `line-strong`. Inputs, secondary buttons |

Focus is green on a field (a 1.5px border and a 3px `--green-tint` halo)
and a 3px terracotta outline on a button. On night: `--night-line`
`#47382E`, `--night-line-strong` `#4A3A30`, focus in mustard.

## Brand and action

**Green commits. Terracotta asks for attention. One primary per screen.**

| Token | Hex | Role | Use |
|---|---|---|---|
| `--green` | `#2F4A2C` | `accent` | Settle, send, save, confirm. Success |
| `--green-press` | `#24391F` | `accent-hover` | Hover and pressed |
| `--green-tint` | `#E8EFE3` | `accent-tint` | Selected rows, tint fills, focus halo |
| `--terracotta` | `#B9522F` | `attention` | Active navigation, New order, Export, links |
| `--terracotta-press` | `#9C4325` | `attention-hover` | Hover; terracotta as text on its tint |
| `--terracotta-tint` | `#F6E4DC` | `attention-tint` | Offers, soft badges, ghost hover |
| `--mustard` | `#E7A83A` | `highlight` | Bestseller, ratings, the logo's top. Never text on a light ground |
| `--brick` | `#A33A28` | `danger`, `danger-fill` | Void, refund, delete, errors |
| `--on-fill` | `#FFFDF8` | `accent-fg`, `attention-fg` | Text on a green, terracotta or brick fill |
| `--on-mustard` | `#40320D` | `highlight-fg` | Text on a mustard fill: the kitchen's Preparing band. 7.4:1 |

Brick has a tint for the destructive button: `--brick-tint` `#FBEDE9`,
hover `#F6DDD6`, border `--brick-line` `#E7C3B9`.

## Status

A fill and an ink, the ink darkened until it holds 4.5:1 on its fill.

| Meaning | Fill | Ink | Chip tone |
|---|---|---|---|
| Free, ready, paid | `#DDE8D3` | `#216B26` | `success` |
| Occupied, refunded, failed | `#F6D3CB` | `#A8321C` | `danger` |
| Needs bill, pending | `#FAEBC8` | `#7A5306` | `warning` |
| Cleaning, neutral, void | `#E8E2DA` | `#5A4F46` | `neutral` |
| Info, seated, held | `#DCE8F7` | `#23509A` | `info` |
| Late | `#F6E4DC` | `#9C4325` | `accent` |
| Reserved | `#E2D9F0` | `#55399F` | no state uses it yet |

On night the fills are dark and the inks light: green `#31402C` and
`#A8CE97`, brick `#5A2A20` and `#FFC4B4`, a late ticket's rule `#C4432B`.
Mustard and info on night are derived (`tokens.css` marks them).

**Status is colour plus a word. Never colour alone.** A chip reading
"Ready" in green is correct; a green dot alone is not. Kitchen staff work
at an angle, in steam, and some of them are colour-blind.

A table's five states each take their own tone, so the floor reads at a
glance: free `success`, seated `info`, ordered `danger` (the occupied
rose, no alarm), paying `warning`, to clear `neutral`.

## Typography

| Family | Role | Class |
|---|---|---|
| **Fraunces** 600 | Titles, the total on a screen | `font-serif` |
| **DM Sans** | All interface text | `font-sans` |
| **Archivo** | Every number in a column | `font-mono` |
| **Caveat** | Script garnish | `font-script` |

`font-serif` carries its own ink and weight: heading maroon, 600,
`-0.02em`. A colour or weight class on the same element still wins.

**Archivo is the numeral face, and `font-mono` is its class, though it is
no monospace.** The design asks for tabular numerals throughout, but
Fraunces and DM Sans as Google ships them have no `tnum` feature: their
digits are proportional and a column of rupees would not align. Archivo
has it. The class kept its name to spare a hundred call sites. ADR-128.

Script is decoration: never a button, label, price, status or field. One
script line per screen region at most.

### Type scale

| Token | Size | Role |
|---|---|---|
| `--text-meta` | `11px` | Uppercase, `.16em`, 700. Metadata only. The floor |
| `--text-sm` | `13px` | Secondary text, table cells |
| `--text-base` | `15px` | Body, every interactive label |
| `--text-lg` | `17px` | Sub-headings, emphasised rows |
| `--text-xl` | `22px` | Section titles (serif) |
| `--text-total` | `32px` | Page titles, the one total on a screen (serif) |
| `--text-kitchen` | `26px` | The kitchen display's lines (ADR-091) |
| `--text-display` | `44px` | The kitchen display's table and timer; the login title |
| `--text-hero` | `66px` | The login screen's headline, from 1536px wide (D01) |

Kept from before, because the reasons have not changed (ADR-014):
nothing renders below 11px, sentence-case text is 13px or larger, every
interactive label is 15px, and form controls are 16px minimum so mobile
Safari does not zoom. The design draws small buttons at 13 and 14px; the
15px floor wins.

## Touch sizing

| Token | Value | Rule |
|---|---|---|
| `--tap-min` | `48px` | Minimum for anything tapped during service |
| `--tap-charge` | `56px` | The charge button. Always |
| `--tap-gap` | `8px` | Minimum gap between adjacent tap targets |

Mouse-and-keyboard screens in the back office may use 40px (`md`) and
32px (`sm`) buttons. No hover-only affordances, no drag as the only path.

## Spacing

Unchanged: **2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 32, 48, 64**, all native
Tailwind steps. The design's own scale (4, 8, 12, 16, 24, 32, 48) is
inside it. Card padding is 24, panel padding 32 on a desktop.

## Radius

| Token | Value | Class | Use |
|---|---|---|---|
| `--radius-tight` | `8px` | `rounded-tight` | Small tags, swatches |
| `--radius` | `14px` | `rounded` | Inputs, tiles, keys, rows |
| `--radius-card` | `20px` | `rounded-card` | Cards, panels |
| `--radius-sheet` | `24px` | `rounded-sheet` | Sheets, modals |
| `--radius-pill` | `999px` | `rounded-pill` | Buttons, chips, pills |
| `--radius-round` | `50%` | `rounded-round` | Avatars, dots |

## Elevation

Warm, never grey. Each class resolves per surface.

| Class | On paper | On night |
|---|---|---|
| `shadow-rest` | `0 1px 2px rgba(91,31,31,.05)` | none |
| `shadow-card` | `0 2px 6px rgba(91,31,31,.04), 0 12px 26px rgba(91,31,31,.06)` | `0 12px 26px rgba(0,0,0,.28)` |
| `shadow-modal` | `0 8px 18px rgba(91,31,31,.08), 0 28px 56px rgba(91,31,31,.10)` | as card |
| `shadow-action` | `0 4px 12px rgba(47,74,44,.22)`, under the primary button | none |

`--scrim` is `rgba(58,46,42,.44)`, with no backdrop blur: a full-screen
blur is the most expensive thing a cheap tablet can be asked to paint.

## Buttons

| Variant | Look | For |
|---|---|---|
| `primary` | Green fill, ivory text | Settle, send, save, confirm |
| `attention` | Terracotta fill | New order, export, generate |
| `secondary` | Ivory, strong hairline | Reversible, beside a primary |
| `ghost` | Terracotta text | Navigation, inline links |
| `danger` | Brick text on brick tint | Void, refund. Always confirmed |

All pills. Sizes: `charge` 56, `tap` 48, `md` 40, `sm` 32.

## Logo

The table mark: a mustard top over two arches, with the wordmark. Never
redraw or stretch it. The files in `web/shared/brand/` are the brand's,
recoloured to this sheet: the arches and wordmark in `--green`, the top
in `--mustard`, ivory `--card` where the ground is dark. `Logo` in
`web/shared/ui` draws the same shapes inline, in the surface's colours.
Minimum mark size 20px. The mono files are for thermal printers.

## Icons

Line icons in `currentColor`, 1.75px stroke, round caps and joins, on a
24px grid. Never filled, except status dots and the veg marker.

## Implementation

- `web/shared/tokens.css`: every token above on `:root`, plus the roles
  that `[data-surface="paper"]` and `[data-surface="ink"]` resolve.
- `web/shared/tailwind.preset.ts`: exposes the roles only, replacing
  Tailwind's default colours, sizes, radii and shadows. Never raw hex.

Components reference Tailwind classes only. A raw hex value in a `.tsx`
file is a bug.
