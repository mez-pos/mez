# Design reference index

What each archived file contains, and where each screen lives inside it.

**Read one row, open one file, jump to one line.** These files are 13 KB to
131 KB of inline-styled markup. Opening them to browse is the fastest way to
burn a session's context. Find the screen here first.

## The files

| File | Surface | Screens | What it is |
|---|---|---|---|
| `Nofiltr POS System.dc.html` | (none) | 0 | Cover page: positioning, the 35-screen count, type stack, palette rationale, house rules. Its content is unbound template placeholders, so it lists no screens. |
| `Nofiltr Terminal.dc.html` | Terminal | 12 | Staff-facing counter and floor app. Dark ink palette. |
| `Nofiltr Back Office.dc.html` | Back office | 14 | Manager and owner web app. Paper palette. |
| `Nofiltr Guest Ordering.dc.html` | Guest | 9 | Guest phone, QR and kiosk surfaces. Paper palette. |
| `support.js` | (none) | 0 | Generated `dc-runtime` bundle. Supplies the `<x-dc>`, `<sc-if>` and `<sc-for>` custom elements the files need to render. Not ours, do not edit, do not port. |
| `.thumbnail` | (none) | 0 | WebP preview image emitted by the design tool. |

## How a screen is marked up

Each file is a single-page prototype. A screen is a block wrapped in a state
switch:

```html
<sc-if value="{{ s_register }}"> ... </sc-if>
```

So `s_register` is the canonical identifier for a screen, and the line numbers
below point at its opening `sc-if`. Line numbers are accurate as of the commit
that added this file; if they drift, `grep -n 's_register' <file>` finds it.

## Terminal: `Nofiltr Terminal.dc.html`

| Screen | Token | Line | Covers |
|---|---|---|---|
| Register | `s_register` | 76 | Menu grid, ticket rail, modifiers, discount, hold, fire, charge |
| Payment | `s_pay` | 184 | Tender methods, amount tendered, tip, split evenly / by item / by seat, receipt choice |
| Invoice | `s_invoice` | 288 | Full tax invoice, 80mm thermal preview, e-invoice IRN, reprint and duplicate |
| Tables | `s_tables` | 401 | Floor plan, zones, table state, transfer, merge, split seats, mark clean |
| Orders | `s_orders` | 491 | Order list across channels, order detail, event timeline, settle, reprint KOT, refund |
| Kitchen | `s_kitchen` | 578 | Kitchen display, station lanes, elapsed time, late flags, recall last |
| Reservations | `s_reserve` | 630 | Bookings by day, waitlist, seat now, notify, cover forecast |
| Till | `s_till` | 699 | Blind drawer count, cash drop to safe, paid out, drawer log |
| Shift | `s_shift` | 763 | Sales by channel, tenders, exceptions, staff and tips, X-report, Z-report |
| Refund | `s_refund` | 848 | Item selection, reason, refund destination, manager approval |
| Customer display | `s_display` | 906 | Guest-facing second screen, running total, resident artwork |
| Lock | `s_lock` | 947 | Staff code entry, open shift, kitchen note |

## Back office: `Nofiltr Back Office.dc.html`

| Screen | Token | Line | Covers |
|---|---|---|---|
| Dashboard | `s_dash` | 83 | Revenue by hour, needs-you-now, top sellers, channel mix, live floor, art wall |
| Sales register | `s_sales` | 188 | Daily sales register, Tally CSV, GSTR-1 JSON, PDF export |
| Item mix | `s_mix` | 265 | Sold, revenue, cost, margin, attach rate, menu classification |
| Menu editor | `s_menu` | 295 | Categories, menus, item editor, price, cost, station, channel availability |
| Inventory | `s_inventory` | 386 | Stock count, on hand, par level, wastage log, auto-deduct |
| Purchasing | `s_purchasing` | 446 | Purchase orders, suppliers, suggested order |
| Staff | `s_staff` | 506 | Weekly rota, publish, team and access, timesheet |
| Customers | `s_customers` | 571 | Guest list, tier, visits, lifetime value, usual order, recent visits |
| Promotions | `s_promos` | 642 | Happy hour, combo, BOGO, codes, loyalty programme, gift cards |
| Invoice ledger | `s_invoices` | 707 | Document ledger, GSTR-1, GSTR-3B, e-invoice IRN log |
| Payouts | `s_payouts` | 746 | Settlements, gross, fees, refunds, net payout schedule |
| Devices | `s_devices` | 804 | Print routing |
| Channels | `s_channels` | 834 | Aggregator orders MTD, revenue, commission, menu mapping |
| Settings | `s_settings` | 857 | Venue settings, save and discard |

## Guest: `Nofiltr Guest Ordering.dc.html`

| Screen | Token | Line | Covers |
|---|---|---|---|
| Menu | `s_menu` | 51 | Order-ahead menu, offer strip, categories |
| Item | `s_item` | 112 | Item detail, hero photo, allergens, modifiers |
| Cart | `s_cart` | 158 | Line items, upsell, subtotal, member discount, GST, packaging, promo code |
| Pay | `s_pay` | 207 | Payment choice, tip, GST invoice request, pickup slot |
| Track | `s_track` | 257 | Pickup code, live status, ETA |
| Receipt | `s_receipt` | 300 | Guest tax invoice, share |
| Table QR | `s_qr` | 359 | Live table bill, covers, already-paid-by-others, pay share |
| Loyalty | `s_loyalty` | 399 | Member card, tier, points balance, usual order |
| Kiosk | `s_kiosk` | 469 | In-store kiosk, eat in / take away, tray, pay now, start over |

## Notes

- Screen tokens repeat across surfaces. `s_pay` and `s_menu` each exist in two
  files, so always pair a token with its file.
- Counts: 12 + 14 + 9 = 35, matching the "35 Screens" figure on the cover page.
  Nothing is missing from the archive.
- The `Covers` column names what is on a screen so it can be found. It is not a
  spec. Screen specs are written lazily, per `docs/design/screens/README.md`,
  at the start of the phase that builds the screen.
