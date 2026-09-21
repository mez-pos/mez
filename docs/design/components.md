# Components

The shared primitive inventory. Built in Phase 05. Every screen after that is
assembled from these — if a screen needs something not listed, add it here in
the same task, don't invent a one-off.

## Primitives (Phase 05)

| Component | Notes |
|---|---|
| `Button` | variants: primary (vermilion), secondary, ghost, danger (outlined). sizes: sm (32px), md (36px), tap (44px, the default), charge (58px). Labels are 15px at every size |
| `Money` | renders an `Amount`. Mono, tabular, right-aligned, currency-aware. **The only way money is ever rendered.** `variant="total"` is the one serif total on a screen. Negative uses U+2212 |
| `Qty` | mono, tabular, with +/− tap targets. Reports the next value; holds none |
| `StatusChip` | colour **and** word. Takes `{ kind, status }` from `web/shared/ui/status.ts`: kitchen, order, payment or line. Never free text |
| `Card` | ink-raised or paper surface |
| `Sheet` | bottom sheet on touch, side panel on desktop. Decided by `pointer`, not width. Escape, close control or scrim closes it |
| `Modal` | confirm, form. Escape and a visible close, always. The scrim does not close it. `Confirm` is the yes/no form |
| `NumPad` | for amounts and PINs. Keys are 56px. Amounts shift in from the right as minor units; `numpad-keys.ts` does the arithmetic |
| `SearchField` | debounced, clearable, keyboard-dismissible. Escape clears, Enter reports and blurs |
| `DataTable` | back office only, imported from `ui/DataTable`, never the index. Sortable, cursor-paginated; the owner fetches |
| `Toast` | transient. Never for errors that need a decision: it has no danger tone |
| `EmptyState` | icon, one line, one action |
| `Timeline` | renders an order's event stream. Every event type has words, or the build fails |
| `Meta` | the small-caps trail: `TERMINAL · REGISTER`. Items in ordinary case, uppercased by CSS |

Built primitives live in `web/shared/ui/` and are exported from its
`index.ts`. The dev-only gallery at `/gallery` in the terminal shows each one
on ink and on paper.

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

- **Keep** the tokens, the touch rules, the ink/paper split, the one-accent rule,
  the metadata trail, the monospaced-numbers rule. These are the good part.
- **Change** any layout that fails a real service constraint — too many taps to
  fire an order, a total that scrolls off screen, an action hidden behind hover.
- **Record** the departure in the screen spec under Reference, one line, so it
  isn't relitigated later.
