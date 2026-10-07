# Known broken, left out, and not seen by eye

> What is known to be wrong, limited or unchecked, by screen and by ADR, newest
> first. Moved out of `STATE.md` on 2026-10-05 (rebuild Task 17h3) so that file
> stays short. **Not read whole.** Before changing a screen, find its entry here
> (search for the screen's name or its ADR) and read that one.
>
> A task that leaves something broken, left out or unseen adds or edits its entry
> here, and puts in `STATE.md` only what the next task must know. An item is
> struck from a "not seen by eye" list when someone has seen it in a browser, and
> not before: the walk of Task 17h2 (ADR-199) struck what it saw, and no more.

- **The photos in the backup** (ADR-275; phase 12 Task 4c). Seen on
  2026-10-07 in tests: to a folder, a stand-in S3 and a real MinIO and
  back, each photo held to the hash in its name both ways; the whole
  server leaving its photo behind as it stops. **Not seen**: hundreds of
  photos; the menu screen adding one with the backup running; Amazon's
  S3 or R2. A photo removed from the menu stays at the destination for
  ever; one deleted from the destination by hand is sent again within ten
  minutes. A long first copy of many photos delays the probe's next look
  at the destination, so the status can lag by as long as the copy takes. Unreferenced photos are still never swept
  on the venue's side (ADR-110), so they are backed up too.

- **Backups to a bucket** (ADR-274; phase 12 Task 4b). Seen on
  2026-10-07 in tests: against a stand-in S3, and against a real MinIO in
  Docker, where a 12 MB database went up in parts and came back whole, a
  wrong secret was refused, and the whole server ran, stopped and was
  restored. **Not seen**: Amazon's S3; R2; HTTPS; a slow line; a month's
  bill. A server started with no internet captures nothing
  for the bucket until it first answers, and what was sold meanwhile
  goes as one whole copy then: a disk that dies before that takes those
  sales. A line that drops after the start loses nothing. (A folder has
  no such gap, and a venue can only have one destination.) The status says
  "starting" for up to a minute before it says failing. The secret is in
  `backup.json` in the clear. The binary is 34 MB, from 19.5 MB before
  backups. No encryption options, storage class or session tokens.
  `AWS_*` variables in the server's environment are still read by the SDK
  and can redirect it. The race detector has not run (it needs cgo).

- **Backups to a folder** (ADR-273; phase 12 Task 4a). Seen on
  2026-10-07, in tests on Windows against real files: a restore from the
  folder alone holds every row, while the server runs and after it stops,
  across three starts; the whole server stopped at once leaves a backup
  that restores and migrates clean. **Not seen**: Linux at run time; a
  day of service (the nightly whole copy, the pruning after seven days,
  the log being folded in under a busy till); a drive pulled out and put
  back, on real hardware (a folder moved away and back was); a network
  share, and one that hangs rather than fails; a restore to an earlier
  time; the race detector, which needs cgo. After a destination comes
  back the status can say failing for up to five minutes, until
  Litestream tries again or someone backs up now. A destination given
  with a `\\?\` prefix or an 8.3 short name is not recognised as inside
  the data directory. Left out until
  4d to 4f: any way to set the destination
  but writing `backup.json` by hand and restarting, a status anyone can
  see (a failing backup is only in the log), and `mez restore`. Litestream
  is held at v0.5.15 because 0.5.16 and 0.5.17 do not work on Windows;
  this has not been reported to its authors. Its lines in the log are
  its own words, and a restore logs through Go's default logger.

- **The installer's shortcut opened a kiosk nobody could close** (ADR-272,
  fixed on 2026-10-07). The owner installed on the developer's PC and the
  till filled the screen with no title bar: the installer started Edge
  with `--kiosk`, as ADR-265 chose. It now opens a maximised app window
  with the usual buttons; full screen is a box in the installer, not
  ticked. Seen: Edge started with the new arguments has a title bar and
  its three buttons and opens maximised; with the old ones it has no
  title bar. **Not seen**: the new installer run, so neither the shortcut
  it writes nor the box by eye; Chrome; a machine with neither browser. A shortcut made
  by the installer before this keeps its kiosk arguments until the new
  installer is run over it.

- **The address as a QR code, and addresses by adapter** (ADR-271; phase
  12 Task 3f). Seen on 2026-10-07 in a browser against the built binary,
  at 1366 and 1024 wide, on a PC with one real adapter and two of
  Hyper-V's: the code drawn beside the address and changing with the kind
  of device; the two virtual adapters named small under it; the printer
  search saying it searched the one real network, where before it
  searched three. **Not seen**: a camera reading the code (it is the
  encoder the table codes use, and the test holds it to the address); a
  computer with two real adapters and its two bands; a machine with only
  virtual-looking adapters; Linux adapter names in the flesh. Left out:
  virtual is judged by the adapter's name and can be wrong either way (a
  wrong guess moves an address to small type or searches a network late,
  never hides one); a tablet's camera opens the address in its browser,
  not in a kiosk; the Devices screen in the back office does not show the
  address or the code, only the wizard does.

- **The setup token** (ADR-270; phase 12 Task 3e). Seen on 2026-10-07
  against the built binary: with curl, status locked without the token and
  unlocked with it, never unlocked from the LAN address, a post refused
  with none and with a wrong one; in a browser, `/` showing "Open mez from
  the installer to set up", the setup link opening the questions in a
  fresh window and in a tab already on the locked page, the token gone
  from the address and from no request's URL, the venue made, the token
  file deleted and the tab's copy forgotten. The installer compiles.
  **Not seen**: the installer run, so not "Open mez now" opening the link,
  not `mez setup-link` run by it as administrator, and not the link
  through Edge's kiosk arguments or through Windows with neither browser;
  the link printed to a real console; the file's ACL under the service.
  Left out: the desktop shortcut cannot carry the token, so an owner who
  unticks Open mez now must run the installer again or use an
  administrator's terminal; a program running as the same Windows user as
  the browser, or as an administrator, is not kept out and cannot be;
  Docker and a headless server still use `mez setup`.

- **The wizard's last step, and where the server is** (ADR-269; phase 12
  Task 3d). Seen on 2026-10-07 in a browser against the built binary, at
  1366 and 1024 wide: the step after the printer; this PC's LAN address
  first and its two virtual adapters' under it; the address changing with
  the kind of device; a back office code made, and a second origin opened
  at the address on screen pairing with it and reaching Staff Login;
  Another code giving a new one; Start selling landing on Open the shift.
  **Not seen**: a real second device; a server with no address to show; a
  code that could not be made; the address with the internet down; under
  the Windows service, where the port is 7777. Left out: the address is
  plain HTTP (ADR-257); nothing checks the firewall, the screen only
  says what to look at; `mez setup` in a terminal is still the way for
  Docker and a headless server.

- **Printer discovery, and the wizard's printer step** (ADR-268; phase 12
  Task 3c). Seen on 2026-10-07 in a browser against the built binary: the
  step after Create; a real search of the developer's three networks
  finding nothing and saying which it searched; a typed address refused
  for its shape and by the server for not being on a LAN; a test page of
  932 bytes arriving at a stand-in printer on 127.0.0.1:9100 and "Sent.
  Did a page come out?"; a second address that refused, "Not printed" with
  the queue's reason, and the first printer retired for it. The spooler's
  list was read for real by a test (five printers, all virtual, all left
  out). **Not seen**: a real printer found on a LAN or through Windows,
  or printed on; a found printer's row on screen; the search under the
  service's account; Continue without it retiring the printer (tested as
  logic). Left out: the search walks a /24 around each card and at most
  four cards, so a printer elsewhere on a larger network is typed in;
  IPv6; mDNS and vendor
  discovery; anything that tells a printer from another device with port
  9100 open; the back office's Printers screen has no search button;
  Linux and Docker list no installed printers.

- **The setup wizard** (ADR-267; phase 12 Task 3b; `setup-wizard.md`).
  Seen on 2026-10-07 in a browser against the built binary and an empty
  data directory, at 1366 and 1024 wide: `/` going to the wizard; each
  step's own refusals; the tax profile moving the currency and the year;
  Create landing the owner on Open the shift with the prefix set; the
  back office then open on the same pairing; a device by the LAN address
  told to go to the computer, and offered Pair once the venue exists.
  **Not seen**: any failed stage of Create (the server refusing an answer
  the step let through, a lost reply, an expired code, a refused prefix),
  which are tested as logic only; a screen under 1024 wide; a touch
  screen with no keyboard; the installer's shortcut opening it. Left out:
  what is typed is lost on a reload; a reload after Create needs `mez
  pair`; no preview of the first invoice number; ten currencies in the
  list; D24's tables and menu import; a US venue leaves with no rates set
  and cannot sell until Settings has them.

- **First run over HTTP** (ADR-266; phase 12 Task 3a). Seen on 2026-10-07
  with curl against the built binary: the status, the refusals by LAN
  address, foreign `Host`, foreign `Origin` and text/plain, a venue made,
  the second attempt refused, the code pairing a device and the owner
  signing in. **Not seen**: from a browser (no screen until 3b); under the
  service's account; a real DNS rebinding or cross-site page. Left out:
  (any user or program on the server's computer, or a forwarder on it,
  could make the venue until the owner had: closed by the setup token,
  ADR-270); Docker, a headless server and one behind a proxy cannot
  use it and keep `mez setup`; a reply lost after the venue is written
  leaves no code on screen (`mez pair`); no idempotency key; a US venue
  still cannot finish.

- **The Windows service and installer** (ADR-263 to ADR-265; phase 12
  Tasks 1, 1b and 2). Seen on 2026-10-07 by silent install on the
  developer's PC: the service running as `NT SERVICE\mez` with its database
  open, the event log's "mez is starting.", the three restart delays, the
  firewall rule, both shortcuts, an upgrade in place, a silent uninstall
  that kept the data. **Not seen by eye**, on a clean machine, from the
  downloaded installer: the SmartScreen box and the installer's pages; the
  shortcut opening the till full screen; a test print on the venue's
  printer, LAN and USB, as the service's account; `mez.exe` ended in Task
  Manager coming back within ten seconds; a second copy holding the port
  and "stopped on its own" at 5 seconds, 30, then 5 minutes; the PC
  restarted and mez up before sign-in; `mez pair` in an administrator's
  terminal still reading the database; Uninstall asking about the data
  twice, keeping it on No and deleting it on Yes, Yes. Not built: a check
  that a `-user` account can write the data directory or log on as a
  service; a firewall rule for a network marked Public; a shortcut that
  follows a changed port; signing (no certificate).

- **Security, left for the owner** (ADR-257, the first review): five things
  the review found that change what the product does, and so were not changed
  by it. **The LAN is plain HTTP**: device tokens, staff tokens and PINs
  travel in clear on the cafe's wifi until phase 12's TLS story; the till's
  network must not be the guests'. (A table's code as a lasting credential,
  also left here, is fixed by ADR-262: ordering to the kitchen and the bill
  need the phone's session for the sitting, which ends with the ticket;
  what remains: every phone's first order of a sitting is a request staff
  accept, staff can still accept a stranger's, and a round's charge and a
  receipt are read by the code and that round's or bill's id.) (The PIN
  lockout, also left here, is
  fixed by ADR-260: a third lockout in a day locks until a manager clears
  it, and powerful roles have six digits; what remains: a colleague can be
  locked out on purpose with seven wrong PINs, and a guesser pacing four
  tries a quarter hour never trips a lockout, which the six digits answer.)
  (Tips, also left here, are fixed by ADR-261: a tip over the venue's
  ceiling, 50% of the payment unless set, needs a manager holding
  tip.approve, and is listed in the audit trail; what remains: the ceiling
  is per payment, not per bill, and the printed Z-report does not show
  approved tips.) (The printer address and the Windows data directory, also left
  here by ADR-257, are fixed by ADR-259; what remains: a printer saved with
  a hostname or public address before ADR-259 keeps it until next edited,
  the send does not re-check the address, and the binary's own folder, such
  as `C:\mez`, is not restricted until phase 12's installer.)
- **Quality, noted and not changed** (ADR-257): the menu allows a negative
  modifier price and the projection refuses it, so "no cheese, less 20" can
  never be rung up; the cash rounding step is validated and stored and never
  applied; releasing a table closes each order in its own transaction, so a
  failure part way leaves some closed and the tables not cleared; one event
  whose payload the running binary cannot decode (after a downgrade) ends
  every stream of the venue on every reconnect; the relay's reset of an
  overrun stream is sent from the reading goroutine; a public id collision
  in `db/orders.go` is told from other unique violations by the driver's
  message text. In the web apps: the back office's "today" and quick ranges
  take the trading day to start at 04:00 in the browser's zone while the
  server honours the venue's `day_start`; `cmd/mez/main.go`'s `run` is some
  260 lines of wiring; the tender flow is written twice (`PayScreen.tsx` and
  `WaiterBill.tsx`); channel, payment-method and service labels and the rate
  formatter each exist in three to five copies that have drifted; the guest
  app repeats its header and line list in three screens. Ten eslint
  warnings (ADR-258) stand, each a deliberate pattern: `npm run lint` in
  `web` lists them.
- **Orders to collect at the counter** (ADR-253, `terminal-past-bills.md`):
  **not seen in a browser at all**: the To collect tab, the code and the
  kitchen's word on a row, the Call out card in the panel, Handed over and
  its toast. The real binary cannot make a paid order to collect without a
  gateway's keys. Checked by the type checker and the tests of its logic and
  of the server. Handed over has no undo. The panel shows no code for an
  order opened by its address before the day's list has loaded. The kitchen
  display shows the ticket as any takeaway, with no code. Nothing warns that
  codes are running out at a counter that never presses Handed over.
- **The pickup code in the back office** (ADR-252, `backoffice-floor.md`):
  **not seen in a browser at all**, at any width: the section under the
  tables, its three buttons, the warning, both confirms, the QR modal and
  the print modal's new lines. Checked by the type checker and the tests of
  its words and of the server. Nothing was printed, so the door's card on
  paper is unseen. The door's card is the size of a table's and is cut from
  the same sheet: put on a table by mistake it takes orders with no table.
  There is no poster size. At a venue with no gateway the code can still be
  shown as a QR and photographed; only the warning says not to.
- **Ordering to collect on the phone** (ADR-251, `guest-collect.md`): seen at
  390 wide in a headless browser with the API mocked: the landing, the menu,
  the cart, the order unpaid, paying, sent with its code, the strip, "not
  taking orders" and a wrong code. **Not seen:** against the real binary, a
  real gateway's page and the way back from it, through the relay (whether
  it carries `/p/<code>` is not checked), on a real phone, the ready and
  cancelled states, or with a screen reader. The phone remembers one order:
  ordering something else before collecting forgets the first one's code.
  The cart still says "Please tell a member of staff" about allergies, to
  someone who may not be in the venue yet.
- **Ordering to collect over the guest API** (ADR-250): asked by tests and a
  mocked browser only, never in the real binary, through the relay, or with
  a real gateway. A guest who paid has no receipt on the phone: a
  receipt is asked by a table's code. An order's id is all that guards its
  pickup code. The limits are by address and then by venue, reckoned and not
  measured: five addresses can read the venue's count away and three can
  fill the thirty that may wait unpaid; phones behind one carrier's shared
  address share its five orders in ten minutes. A wrong order id is not
  counted as a guess. An address that changes (one IPv6 network has many)
  is held only by the venue's counts, and can fill the thirty in minutes:
  guests are then told to wait for up to half an hour. While a paid order
  waits for a free pickup code (ADR-249) its payment still reads as open,
  with the gateway's page: a guest could pay twice, and nobody is at a
  table to notice.
- **An order to collect** (ADR-248, ADR-249): placed, paid for and sent, with
  a pickup code. A sent one is a ticket with no table on the `web` channel:
  it is on the terminal's Orders screen under To collect (ADR-253), and the
  reports show it as a counter ticket would. A paid ticket stays open,
  holding its code, until staff press Handed over: a counter that never
  closes them runs out of codes after a few thousand, and a paid order then
  waits, said in the log as an error. If no code is free a ticket is opened
  and left empty until one is. The code is three consonants or digits with a
  short list refused; no list in the venue's language. Migrations 00049 and
  00050 build `guest_rounds` and `gateway_charges` again: tested on seeded
  databases, never run on the deployed one. Back the database up first.
- **The venue's pickup code** (ADR-247): in the services and the staff API
  and, since ADR-252, the Floor plan's codes card.
- **Limits on guests** (ADR-246): tested through the handlers with the
  limits lowered; never against a room of real phones. **The relay limits a
  whole venue to 1200 requests a minute (`relay.DefaultLimit`), which about
  46 phones asking at their fastest reach: a full room through the relay
  is told to wait by the relay before any limit here.** Raise
  `RELAY_VENUE_RPM` on the relay, or make the screens ask
  less often. The figures are reckoned from what the screens ask, not
  measured. The counts are in memory: a restart forgets them. A table at
  its limit is in the log and nowhere an owner looks. The phone has its own
  words for an order refused this way and says nothing when a poll is: it
  asks again. A table's staff-seated rounds can still reach the kitchen 20
  times in ten minutes from one photographed code, until it is renewed.
- **The sheet of codes** (ADR-245): seen in the real binary as the page
  under its print styles at A4's width, eleven cards for a floor of twelve
  with one code turned off. **Nothing was printed, and no card was scanned
  by a phone.** Not seen: the browser's own print preview and where its
  pages break, a venue with a long name, a table with a long name, a printer
  that is not A4. The sheet is A4 only. A card says the venue, the table and
  the venue's host: not the code in letters, so a card that will not scan
  cannot be typed in. Reprinting one table means printing the sheet.
- **Table codes for the owner** (ADR-243, ADR-244): seen at 1366 and 1920
  in the real binary with `public_url` set: the list, Make the missing
  codes, the QR, Renew and Turn off with their confirms, a table with no
  code and Give it a code. A renewed code was asked for as a guest and led
  nowhere. Not seen: the card with no `public_url`, a refusal, a role
  without `settings.edit`, a keyboard or a screen reader, a floor of fifty
  tables. The QR was drawn and not scanned by a phone. Renewing or turning
  off a code ends, for that table, every order and receipt a phone could
  ask about through the old one, and the guest is told only that the code
  leads nowhere. Who renewed a code is in the log and not in the audit
  trail. Nothing prints yet (6c), and guests' requests have no limit (6d).
- **Ordering the same again** (ADR-242): seen at 390 wide in the real
  binary, one dish added onto a cart that had one of it. Not seen: a dish
  gone from the menu since, or its choices changed, being named in the
  cart; the cart at its most; a keyboard or a screen reader. Only the
  phone's latest order can be ordered again: there is no list of past
  orders (M10). If the cart is both full and missing a dish, only the
  missing dish is said.
- **A guest's receipt** (ADR-240, ADR-241): seen at 390 wide in the real
  binary, a bill paid by UPI through the staff API, with its real invoice
  number and the time in the venue's zone. Not seen: a discount, tax not in
  the prices, two payments, a credit note or a refund, a legal name that
  differs, the states with no receipt, a long invoice number wrapping, a
  screen reader. A phone that neither opened the bill nor tracked an order
  while the bill was open has nothing to ask by. The phone remembers one
  bill for a table, the last it saw. 45 days is a figure nobody chose but
  Task 5d. A renewed table code ends every receipt asked for by the old
  one. A credit note shows only as an amount. A tip is not on it. Keeping
  it is a screenshot: nothing prints, saves or sends it.
- **Tracking an order** (ADR-236, ADR-238, ADR-239): seen at 390 wide in
  the real binary, one order at a seated table moved by the kitchen's own
  API: with the kitchen, being made, ready. Not seen: accepted (a dish made
  at no station), cancelled, a recall, an order to two stations, the strip
  over the menu at each step, an order that paid first, a screen reader.
  A round sent before migration 00047 says "with the kitchen" and no more.
  A round whose every dish is made at no station stays "accepted" for good.
  Nothing says an order was taken to the table: no event records it, so
  "ready" is the last step, and "A member of staff will bring it to your
  table" is said whatever the venue does. The phone remembers one order,
  the latest: an earlier round at the same table is not followed. How often
  phones ask has not been measured against a full room or through the relay.
- **Splitting the bill** (ADR-232 to ADR-235): seen at 390 wide in the real
  binary with a made-up Razorpay key: the three choices, the stepper, a typed
  figure and each refusal of one, a waiting payment with "I am not paying
  this", giving it up, and the rest with another guest's share (written
  into the database by hand) taken off. **No share has been paid at a real
  gateway, and two real phones have not split a bill.** Not seen: a paid or
  failed share's line, `bill_claimed` and `bill_moved` on screen, a phone
  with storage refused, a keyboard or a screen reader. A page given up can
  still be paid; if the bill no longer owes it, that is an unapplied charge
  for staff. A share under the gateway's smallest charge is refused by the
  gateway, and says only that paying online is not working. A phone that
  cleared its storage has a new name, and its old open shares count against
  it until they expire. Paying by UPI is always for the whole of what is
  left: a share is online only. The split is not by item.
- **Paying online on the payments card** (ADR-231): seen at 1366 with no
  gateway, with one never paid through, and with a paid charge written into
  the database by hand; never at 1920. The time is in the browser's zone.
  Keys revoked since the last payment still show that payment.
- **Stripe** (ADR-230): no charge has been made and no webhook received. It
  was tested against a stand-in written from Stripe's documentation, and the
  real API only as far as it refusing a made-up key. Currencies without two
  decimals are refused. No API version is pinned. Cashfree is not built, and will not be while
  it needs a guest's phone number (ADR-254).
- **Paying first** (ADR-225 to ADR-227): in effect where a gateway is set
  and the owner chose "Before the order is sent" on the payments card
  (ADR-229; seen at 1366 with and without a gateway, saved once, never at
  1920 or by keyboard). The phone pays on the order's screen (ADR-228): seen at 390 wide, the
  open, failed, taken and paid states with charges written into the database
  by hand, and Pay itself as far as a gateway refusing made-up keys. **No
  round has been paid for at a real gateway, and no real phone has gone to a
  gateway's page and come back.** A phone that cleared its storage cannot pay
  for the order it placed. The cart does not say, before Place Order, that
  the order will have to be paid for. Nothing
  ends an unpaid round. A round's charge that can never be applied (its table
  out of service) stays open and is retried by every sweep, with only the log
  to say so. A paid ticket no staff opened waits for someone to close it. No
  real gateway has been paid for a round.
- **The gateway's webhook** (ADR-224): no call from the real Razorpay has been
  received, and the path was not run through the relay. Set wrong, it fails
  quietly: a 401 and a warning each time, and payments land by the sweep.
- **Paying through a gateway** (ADR-220 to ADR-223): "I refunded it" is a
  person's word: mez makes no refund and checks none. Resolved charges are in
  no report. **No real charge has been made.** The real Razorpay was reached once, with made-up keys, and
  refused them as it should. Not seen by eye: the gateway's page, coming
  back, and the waiting, paid, failed and taken-not-on-the-bill lines. The
  keys and `public_url` are set in the configuration or environment only. An unapplied charge is
  logged and listed by `charges.Unapplied`; no screen shows it until 4f4.
- **Paying by UPI** (ADR-215, ADR-216): the back office card was not seen at
  400 wide, for a venue not in rupees, or with its read failing. **No phone
  has scanned the QR (ADR-217) or opened the link**: it agrees with another
  encoder and a decoder reads it back. "I have paid" (ADR-218, ADR-219) was not
  seen with the amount differing from what is owed, with a refusal, in the
  waiter's frame, or clearing by a payment recorded on the tender screen. Two guests can each pay the whole balance.
  Not seen: a UPI app opening on the link, on any phone.
- **The bill on the phone** (ADR-214): a phone of the table's sitting reads
  that table's bill from anywhere (ADR-262). A table with two tickets shows the older.
  Nothing is shown once a bill closes, until Task 5's receipt. Not seen by
  eye: tax rows, a discount, On the house, Paid and Left to pay, nothing
  open, the failures.
- **Place Order on the phone** (ADR-213): no special instructions. A waiting
  order is asked about by sending it again every 8 seconds; Task 5 should
  replace that. A second order placed while the first waits loses word of the
  first. Not seen by eye: a declined order, any refusal, a slow or dropped
  connection, a reload while waiting, a real phone, the relay.
- **Table requests** (ADR-212): the chime cannot be turned off, and a terminal
  reloaded and not touched since is silent. Not seen by eye: the waiter's card
  and frame, the Table List's row, the sheet on touch, a refusal's words, the
  30 second repeat, two requests from one table, 1536px and wider, and the
  sound itself (counted, not heard).
- **A guest's order** (ADR-210, ADR-211): a request staff ignore stays listed
  until the shift closes. Rounds onto an open ticket are
  limited only by 300 requests a minute an address, until Task 6; the limiter has
  one bucket an address for every limit, so a tighter limit on ordering needs its
  own. The guest app does not call the endpoint yet. Not run through the relay or
  from a phone. A table with two tickets open gives a guest's round to the older.
- **What the walk found and left, for the owner to decide** (ADR-199):
  - The manager role a venue starts with has no `menu.edit`, `settings.edit` or
    `device.manage`: Arjun has no Menu pill and the back office refuses him Menu. The
    flow map says a manager has Menu and Reports. Give the default manager
    `menu.edit`, or change the flow map.
  - "Not printed N" counts every kitchen ticket at a station with no printer. A venue
    that runs on kitchen displays alone sees the pill on every screen, growing all
    day. Should a station with no printer queue a job at all?
  - The Z-report's slip (`ZPaper`, and the printout) has Net and Total before credit
    notes and the taxes after them, with no credit-note line between. The report
    beside it now says so on its Tax card; the slip does not.
  - On tax-inclusive prices the totals read Subtotal ₹200.00, CGST ₹4.76, SGST
    ₹4.76, Total ₹200.00, as if the rows added up. The invoice says "Taxable value".
  - The desktop's cash quick amounts show four of five, so the ₹500 step is never
    offered: a ₹200.00 bill offers ₹210.00 and not ₹500.00. The waiter's shows it.
- **Small things the walk saw**: a build with no version number whose commit starts
  with a digit reads "mez v8bfad97-dirty" (`versionText`); the connection is "Live" on
  Staff Login and the kitchen board and "Online" everywhere else; the Orders sheet
  names a takeaway order "Counter" where the rail says "Takeaway"; an approval lists
  everyone, also those who cannot approve (the server refuses them); the kitchen
  board has its script line twice at 1920 wide; refunds made before ADR-199 read
  "Not recorded" in Audit for good.
- **Not walked**: a card payment (the waiting card) on the desktop, a split by item
  or by amounts, a void, comp or discount, transfer and merge, delivery, a reprint,
  the tips pool, the Menu, Inventory, Staff and Settings screens (Arjun may not open
  two of them; nobody signed in as Priya), any width but 1920x1080 and 1366x1024.
- Left from 17h1 (ADR-197, ADR-198): `/shift/close` typed by hand still lets someone
  with `payment.take` count; the printed Z-report labels its device "Till"
  (`escpos/zreport.go`); the kitchen board's clock and a ticket's sent time are read in
  the device's zone, not the venue's.
- To walk again: the browser the earlier sessions used (Playwright's) was held by
  another process, so this walk used the Chrome DevTools one. `make check` cannot
  rebuild `bin/mez` while a server is running from it (Windows locks the file).
- Printers, Devices and Audit (ADR-195): the dev venue now has three printers that
  go nowhere (127.0.0.1:9197 to 9199), one retired and one set for receipts, so a
  receipt there queues and retries. Not seen by eye: a revoked or a quiet device,
  a refusal over the printers table, and Devices and Audit under 1280px.
- The Z-report and the close (ADR-194): the dev venue's first shift is now closed
  (Z-report 1, ₹282.00 over, counted by hand to try it) and a second is open with a
  ₹10,000.00 float. Not seen by eye: a width under 1920 (the slip under the report),
  a balanced drawer and one within its limit, a closing stock count on step two, a correction, a report from before the new figures, and Print a copy carried through.
- Settings (ADR-193): Floor plan, linked from its rail, is still its old layout (ADR-195). Not seen by eye: a width
  under 1920, a save, Leave carried through, the rail for someone without
  `device.manage`, and the Tax and Shift sections' cards beyond their headings.
- Staff and roles (ADR-192): no roster, shift column, phone or PIN column. An unsaved
  change to a role is dropped when another role or person is picked. After changing
  your own role the switches follow your old permissions until you sign in again.
  Not seen by eye: a width
  under 1920, a deactivated person shown and brought back, a person added, a new PIN,
  a role added, renamed and deleted, a refusal, and a manager's view (switches they
  cannot move).
- Inventory (ADR-191): no purchase orders or suppliers. A full bar is three times the
  low level, our own rule. After a count or a write-off started from the buttons
  beside the title, focus does not return to that button. Not seen by eye: a width under 1920, a retired ingredient
  shown, a count and a write-off carried through from the new buttons, History and
  Edit from a row's menu, the week's change against the week before (the dev venue
  has one week), the screen without `reports.view`, and the Recipes view after the
  change.
- Menu management (ADR-190): opening another item or leaving the screen drops an
  unsaved edit without asking, and an item changed on another screen while it is open
  is saved over. The list no longer shows each item's tax class, station, course or
  HSN (they are in the editor). No cost price, combos or drag to reorder. The dev
  venue has a retired "Test biscotti" in Cakes from the browser check. Not seen by
  eye: a photo chosen and saved, choices added to an item, Move up and down, a
  category added, renamed and retired, the Kitchen and tax tab's refusal, the
  Choices and Stations views after the change, and a link to `/menu?item=N`.
- An item's cost and margin (ADR-189): sales from before migration 00035 are never
  costed by dish; a dish with no recipe reads "Not costed". No CSV has cost. The dev
  venue now has recipes for Espresso, Cappuccino, Americano and Lemon tart and two
  orders rung up through the API on 2026-10-05; its Espresso and Cappuccino read "0
  of 2 costed" because one sale fell between the two migrations. Not seen by eye: a
  margin below zero, a range reaching before costs were kept (the note), a dish whose
  line was voided after a reopen, a failed read.
- Staff reports (ADR-188): no ratings, no customers. Tips are those taken, not the
  pool's shares. A tipped payment from before the days, voided in them, shows as a
  row below zero (it keeps the rows equal to the total). Not seen by eye: a guest's row, someone who only took payments, someone no longer
  on record, a range with nobody, a failed read, a width under 1920.
- Items and inventory (ADR-187): no purchase orders, no CSV for wastage. The dev venue now has four stock items and four write-offs keyed
  on 2026-10-05 to show the cards. Not seen by eye: a trend arrow (every item was new
  in its days), more than five write-offs, a day with none, the screen without
  `stock.manage` (no stock health card), a failed read, a width under 1920.
- Sales reports (ADR-185): no outlet or order type filter, no CSV for service types.
  "Total guests" counts only parties someone counted. Not seen by eye: a range of
  more than seven periods (the named bars), delivery on the trend, more than six
  categories ("Others"), a method that refunded more than it took, a failed read, a
  download carried through, From and To typed by hand, and "View all" scrolling to
  the items table. The range is not in the URL: a reload returns to seven days.
  In Safari and Firefox on a Mac a click on a pill may not focus it, and the shared
  disclosure can close a menu before the click lands; tried in Chromium only.
- The dashboard (ADR-183): bars are one colour (the report's hours are not cut by
  service type); "Total orders" is paid bills, not open orders; today so far is
  compared with the whole of yesterday; a table to clear counts as active. Not seen by eye: a low
  or out-of-stock row, more than five rows in a list, a day with nothing sold, a failed
  read, a cashier's dashboard (cards by permission), a bill after 10 PM,
  and a width under 1280px.
- The back office's frame (ADR-182): no search, no bell. Not seen by eye: the bar
  with the server down, Sign out carried through, a `g` chord, a width under 768px
  (the sidebar as a row), and a screen 900px tall or more at 1366 wide.
- The board's foot (ADR-177): no Auto advance, no Mark all complete, no Settings
  screen. The chime was not heard (the check was by eye): try it on a real display,
  and after the tab has been idle. The dev server missed a file change once (Vite on
  the I: drive): touch the file if a screen looks stale. Not seen by eye: grouping
  with two tables' tickets interleaved, and night at 1366 wide.
- Moving tickets (ADR-180): a role the owner made that both sells and sees the
  kitchen is not given `kitchen.work` by the migration; the owner ticks it on People.
- A cook's board (ADR-179): nothing leads from `/pass`
  to the rest of the terminal, or back for a manager who opens it by its path. Not
  seen by eye: a role made by the owner landing there, a wall display and a signed-in
  cook at one station at once, and a manager at `/pass`.
- Coming next (ADR-181): every open order is projected on each board read; not
  timed with many open. Not seen by eye: more than five (the "more behind these"
  line), an order with no table, two courses held on one order, and the list going
  when a course is fired.
- Today's stats (ADR-178). The day is read whole on each board
  read; a busy station has not been timed. Not seen by eye: the one-line version
  under 1536px (the browser would not resize), and a slowest ticket with several dishes.
- The kitchen's board (ADR-176): a new ticket has no Mark ready of its own. The clock is the device's
  zone, not the venue's. Not seen by eye: the delivery icon, a voided line, a line's choices, a Waiting timer, a failed action, a column long enough to scroll at
  1080p, and two displays at one station.
- Start preparing (ADR-175): the ticket index cannot say what is in progress without
  the orders. Nobody is named as the one who started a ticket.
- The shift summary (ADR-174): no ratings, no Print summary, no area. Not seen by
  eye: several tables' bars, "No table", a table gone from the floor, a failed read,
  and someone who has done nothing.
- One person's shift (ADR-173): read whole from the shift's
  orders on each ask; a busy shift has not been timed. A refund is not taken off
  "sales". A table the cashier opened counts for the cashier, not the waiter who
  served it.
- The waiter's bill (ADR-172): no Print bill before payment, no QR, no Wallet, no
  keyed tip. A toast sits over the foot's buttons for its few seconds. Not seen by eye:
  By item and Custom carried through from here, Change the split, a stale split,
  the keypad for cash, a refusal, Print receipt, an order with
  no table ("Close order"), and three or more shares (the column scrolls).
- Transfer and merge (ADR-170): a merge of several tables is one request each, in
  turn; if one is refused the ones before it stand. No note about QR sessions, no
  "Notify the kitchen". Not seen by eye: a refusal, several tables joined at once, join
  and let go in one go, a floor of more than sixteen other tables (the tiles scroll).
- The table's screen (ADR-169): a table with several orders is the floor's; a free
  one is seated and one to clear is cleared from the waiter's home (ADR-171). Notes carry no time or name (a note keeps neither). Every course wears the same
  icon. The order
  screen's back arrow returns here for an order at a table (ADR-171). Not seen by eye: a
  joined table ("Merged"), a saved order, a refusal, and a table gone from the floor.
- Course View (ADR-167): the design draws the tab's name and nothing under it, so its
  layout is ours. A waiter fires only from here (ADR-168). Not seen by eye: a line
  with no course, a course "To send", a Ready line, three courses, a saved order.
- Confirm and send (ADR-166): no ticket number (the server gives it on sending), no
  choice of printer or screen, no "Notify me", no Undo. The ticket's face is Archivo,
  not a monospace. Not seen by eye: an order note, a line's
  note and choices on the ticket, a refusal, and a ticket longer than the modal.
- The waiter's order screen (ADR-165): no Add More (the menu is beside the rail). The item modal is D04's, not
  T02's sheet with a photo on top. The card's "+" is terracotta, as on D03. A cleared
  line stays on the rail struck through, as a removed one does. Save leaves a table
  order open and returns to the tables; only an order with no table is parked (ADR-168).
  Items added and not sent stay unsent after Save, and the table's card does not say so. Not seen by
  eye: a new order with no table ("Pick a table"), takeaway and delivery chosen in the
  bar, a joined table's title, a failed Clear, a menu with nothing popular, the bar
  with the server down, and "Bill and pay" carried through.
- The waiter's home (ADR-163): every table is listed, not the waiter's own (no table
  is assigned to anyone), so the line reads "the tables", not "your tables". No bell,
  Table Requests, QR Orders (phase 11) or Reserved (out of scope). Active Orders counts
  every open order at the venue and opens the day's orders. A waiter has no status
  bar: no clock, venue or version on screen; the top bar says Online or what is wrong
  (ADR-164), without the retry countdown; seen by eye Online and with the server
  stopped. Not seen by eye: a joined table's card, a floor of more
  than twelve tables (the cards scroll), a failed read, a width under 1280px (three
  columns, then two), and Print queue opened from More.
- The refund modal (ADR-162): not seen by eye are a refund by amount, a wrong PIN, and a bill with nothing left.
  At 768px high the modal scrolls to reach Continue. As a refund is issued the modal
  asks for one more preview, which the server refuses (409) as it closes: nothing shows,
  but it is a wasted request. A refund larger than any one payment is made in parts
  (ADR-084); the modal says so and does not split it itself.
- The Orders screen (ADR-161): not seen by eye are a voided row, a bill
  from before today found by its number, a reopened bill and Take the balance, a void
  or a reprint carried through, a held order opened from here, a day with no orders,
  and a failed read. A bill paid and invoiced but not yet closed (New Order not tapped)
  reads "Open". No cashier or waiter names, no Filters, no Export, no Email (left out).
  An order from before 04:00 shows its date and wraps in the Time column. The list
  asks again on every order event from any till.
- The day's orders (ADR-160): the list projects every order it names on each ask, as
  the open tickets list does; a day of several hundred orders has not been timed. It has
  no cashier or waiter names and no payment method (the panel reads the order itself).
  An order closed with every line voided reads "voided"; no command voids an order as
  such. The day start is read as it stands when asked.
- Split and the payment states (ADR-159): not seen by eye are a stale split, a guest
  with nothing to pay, taking a guest away, a tip on a card payment's waiting card, and
  a server's own refusal (the one seen was a dropped connection). At 768px high the
  unassigned items sit below the fold of the mode column, which scrolls. Two lines of
  the same item have the same spoken name on their "Give" buttons. A split made and its
  shares paid are lost on a reload, as before. No QR and no card machine (left out).
- The payment screen (ADR-156): not seen by eye are a tip keyed on the keypad, a cash
  refusal, a discount given from here, a table order's title with a joined table, and "Issue the
  invoice first". At 1366x768 the payment step scrolls to reach the keypad.
  After the review: the quick amounts count the tip ("exact" is the bill and the tip),
  a tip is cleared when a split is taken or another share picked, and an amount field
  is named with its amount; typechecked and unit-tested, not tried in a browser. Left:
  on a tiny bill two tip percentages can round to the same amount and the lower one
  shows pressed; the last change shown stays until the next cash is keyed; tapping the
  method already chosen clears what was keyed (as before).
- Audit (2026-10-03): `FormModal` was typechecked and tested, not tried in a
  browser; look at one save, one refusal and Enter in a two-field modal. `mez
  setup` pairs a till by default, so a venue set up with defaults reaches the
  back office with `mez pair -device-kind back_office` (ADR-144); phase 12's
  wizard should pair it. Setup's closing line and expiry now follow the
  device kind and the venue's zone.

- Redesign (ADR-128): screens are restyled through the primitives only.
  The specs' layouts were not re-read against the mez screen files,
  only their vocabulary and rules. Not seen by eye: card and
  UPI tender, split, a menu card with a photo, the
  discount, note, approval and tickets sheets, a past bill and its
  refund sheet, the print queue, closing stock, floor states other than free and ordered, move and
  join, the kitchen's waiting tone, order and line notes, a voided
  line and Recall. In the back office: menu choices, recipes, roles, tips, printers, and the
  sales screen after the charts turned green. Charts are one colour;
  the design's green, terracotta, mustard order waits for a chart with
  more than one series. The
  drawer count's face values sit right-aligned in their column, as
  before the redesign. NumPad keys use `sunken`, a stand-in on night that matches
  the canvas: no night screen has a keypad yet. Menu table cells wrap at
  1366px (Task 7).

- Devices (ADR-140): revoking a device does not close its open live stream; it
  is refused at its next connect, and every request it makes is refused at once.
  A revoked till's name in the hub is the one it connected with. Rename and
  revoke carry no `Idempotency-Key`. A device is "last seen" at most once a
  minute. The pairing code is shown once and cannot be shown again from the
  list (only its hash is kept).
- Stations and courses view (ADR-139): it cannot show whether a station has a
  printer (that is `settings.edit` data), so a retire says the printer follows
  and the toast says whether it did. A kitchen display set to a retired station
  still shows its board and is set to another by hand (ADR-134). Ordering is by
  arrows, not drag.
- Shift, tips and receipt screen (ADR-138): the receipt preview is of a made-up
  bill and shows no credit note; it was checked against the layout, not a
  printer. A tip weight for a renamed or removed role is not shown and drops at
  the next save. The preview asks the server on every pause in typing.
- Tax and numbering screen (ADR-137): the live preview of the next numbers
  needs a second ask per pause in typing; the rounding step is saved but nothing
  applies it (ADR-132). The profile card's "leaves N items without an HSN" shows
  only after a save, not as a draft. A tax class's retire refusal names eight
  items. The shell's navigation still drops a dirty draft.
- Venue settings screen (ADR-136): the "Leave without saving?" prompt is not
  seen by eye (only a second section can raise it, from the rail), and the
  shell's navigation drops a draft without asking. Timezone suggestions come
  from the browser's list.
- Receipt template (ADR-135): no screen yet (6i). The till's on-screen invoice
  preview does not show the header and footer, and no logo (ADR-079). A
  duplicate of an old receipt prints with today's template. Not tried on a
  printer: tested through the layout decoder at both paper widths.
- Stations and courses (ADR-134): no screen yet (6j). A retired station's
  kitchen display keeps its board and must be set to another by hand. Retiring
  a course leaves a gap in the numbers until the next ordering. Server and
  item form only, not tried in a browser.
- Rules (ADR-133): no screen yet (6i). A renamed role orphans its tip
  weight, and nothing checks. A day start change re-buckets past days in
  the reports, audit trail and CSV (events are placed by time); only
  shifts keep the day they opened under. Not tried in a browser: server
  only, Go tests and the HTTP round trip.
- Numbering (ADR-132): the cash rounding step is saved but no sale or
  payment applies it yet (a payment path owes that). The financial year
  start cannot move once a number is issued. A tax profile change does
  not start a new invoice series; one series spans it (ADR-131).
- Audit (ADR-117): discounts and refunds recorded before Task 4c show
  "Not recorded" for who authorised them. The Person filter lists active
  staff only (it reads `GET /api/v1/staff`); a deactivated person's rows
  show by name but cannot be filtered on. The audit screen has no CSV
  button yet (the route exists, ADR-119). The analysis (ADR-122) sums
  what each correction was worth to its bill, not money out of the drawer.
- CSV (ADR-119): each sheet reads the range again. The Tally files skip
  the formula guard and the byte order mark the report files have.
- Sales screen (ADR-120, ADR-121): tried with one day of sales and no
  credit notes, refunds or discounts, so the heatmap was seen with one
  cell only (its layout over many hours is tested in vitest, not by eye).
  A CSV is in the server's order, not the table's sort.
- Corrections (ADR-123): the browser run had one person, so a row for a
  guest, "not recorded" or someone removed was not seen by eye. Back from
  the trail returns `/sales` to seven days. A deactivated person's link
  shows "Someone no longer active" in the Person box (not seen by eye).
  A sales download and a corrections download can run at once. The range is not in the URL, so a reload returns to
  seven days. A CSV has not been opened in Excel itself. Left open past
  04:00, the range stays yesterday's and no quick range shows pressed.
- Reports (ADR-118): the screen lacks 5d to 5f. A range projects every order
  it touches from its first event; a year at a busy venue may take seconds.
  Items and categories are named as the menu stands now. The day start is
  the venue's setting as it stands when asked (ADR-133).
- Stock cost (ADR-124): movements from before migration 00028 have no
  cost. Nothing shows a cost's history; a new cost is for stock moved
  from then on.
- Closing stock (ADR-125, ADR-126): a shift with no closing count is not
  listed. Items are named as they are now. The browser run had one close
  on one day; several shifts and a partly costed item were tested in Go
  and vitest, not by eye. No subtotal per shift on screen.
- Stock (ADR-111 to ADR-114): The
  closing count cannot be skipped while items are marked. A close uses today's
  recipes, not the ones when the line was rung up. A sold movement needs a
  real staff id: the guest channel (phase 11) must supply one. A movement
  entered by hand after the closing drawer count records no shift (the
  closing count names its own, ADR-114).
- Photos (ADR-110): `<data dir>/photos/` is backed up beside the
  database since ADR-275; unreferenced photo files are never swept; the
  guest app does not show photos yet.
- Sold out (ADR-109): only the back office can mark it, not the till; the
  server does not refuse a sold-out item rung up (AddLine is unchecked);
  a till with a wrong clock ends it at the wrong time.
- Menu editor (ADR-106, ADR-107): no `Idempotency-Key` on
  its routes, as with printers. Duplicate item names are allowed.
  Stations, courses and tax classes have a write path (ADR-134). An item's
  groups save in a second request after the item (ADR-108). A negative
  choice price is not checked against the items offering it. A venue
  whose tax profile changes to one needing
  HSN keeps items without one until each is edited; the tax settings list
  them (ADR-131). The table scrolls sideways below about 1100px of content width.
  Retiring an item shows no "open tickets keep it" note. Form controls
  use the shared `inputClass`, which does not set the 16px control size
  tokens.md asks for (the printers form has the same gap).

- Shell (ADR-105): tried as owner and manager (Task 4b); a cashier sees
  the dashboard alone (vitest). Printers, floor and Z-reports still carry
  their own "Back office" link above the title, now redundant with the
  navigation. Permissions changed mid-session reach the shell at the next
  sign-in or reload.

- Carried from phase 04, owned by no task yet:
  - The done-when script against the real binary. The same loop now runs
    in process on the real stack (ADR-092); the binary itself is left for
    phase 12's installer smoke test.
  - A `float64` lint rule for `internal/domain` (`docs/02-domain.md`).
  - A guard that fails the build when `order.State` changes without a bump
    to `order.ProjectionVersion`; today a forgotten bump serves stale
    snapshots.
- Exports (ADR-103, ADR-104): not yet imported into a real TallyPrime;
  its ledgers are mapped by name once in Tally. Shifts closed before
  migration 00020 have no files. A shift whose files cannot be written
  does not close, as with its Z-report.
- Z-report (ADR-097 to ADR-100): categories are the menu's when the
  report is built, not when the item was sold. The printout has no
  by-item lines; the screen does. The till shows only the report its own
  close issued, until reload; the back office list was typechecked and
  API-tested, not tried in a browser. No screen issues a correction yet
  (the API does). A report that cannot be built blocks the close; there is no
  way round that short of fixing the data.
- Tips (ADR-101, ADR-102): hours are typed in at pooling (no timesheets
  until phase 10); a pool is once per shift with no correction; the rule
  and weights are set through the rules route (ADR-133), with no screen yet (6i). The browser run had
  one staff member, so a multi-person or by-hours pool was tested in Go
  and vitest, not by hand; staff can now be added (Task 4a).
- Shifts (ADR-093 to ADR-096): terminals learn of a shift opening,
  counted or closed by polling every 30 seconds, not the stream. Opening
  needs shift.open (ADR-152) and closing shift.manage: a cashier who cannot
  close is told to ask a manager, with no override sheet. No till screen for cash in or out yet (the API has
  it). The day start is a venue setting (ADR-133). A payout larger than the drawer is
  not refused. Closing leaves open tickets to carry into the next shift.
  A counted drawer cannot be recounted. The variance threshold is set
  through the rules route (ADR-133), with no screen yet (6i). Denomination
  tables exist for INR, USD, EUR and GBP only; others count one total.
- `AddLine` is not checked against the menu (price, tax, modifier min/max);
  the menu is now written over HTTP (ADR-106), so this matters more. GST place of supply is always
  the venue's state, so no IGST. `mez setup` cannot finish a US venue.
- `orders.table_id` has no foreign key (ADR-085). Guests left closes and
  clears in two steps, not one transaction; a retry repairs it (ADR-087).
- Nothing stops seating a dirty table over the API; the floor only offers
  Seat on a free one. The plan and the list share `/tables`: a reload returns to the plan.
- A table with two tickets cannot be moved from the floor; the timeline
  names tables by number (ADR-088).
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
- A bill printed before payment has no number: numbers come with the
  settling payment (ADR-046, ADR-078). Task 4's pre-payment bill layout
  must say it is not a tax invoice.
- A split lives only on the tender screen (ADR-077); by guest picks seats
  there until lines carry them (phase 08).
- The server does not cap a tender's `reference`; the till caps it at 40.
- Printing is ASCII only (ADR-079): non-Latin item names print as "?".
  No physical printer has run the driver yet.
- The Windows spooler's write path is untested on a real printer (ADR-080).
- The cash drawer opens only with the settling receipt, not on a cash part
  payment (ADR-081).
- The ticket rail marks a line Ready but not sent or held by course. No
  `collected` step (ADR-090). A display's station is per device; a void
  on a sent line reaches it within 20 seconds, not at once (ADR-091).
- The till refunds to one payment per credit note (ADR-084); past bills
  search only exact numbers and invoice sequences.
- `make build` does not build under cmd or PowerShell; for the browser,
  `CGO_ENABLED=0 go build -tags frontend -o bin/mez ./cmd/mez`.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `make dev` (ADR-144) prints a pairing code for each; pair
  in the browser, sign in as the owner or a demo person and count a float to
  open the till. A code for the wrong app is refused but its device stays
  (ADR-068).
- The tables screen (ADR-155): not seen by eye are clearing a table, Guests left, join
  and part, a table with two orders, the paying and to-clear states, a held order, the
  Notes tab with notes, a refusal, and the Terrace.
  A floor of seven or more columns scrolls sideways at 1366px.
  After the review: Seat waits for the floor, the order is read again each time its
  table is picked, and Send waits while this till still has items going to that order;
  those three and the tabs' arrow keys were typechecked, not tried in a browser. A move
  or join keeps the party's table as it was when the mode began. A tile's spoken name
  leaves out what it owes.
- The item modal (ADR-153, ADR-154): a note refused after its line was added is lost
  with a toast. A tap meant for a card's "+" that lands on the card opens the modal. Not seen by eye: a photo, the Popular pill, a refusal, a screen under 1024px.
- The order screen (ADR-151): not seen by eye are a photo card, a sold-out card, a
  held order's banner, a delivery order, a discounted order's totals and a line the
  kitchen has seen (its bin and plain quantity). The diet filters reset on a reload.
- The open-shift screen's sold-out card was not seen by eye, and its float panel and
  buttons sit below the fold at 768px high (ADR-150).
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works, at
  `C:/Program Files (x86)/GnuWin32/bin/make.exe`). `make check` does not
  rebuild the web bundles: run `make build-web` before trying the browser. `make
  generate` needs sqlc v1.31.1 and can dirty `go.sum` with goose's test
  modules: restore it with `git checkout go.sum`. `bin/mez` has no
  `.exe`, so kill a stray server with `taskkill /F /IM mez`.
