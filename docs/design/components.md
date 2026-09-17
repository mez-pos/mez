# Components

The shared primitive inventory. Built in Phase 05. Every screen after that is
assembled from these — if a screen needs something not listed, add it here in
the same task, don't invent a one-off.

## Primitives (Phase 05)

| Component | Notes |
|---|---|
| `Button` | variants: primary (vermilion), secondary, ghost, danger. sizes: sm, md, tap (44px), charge (58px) |
| `Money` | renders an `Amount`. Mono, tabular, right-aligned, currency-aware. **The only way money is ever rendered.** |
| `Qty` | mono, tabular, with +/− tap targets |
| `StatusChip` | colour **and** word. Takes a status enum, never free text |
| `Card` | ink-raised or paper surface |
| `Sheet` | bottom sheet on touch, side panel on desktop |
| `Modal` | confirm, form. Escape and a visible close, always |
| `NumPad` | for amounts and PINs. 44px keys minimum |
| `SearchField` | debounced, clearable, keyboard-dismissible |
| `DataTable` | back office only. Sortable, cursor-paginated |
| `Toast` | transient. Never for errors that need a decision |
| `EmptyState` | icon, one line, one action |
| `Timeline` | renders an order's event stream |
| `Meta` | the small-caps trail: `TERMINAL · REGISTER` |

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
