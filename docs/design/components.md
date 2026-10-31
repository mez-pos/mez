# Components

The shared primitive inventory. Built in Phase 05. Every screen after that is
assembled from these — if a screen needs something not listed, add it here in
the same task, don't invent a one-off.

## Primitives (Phase 05)

| Component | Notes |
|---|---|
| `Button` | variants: primary (green), attention (terracotta), secondary, ghost (terracotta text), danger (brick on its tint), tint (green on its tint, for an add repeated down a list). All pills. sizes: sm (32px), md (40px), tap (48px, the default), charge (56px). Labels are 15px at every size and never wrap. `buttonClass(variant, size)` is its look for a link (ADR-406) |
| `Money` | renders an `Amount`. The numeral face, tabular, right-aligned, currency-aware. **The only way money is ever rendered.** `variant="total"` is the one serif total on a screen. Negative uses U+2212 |
| `Qty` | tabular, in one pill, with +/− tap targets. Reports the next value; holds none |
| `StatusChip` | colour **and** word. Takes `{ kind, status }` from `web/shared/ui/status.ts`: kitchen, order, payment or line. Never free text |
| `Card` | an ivory card on a hairline and a warm shadow; the ticket surface on espresso night |
| `Sheet` | bottom sheet on touch, side panel on desktop. Decided by `pointer`, not width. Escape, close control or scrim closes it; one that holds something typed or chosen (`holdsForm`: an approval, a note, a line being changed) is not closed by the scrim (TECH-9, ADR-387). Its panel pads by a phone's safe-area insets, so its foot clears the home bar (GST-6, ADR-396) |
| `Modal` | confirm, form. Escape and a visible close, always. The scrim does not close it. `Confirm` is the yes/no form. A body that fails to draw shows "This window failed to draw" with Try again and Close in place of body and footer; the title and Close stay (TECH-8, ADR-402) |
| `NumPad` | for amounts and PINs. Keys are 56px. Amounts shift in from the right as minor units; `numpad-keys.ts` does the arithmetic. `size="signin"` is the sign-in screen's (keys 48 to 76px with the screen's height) and `action` its twelfth key, Login (ADR-406) |
| `SearchField` | debounced, clearable, keyboard-dismissible. Escape clears, Enter reports and blurs |
| `DataTable` | back office only, imported from `ui/DataTable`, never the index. Sortable, cursor-paginated; the owner fetches |
| `Toast` | transient. Never for errors that need a decision: it has no danger tone. It may carry one button that takes back what it says (**Undo**), and then stays 8 seconds (ADR-387). A warning toast is the warning chip's fill, ink and border, with no stripe (ADR-390) |
| `EmptyState` | icon, one line, one action |
| `Timeline` | renders an order's event stream. Every event type has words, or the build fails |
| `Logo` | the table mark and wordmark, inline, in the surface's colours. `variant="mark"` is the table alone |
| `Meta` | the small-caps trail: `TERMINAL · REGISTER`. Items in ordinary case, uppercased by CSS |
| `focusButton`, `focusField`, `selectedRing` | `ui/focus.ts`: the one focus rule (ADR-403). Every pressable thing takes `focusButton`, every field `focusField`; a chosen tile `selectedRing`. A test fails on a second focus colour or width |
| `fieldClass`, `inputClass` | `ui/field.ts`: every field's look, and the one-line 48px field. A field is `cx(fieldClass, its size)`, never its own border and focus (ADR-406) |
| `Switch` | `ui/Switch.tsx`: the word, the sliding pill, and On or Off said; `inline` or `row`. The only `role="switch"` (ADR-406) |
| `pillClass` | `ui/pill.ts`: a toggle or tab, `{ size: "md" | "tap", chosen: "accent" | "attention" }` (ADR-406) |
| `stroke` | `ui/stroke.ts`: an icon's stroke when it is not the default: `hairline`, `large`, `bold` (ADR-405). Never a number |
| `radioGroup` | `ui/radio-group.ts`: spread on any `role="radiogroup"` of radio buttons. One tab stop on the chosen radio; arrows, Home and End choose, past a disabled one (TECH-20, ADR-403) |
| `ErrorBoundary` | where a screen's failure stops (TECH-8, ADR-402), imported by path. `size`: page (around an app or a screen with no shell: a card and Reload), region (a card with Try again and Reload, inside a shell, or in a `frame` of its own, as the order rail's column), modal (inside `Modal` and `Sheet`, already). `where` names it in the report the server logs; `resetKey` clears it; `onRetry` reads afresh before Try again draws (`useFreshRetry`); `line` is a place's own words. `RouteBoundary` (`web/shared`) is the route's, cleared by the address |

Built primitives live in `web/shared/ui/` and are exported from its
`index.ts`. The dev-only gallery at `/gallery` in the terminal shows each one
on paper and on espresso night.

`pillClass` (`web/shared/ui/pill.ts`) is the toggle and tab look: a pill,
the chosen one the green fill. Filters, view switches and area tabs use
it; an option inside a form is a tinted card instead, with a radio's dot
so the choice is never shown by colour alone (`CardChoice`, SET-9).

## Composites (built inside their own phase)

| Component | Phase |
|---|---|
| `ItemGrid`, `ModifierSheet`, `CartPanel`, `TicketList` | 06 |
| `TenderPad`, `SplitPanel`, `InvoicePreview` | 07 |
| `FloorPlan`, `TableTile`, `KDSTicket`, `StationColumn` | 08 |
| `ShiftSummary`, `BlindCountPad`, `ZReport` | 09 |
| `MenuEditor`, `ItemForm`, `ReportChart` | 10 |
| `GuestMenu`, `GuestCart`, `GuestTracking` | 11 — guest bundle only, no shadcn |

## Screen spec template

Screen specs are written **lazily**, at the start of the phase that builds them,
into `docs/design/screens/<surface>-<screen>.md`. A spec written months before
its implementation is stale fiction.

When a phase task says "write the spec for X", open the matching file in
`docs/design/reference/`, look at it, and write:

```markdown
# <Surface> — <Screen>

## Purpose
One sentence. What the person using this is trying to do.

## Layout
Regions, top to bottom. Which are fixed, which scroll.

## Data
What it reads. Which API endpoints. What it subscribes to over WebSocket.

## Interactions
Each action → what it does → what event it emits → what changes on screen.

## States
Loading, empty, error, offline, permission-denied.

## Constraints
Anything beyond the global rules. Minimum sizes, what must stay visible while
scrolling, what must work one-handed.

## Reference
Path in docs/design/reference/ this was taken from, and any deliberate
departure from it.
```

## Departing from the reference design

The generated design is a starting point, not a contract. Some screens are good;
some are not. When implementing:

- **Keep** the tokens, the touch rules, the paper and night split, the one-primary rule,
  the metadata trail, the tabular-numbers rule. These are the good part.
- **Change** any layout that fails a real service constraint — too many taps to
  fire an order, a total that scrolls off screen, an action hidden behind hover.
- **Record** the departure in the screen spec under Reference, one line, so it
  isn't relitigated later.
