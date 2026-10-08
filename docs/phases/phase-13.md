# Phase 13 - The back office from anywhere

## Goal
The owner opens the menu, the floor plan, stock, staff and the reports from
home on their phone, through the same relay that carries guest orders, with
nothing more opened at the venue than today. It is off until the owner turns
it on, and the relay's operator can sell it: a venue key says whether the
relay will carry it (ADR-306).

## Depends on
Phase 11 (the relay and the tunnel) and Phase 10 (the back office).

## What stays true
- The counter never needs it. A venue with no relay, or with this off, is
  exactly as it was.
- The terminal, the kitchen display, the printers and the staff API stay
  unreachable from the internet. Only the back office bundle and the API it
  uses cross the relay, and only on a venue that asked.
- Nothing that moves money or opens a drawer can be done remotely: no
  payments, no refunds, no voids, no shift close, no printing, no setup.
- Every remote request is marked as such in the access log, with the
  staff member and the device.

## Tasks

- [x] **Task 1 - A venue key that allows the back office.**
  `relay issue-key VENUE_ID --remote` prints a key whose scope is signed in:
  `mzr_<venue>.<key id>.remote.<mac>`. A three-part key is what it always
  was: guests only. `Keys.Verify` returns the scope; the agent remembers it;
  `forward` answers 404 for `/backoffice/` and `/api/` on a venue whose key
  is guests-only, before anything is sent down the socket, so the relay is
  the billing switch and a venue cannot turn it on by itself. Tests in
  `internal/relay` and `cmd/relay`. Docs: `build/relay/README.md`.

- [x] **Task 2 - The tunnel carries the back office when the venue says so.**
  A new setting `remote_backoffice` (`MEZ_REMOTE_BACKOFFICE`, default false).
  With it on, the tunnel client passes a request whose path starts
  `/backoffice/` or `/api/v1/` through as it is, and everything else still
  goes under `/order/` as before. With it off, nothing changes:
  `/backoffice/` lands at `/order/backoffice/` and is a 404, as today.
  Tests in `internal/tunnel`.

  As built (ADR-307): `X-Mez-Via: relay` is set by the tunnel on **every**
  relayed request, guests' too, and whatever arrived under that name is
  removed. The setting on a guests-only key, or with no relay, stops the
  start. `GET /api/v1/tunnel` says `back_office`; health does not, since
  health is public.

- [ ] **Task 3 - The API holds a remote request to the owner's rules.**
  A middleware in `internal/api`, inside `requireDevice`, that acts only when
  `X-Mez-Via: relay` is present: the device must be a `back_office` device;
  the staff member, once signed in, must hold `menu.edit`, `settings.edit`,
  `reports.view` or `stock.manage` (a cashier or waiter is refused even with
  a right PIN); and a deny list answers 403 for the order, payment, kitchen,
  print, shift, setup and export-download routes. The access log line
  carries `via=relay`. Tests prove a terminal device, a cashier and a payment
  are all refused over the relay and all still work on the LAN.

  The relay carries no WebSocket (ADR-201: `Upgrade` is a hop header and is
  dropped), so `/api/v1/stream` cannot open from outside. Find what the back
  office does without its stream (the "Online" pill, live tables on the
  dashboard) and decide here: refuse the stream cleanly over the relay and
  have the back office say "Away from the venue" and refetch on focus.

- [ ] **Task 4 - The owner turns it on and sees where to go.**
  Devices screen, under "Ordering from phones": a "Back office from
  anywhere" card with the switch (it writes the setting and says a restart
  is needed, like the relay settings), the address
  `https://<venue>.<domain>/backoffice/` to open on a phone, and a reminder
  that the phone must be paired once with a code from this screen. Docs:
  a section in `deploy.md` after "Guests ordering from their phones", and
  `running.md`'s settings table. A walk by hand with `bin/relay` and
  `bin/mez`: pair a phone browser over the relay, change a price, see it on
  the till.

- [ ] **Task 5 - A remote session is harder to take than a LAN one.**
  Over the relay a PIN alone is not enough: a remote sign-in needs the
  device to have been paired within the venue's wifi or with a code made
  in the last ten minutes (the pairing flow already does this), the PIN
  lockout counts remote attempts separately and harder (3 wrong, then
  locked until a manager at the venue clears it), and a remote session
  expires after 12 hours. ADR before code. Tests.

## Done when
From a phone on mobile data, the owner opens `https://<venue>.mezpos.com/backoffice/`,
pairs it once, signs in, changes a price and adds a table, and the till at
the venue shows both within a second. From the same phone, the terminal at
`/` and the kitchen at `/kitchen` are 404, and a cashier's PIN is refused.
A venue whose key was issued without `--remote` gets 404 at the relay.
