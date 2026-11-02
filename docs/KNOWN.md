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

- **The tablet locked to mez** (ADR-420, Phase 18 Task 6b; 2026-11-02).
  Seen on an Android 13 tablet emulator: pinned when not the device owner;
  as the device owner, Home, Recents and the shade staying in mez, a reboot
  and a reinstall landing in the till locked, the Wi-Fi panel inside the
  lock, "Leave mez" to the tablet's launcher and Home back to the till
  locked, "Release it" undoing the kiosk. **Not seen:** a real tablet;
  Android 10 to 12, or 14 and on; a pinned tablet's reboot; the Wi-Fi panel
  joining another network; the kitchen display locked; "Leave mez" on a
  pinned tablet, or on one with no home app but mez; Android 17's local
  network grant. **Left as is:** while the PC does not answer, "Pair again"
  needs no PIN, and Android's Wi-Fi panel has its own way into Settings
  (less a factory reset, safe mode and a new user), let run in the lock
  while the panel is open (ADR-420's Cost); a kiosk takes a
  computer once (`adb shell dpm set-device-owner`), and its steps are in
  ADR-420 until Task 10 puts them in `hardware.md`.

- **The tablet app's way out** (ADR-419, Phase 18 Task 6a; 2026-11-02).
  Seen on an Android 13 tablet emulator, two fingers through its gRPC
  controller: the owner alone listed, a wrong PIN refused, her PIN opening
  Android's settings and Back returning to the till, the PC down saying
  so. **Not seen:** a real tablet's two fingers; Android 17; the kitchen
  display; the lockout's words. **Left as is:** as made, only the owner
  may let a tablet out (a manager's role is given "Pair and remove
  devices" in the back office); the way out needs the PC.

- **The tablet app finding its PC by itself** (ADR-418, Phase 18 Task 5c;
  2026-11-02). Seen on an Android 13 tablet emulator, its question passed
  to the real announcer by a relay on the PC: the till followed the PC to
  a new address with nobody touching it; an impostor answering `mez.local`
  refused with the screen untouched and left alone after; from "Can't
  reach the mez PC" at start to the till at the PC's new address; after a
  review, the till followed the PC 38 s after it moved (two missed ticks).
  The query itself run on the PC against the real announcer. **Not seen:**
  a real tablet on a venue's wifi (an access point that filters multicast,
  or isolates clients, would stop it: then "Pair again"); Android 17; the
  kitchen display; a renderer lost mid-carry. **Left as is:** a health
  request every 15 s per tablet while the till shows; a tablet whose PC
  never named its venue does not look; a tablet that loses power while
  WebView's storage is unwritten can lose its pairing (seen with the
  emulator killed hard: LevelDB drops the rest of a damaged block); a
  device on the venue's subnet flooding `mez.local` answers can keep the
  real PC out of the four asked, so the tablet does not follow by itself
  ("Pair again" still does): the fix, when wanted, is to set aside
  addresses whose health fails too, so later ticks reach further.

- **The tablet app's proof before a carry** (ADR-417, Phase 18 Task 5b;
  2026-11-02). Seen on an Android 13 tablet emulator: an impostor
  answering health with the venue's id given nothing; the real PC at a
  new LAN address proven and carried to; a proof across the emulator's
  NAT refused. **Not seen:** a real tablet; the kitchen display; a carry
  given up at its ten seconds. **Left as is:** a NAT or port forward
  between tablet and PC defeats the proof (no carry, pair again); an
  address typed as `mez.local` is not carried to; an impostor asked
  learns the device's public id; whoever holds a copy of the database (a
  backup, the mez Cloud mirror's) holds the proof's keys (ADR-417's Cost).

- **The tablet app's pairing carried to a new address** (ADR-416, Phase 18
  Task 5a; 2026-11-02). Seen on an Android 13 tablet emulator: a till
  paired and signed in, its PC moved to another address, "Pair again",
  the new address typed: the till at its sign-in screen as the same
  device, no code (twice, the second time to a port never opened, after
  the venue was learned from the till's page opening); another venue's
  mez given nothing. **Not seen:** the kitchen display's carry (its
  station and board settings with it); a scanned link carried (the same
  path as typed); a renderer lost mid-carry; the five-second give-up; a
  real tablet; Android 17. **Left as is:** a move signs the staff out; "Pair again"'s question still
  says to pair again from the Devices page, where the same venue's PC now
  needs no code (Task 10's words); a venue made by `mez setup` while the
  server runs is named by health only from its next start; the venue's id
  is no proof of the venue (ADR-417 adds the proof, 5b).

- **The tablet app's scanner** (ADR-415, Phase 18 Task 4b; 2026-11-02).
  Seen on an Android 13 tablet emulator whose back camera was fed a QR
  picture drawn by mez's own encoder: a kitchen display and a till paired
  by scanning, the camera refused (the words, Open Settings), a QR that is
  not mez's refused with the network words. **Not seen:** a real tablet's
  camera (focus, glare off the PC's screen, a front camera on a stand); the
  Devices page's own QR on a screen (the emulator cannot reach the PC's LAN
  address here, so the picture carried `10.0.2.2`); Android 17 since 4a;
  the tablet turned while it scans; a camera that stops drawing (the
  8-second check did not fire on the emulator, whose frames flowed).
  **Left as is:** the emulator's picture camera shows part of the
  picture, stretched, and stalls with a second stream: the code is placed
  in the picture by trial (`poster.sh` in the session's scratch, ADR-415).

- **The tablet app's address** (ADR-414, Phase 18 Task 4a; 2026-11-02).
  Seen on an Android 13 tablet emulator: "Pair this tablet" on first run,
  every refusal's words, an address refused while mez was stopped and at a
  port where something else answered, a till paired by typing its address
  then the code, a kitchen display by typing its address with the code,
  the kept address opened on the next start, "Pair again" asking first.
  Seen on Android 17 before the health check was added: the address first,
  then the local network page, the code held across Android's prompt.
  **Not seen:** Android 17 since the health check; `mez.local` (the
  emulator cannot resolve it); a real tablet; a debug build seeded with
  `-PmezUrl` after "Pair again". **Left as is:** typing is the only way in
  until 4b brings the camera; a code is lost if the app is closed while
  Android 17's prompt is up (typed again on mez's pairing screen); an
  address is refused while the PC is off.

- **The tablet that stays a till** (ADR-413, Phase 18 Task 3; 2026-11-02).
  Seen on an Android 13 tablet emulator: full screen, the bars back by a
  swipe and gone again, `KEEP_SCREEN_ON`, Back between the PC's pages and
  never out of the app, the till's layout unmoved at 130% font size, "Can't
  reach the mez PC" and the till back by itself, a renderer crashed once
  (back at the till's PIN screen) and twice ("The screen stopped", Try
  again), the kitchen chime after a reload with no tap (one ring, the app's
  audio stream started), `confirm()` as Android's dialog, a link to the
  internet refused. **Not seen:** any real tablet, its speaker, and a
  maker's Android that may keep its bars or word Back otherwise; Android 17
  since these changes (the permission's pages are as Task 2 left them);
  Android taking the renderer for memory rather than a crash (the same
  path, `didCrash` false); a pinch (zoom is off in the settings, not
  tried). **Left as is:** a renderer death signs the person out, as a
  reload of a browser tab does not; any WebView error but the four "no
  answer" ones keeps WebView's own error page; the back office in the app
  opens no file chooser (CSV import, an item's photo) and follows no link
  off the PC.

- **The tablet app's shell** (ADR-412, Phase 18 Task 2; 2026-11-02). Seen on
  an Android 13 tablet emulator: paired, signed in, a shift opened, an order
  sent. Seen on an Android 17 emulator: the local network page, Android's
  prompt, both refusals, Open Settings, and Allow opening the till. Not seen:
  any real tablet; Android 10 to 12 and 14 to 16; Settings walked to Allow
  (Android's Settings app crashes on that emulator, ADR-412); a maker's
  Settings, which may not say "Permissions" and "Nearby devices". What it left
  for Task 3 is done (ADR-413). A release build shows "No mez
  PC yet" until Task 4. Seen in passing on the till, not the app's: with
  the keyboard up (about 420 tall), Open the shift's action bar sits over
  its denominations, as in any tablet browser.

- **The audit re-run** (Phase 17 Task 33; 2026-11-01). Broken, found by it:
  a refund's approval opens scrolled with the cashier chosen (TBS-21,
  TILL-26); a percentage discount is keyed from the right and its approval
  does not restate it (TILL-25). 55 more, and what each screen still owes
  of the first audit, are in `docs/design/ui-audit-2026-10-31.md`,
  "Re-run, 2026-11-01", with what the re-run did not see. TBS-20, every
  waiter payment ending on "Maximum call stack size exceeded", is fixed
  (2026-11-01, `unsettledHold`); it is in the v0.2.0-beta.1 installer and
  the untagged one of 2026-10-31, not in those built after the fix.

- **The terminal in parts** (ADR-408, Phase 17 Task 32; 2026-11-01). Seen in
  a scripted Chromium on a demo venue at 1366 and 1024: the till's boot (its
  own files, then the warm about 100 ms later); with every script fetch
  blocked after the warm, opening the shift, the pay screen, the day's close
  and the Z-report route, with no fetch tried; the kitchen display, its
  station picker and its board, fetching only its four files across a
  reload; a waiter's home whose file was blocked before the warm and on
  opening, showing the region's error inside the shell, no loop, and both
  Try again (a second on, a fresh fetch) and Reload bringing it back, the
  session kept. **Not seen:** a waiter's table and
  shift summary, and the cook's board on a till, drawn (their files were
  fetched by the warm); the 300 ms fallback where a browser has no idle
  callback (older Safari); a real upgrade under an open till (the blocked
  fetches stand in for it); Try again in a browser that keeps a failed
  module fetch (Chromium does not). **Left as is:** the back office is still
  one 652 KB file.

- **One casing** (ADR-407, Phase 17 Task 31d; 2026-11-01). Seen in a
  scripted Chromium on a demo venue at 1366 and 1024: the sign-in screen's
  "Staff login", the register's "All items" and "Send to kitchen", the pay
  screen's "Split bill", "Back to order" and "Add discount", the floor's
  "Floor plan", "Table list" and "All areas", the back office's "Add new
  item" and "Sales reports". **Not seen:** the waiter's screens ("Active
  orders", "New order", "Order status", "Group by course", "Add item"), the
  kitchen's "New orders" and the item reports' head. **Left as is:** the
  brand's script lines in Title Case, as display art; a label in a string
  that is not drawn as JSX text (an aria-label, a toast) is not judged by
  the test.

- **One keypad, one input, one switch, one pill** (ADR-406, Phase 17 Task
  31c; 2026-11-01). Seen in a scripted Chromium on a demo venue at 1366 and
  1024: the sign-in keypad on the kit's NumPad, keys 54px at 768 tall with
  32px digits, a tap and a typed key counting two digits not three, delete,
  six digits and Login signing in; the till's areas at 48px, the chosen one
  terracotta; a role's 22 permission switches as rows. **Not seen:** the
  kitchen board's switches; the till's fields (a line's note, the day's
  close, closing stock, a payment's reference, the float count, a waiter's
  collect) in their kit look; the action pills now Buttons (the kitchen's
  Back and Lock, Retry now, Pick a table, the waiter's Back and Active
  Orders, More). **Left as is:** a role's name field shows the name as
  stored, "cashier", while everywhere else reads "Cashier".

- **No class that renders nothing, and the kit's strokes** (ADR-405, Phase 17
  Task 31b; 2026-11-01). Seen in a scripted Chromium on a demo venue: the
  lock screen's tagline at 11px, 700, `.16em`; the till drawing only 1.75
  and 2.25 strokes. The class check passes on every build with 1009 classes
  in the built CSS, and named the seven it was written for before they were
  mended. **Not seen:** the setup wizard's tagline, the pay screen's, the
  kitchen board's icons and the QR codes without `bg-white` (the SVG paints
  its own white). **Left as is:** a class built from a template
  (`text-${tone}`) is not judged by the check.

- **The back office's rows, heads and drawer** (ADR-404, Phase 17 Task 31a;
  2026-11-01). Seen in a scripted Chromium on a demo venue at 1366 and 1024:
  the staff rows named as the person and described by the hint, the shown
  one `aria-current`, a chevron on each; Add staff on "Choose a role",
  refused without one; the menu list's 18 rows with no tile and no Active;
  "Menu" and "Staff" as heads; Add a table terracotta; at 600 wide, the
  drawer's Close taking focus and Close and Escape giving it back to the
  menu button. **Not seen:** a menu where some items have photos (the room
  kept for the rest); the dashboard's head for a role that opens nothing;
  "That is not yours to open".

- **One focus rule, one field rule, arrows in every group** (ADR-403, Phase 17
  Task 30; 2026-11-01). Seen in a scripted Chromium at 1024x768 and
  1366x768 on a demo venue: a till button's focus, a 3px terracotta
  outline; a field's, the green halo; a picked table's green ring inside
  its edge beside a focused table's outline; the venue form's fields
  described by their hints, and a refused one by its error with
  `aria-invalid`; Right on the item window's Size, the pay screen's
  payment method and Left on Settings' tax profile, each moving the choice
  and the one tab stop with it; the floor editor's grid, one tab stop of 96,
  Right, Down and End moving between cells; "Online" alone the live region.
  **Not seen:** the pairing window's "By name" and "Also at" sentences
  (the walk's server listened on loopback, so it had no addresses to list;
  the change is their class, `text-sm`); the kitchen's On/Off label; focus
  at night (the token is mustard there); a read-only floor grid passing
  over its empty cells (tested by `gridStep`); the guest's choice groups
  with arrows (the same helper).

- **Error boundaries, told to the server's log** (ADR-402, Phase 17 Task 29;
  2026-10-31). Seen in a scripted Chromium at 1024x768 and 1366x768 on the
  composition venue of Task 28, each part made to throw by bending the
  server's answers in flight (a list sent as a number): the till's rail in
  its own column with the menu still ringing up, and mended by Try again
  once the answers were right; the pay screen's fallback under the till's
  header; a back office route inside the shell, mended by Try again; the
  import window's fallback, closed by its Close. Each reached `logs/mez.log`
  once, as "a screen failed to draw". **Not seen:** the page-size fallback
  behind the gate (lock, pair, the kitchen's pass) and at the root; a Sheet's
  fallback (the same component as Modal's); the kitchen display reporting as
  `kitchen`. **Left out:** the guest's app does not report (no device token;
  ADR-402); a stack is the production bundle's minified names.

- **The GST composition scheme** (ADR-401, Phase 17 Task 28; 2026-10-31).
  Seen in a scripted Chromium at 1024x768 and 1366x768 on a venue set up by
  the wizard under it: the wizard's five tax cards, no "Menu prices"; the
  tax card's line and GSTIN; its "No tax" class; a file with Tax class,
  Station and Course blank imported whole; a cash sale whose slip reads
  GSTIN, BILL OF SUPPLY, the rule's line, no tax lines. **Not seen:** the
  printed bill, its duplicate and the bill before payment (tested as laid
  out, at 80mm and 58mm); a credit note (tested); the guest's receipt
  (tested by its words). **Left as is:** the till's own order panel shows
  "Tax ₹0.00" under the composition scheme, as under No tax; moving from
  the composition scheme to GST leaves items on "No tax", exempt, so they
  sell without GST until moved, and Settings lists items without an HSN
  then but not items on an exempt class.

- **A new venue's station and course, and the import's word for their lack**
  (ADR-400; 2026-10-31, out of phase). Tested in Go: first run, the wizard's
  POST /api/v1/setup and `mez setup` each make a Kitchen and course 1, Main;
  the demo does not; the import says a missing tax class, station or course
  once, as "The file". Seen in a scripted Chromium at 1366x768 on a venue
  from `mez setup`: Stations and courses lists Kitchen and Main, and a file
  with Station and Course blank is told only "The file | Tax class | This
  venue has no tax class yet..." beside its one row's own price. **Not
  seen:** the wizard's venue by eye (tested over HTTP). Venues set up before
  it are not given one: their owner adds them under Menu, Stations and
  courses.

- **The back office for a venue with nothing in it yet** (ADR-399, Phase 17
  Task 27; 2026-10-31). Seen in a scripted Chromium at 1366x768 on the
  empty venue from Task 26's walk, paired as a till: the dashboard's backup
  band ("Nothing is being backed up..." and Set up the backup) and "No
  ingredients tracked. Add what you keep, in Inventory."; Inventory's empty
  card with Add ingredient; Z-reports' empty card with Open the till; Add a
  printer's "What it prints" on Receipts, and the new printer printing
  them; Devices' and the floor's mez Cloud lines; Settings' Off chip in the
  warning tone. On a demo venue (`mez setup -demo-menu`): 36 recipes, each
  "No ingredients tracked". **Not seen:** Printers' empty card and its
  button (the walk's venue had a retired printer, so its list was not
  empty); the danger band for a failing backup (tested); a printer set to a
  station's tickets, and a failure to set one (tested by their logic only);
  the reworded Devices and Z-reports lines after their last change (tested
  as words).

- **Setup says what happened, offers the name, ends on the menu** (ADR-398,
  Phase 17 Task 26; 2026-10-31). Seen in a scripted Chromium on two fresh
  venues at 1366x768, the device step at 1024 too: the time zone chosen as
  "Kolkata · India Standard Time (GMT+5:30)" of 419; the tax cards with
  their dots; "Searched this network and this computer's own printers."; a
  test page sent to a stand-in printer (a socket on port 9100), then
  "Nothing came out, skip" to the Cloud step with Printer marked Skipped;
  Cloud skipped and marked so; `mez.local:7792` first, "Or by number" and
  192.168.29.211 under it; "Add your menu" landing on the back office's
  Menu, signed in as the owner. **Not seen:** a tablet opening
  `mez.local` (ADR-379's own walk); the name with no numbered address; the
  zone list in a browser that cannot list zones (the typed field, tested
  by type only); mez-cloud's PaymentLink with the dot (its build reads
  `fields.tsx`; not built here).

- **The venue's menu, light on 3G** (ADR-397, Phase 17 Task 25;
  2026-10-31). Seen in a scripted Chromium: at 360 and 390, the landing's
  "Welcome to Corner Cafe", the bar's "Corner Cafe" and a made-up long
  name cut short beside the table's pill, "Ordering by mez" under the
  button and the dishes, the tab titled "Corner Cafe"; the phone fetching
  only `fraunces-600` and `dm-sans-14` for the landing and the menu, and
  Archivo once the sheet opened; the landing at 1366; the till at 1366
  still fetching the variable Fraunces and DM Sans. **Not seen:** the
  landing to collect (the same code, its own words); the preload's gain on
  a throttled 3G link (not measured); a venue with no name (setup requires
  one). Left: a venue cannot yet put its own line on the landing.

- **The guest's phone: edges, thumb, and words** (ADR-396, Phase 17 Task
  24; 2026-10-31). Seen in a scripted Chromium: at 360 and 390, "2 in cart"
  beside a card's price, + Add in the tint, "Gluten free" muted; the sheet's
  "Add · ₹2,280.00" beside the count at twelve, and a made-up long "Choose
  ..." ending in an ellipsis with the count in place; the cart's 48px Clear
  and "Tap again to clear" with the count stepped aside; "Place order",
  "Start ordering". With a 34px home bar faked through the DevTools
  protocol, View cart 34px clear and the sheet's Add 51px. The landing at
  768, 1366 and 1920 with its button 32px under the card. **Not seen:** a
  real iPhone (the insets were Chromium's override); a phone on its side,
  where the sheet pads by the notch's inset; the terminal's and back
  office's sheets since (no insets there, so no change by construction).

- **The guest's order tells the truth** (ADR-395, Phase 17 Task 23;
  2026-10-31). Seen in a scripted Chromium at 390x844: "Placed at 6:24 PM ·
  just now"; the strip at the top of the menu after scrolling; a request
  declined at the till read "This order was not accepted" on the phone, and
  Put these back in my cart brought the Upma back; "We can't reach the venue
  right now..." after thirty seconds of checks refused, and "The venue no
  longer has this order..." on a made-up not found. **Not seen:** the
  five-minute nudge (tested); a cancelled order's way back (the same
  button). Left: the nudge's five minutes is fixed, not the venue's.

- **One set of table words; follow-ups that say what happened** (ADR-394,
  Phase 17 Task 22b; 2026-10-31). Seen in a scripted Chromium at 1024x768:
  the Tables screen's line and bars agreeing (8 tables, 5 free, 1 ordered,
  1 paid, 1 to clear in Inside); Seat and order the free table's primary;
  a guest's request accepted, the sheet closed and "Table T8: sent to the
  kitchen."; the paid step's "No receipt printed: this venue has no
  receipt printer..." on the demo venue; the waiter's home in the till's
  words, T5 Paid, T8 "No guest count". **Not seen:** a receipt that printed
  (the demo venue has no receipt printer; tested); a request declined in the
  browser (tested by its words).

- **The till's words** (ADR-393, Phase 17 Task 22a; 2026-10-31). Seen in a
  scripted Chromium at 1366x768: "Tap your name first." from a key before a
  name; the pill **Till**; a dish card's name read first; "More actions, 12
  open orders" and the menu's **Open orders**, **This order's history**,
  **Today's orders**; the open orders sheet's "Opened 8:48 AM"; the tender's
  head "5:48 PM · Masala dosa"; "Guest pays by UPI"; "Paid for
  INV/26-27/000005"; Today's orders' title; **Close the day** in the profile
  menu. **Not seen:** "Send to Kitchen" on an empty table order (tested); the
  waiter's tablet, which keeps its own **Orders** tile (its place is its own
  orders). Left: a short daily order number, which the server does not have.

- **The kitchen display says what changed** (ADR-392, Phase 17 Task 21;
  2026-10-31). Seen in a scripted Chromium at 1366x768: a second send of a
  counter order marked Add-on; a line voided on the till (reason, Arjun's
  PIN) on the kitchen's board 248 ms later, struck with a Void chip; the
  foot's Sound and Skip preparing switches, and Skip preparing's one tap to
  the pass; the station picker on /pass staying on ink; timers ticking.
  **Not seen:** a later course's chip (the demo menu sends no course 2;
  tested in vitest); the void chime, heard. Left: a void of a line never sent,
  and a note on a sent line, name no station and reach the board at its
  20-second read; a ticket whose only line is voided stays in New Orders with
  Start preparing.

- **The kitchen display, heard and trusted** (ADR-391, Phase 17 Task 20;
  2026-10-31). Seen in a scripted Chromium at 1366x768: a ticket sent from
  the till arriving as new, with "1 new below" in a full New Orders; Recall
  and Change station asking for a second tap, and Recall's resetting after
  4 s; the brick band 13 s after the demo server was stopped; the 2px edges
  on night. The sound band seen only with Chrome's rule emulated in the page
  (headless Chromium plays sound without a tap), and gone at a tap on the
  board. **Not heard:** the chime itself, louder and twice, on a real
  kitchen screen; **not seen:** a double tap held by the 700 ms rule (tested
  in vitest), the failure words on screen (tested). Left: "1 new below"
  covers the foot of the column's last card while it shows.

- **Sold out without a fade; the warning toast as a chip** (ADR-390, Phase
  17 Task 19; 2026-10-31). Seen in a scripted Chromium with two demo dishes
  sold out: the till's cards at 1024 (opacity 1, the well's fill, the chip,
  no "+"), the guest's cards at 390, and a warning toast at 1366 in the
  chip's fill with a 1px border (the menu's refusal made up by the walk).
  **Not seen:** the item modal and search results on the till with a
  sold-out dish (the same card); the guest's item sheet for one.

- **Diet marks by shape** (ADR-389, Phase 17 Task 18; 2026-10-31). Seen in a
  scripted Chromium with two demo dishes marked non-veg: the till's cards at
  1024 (triangles and dots, "GF" beside the wheat) and the guest's menu at
  390. **Not seen:** the till's line rail and item modal, the guest's cart
  and the back office's item sales, which draw the same component.

- **The quiet inks and terracotta text** (ADR-388, Phase 17 Task 17;
  2026-10-31). Seen in a scripted Chromium: muted and faint rendering as
  `#6E6359` on the dashboard at 1920x1080 and the till's pay screen at
  1024x768; the key line at the sidebar's foot at 1920x1080 and not at
  1366x768, where each letter's hover says it; the deeper terracotta on the
  dashboard's links. **Not seen:** mez-cloud's account area with the new
  inks (it is built from this checkout's tokens.css; build it there); the
  night surface, which is unchanged and unchecked against AA. Left: muted
  and faint are one colour on paper, so the small-caps metadata is no
  lighter than a note.

- **No silent loss of work** (ADR-387, Phase 17 Task 16; 2026-10-31). Seen
  in a scripted Chromium at 1366: an item's unsaved name asked about on
  another item (Keep editing kept it), on the sidebar (Escape stayed) and on
  `g d` (Leave went to the dashboard); a group's Retire asked "3 items stop
  offering it"; an item retired from its editor and brought back by the
  toast's Undo; Settings' "Save venue details" and "Save phone payments",
  and the sidebar asking on a venue draft; an area with tables in service
  told so, with Close; a table's Retire asked with its QR code. At 1024 on
  the till, a typed order note kept after a tap on the scrim, and closed by
  Escape. **Not seen:** Sign out asking; the approval and line sheets (the
  same prop); the drawer under 768px; the browser's prompt on closing the
  tab, now the shell's. Left: forms outside Settings and the item editor (a
  stock count, a recipe, a role's permissions) do not hold drafts with the
  guard yet; the Undo toast covers the foot of the screen for 8 seconds.

- **Tax words that match the venue** (ADR-386, Phase 17 Task 15;
  2026-10-31). Seen in a scripted Chromium on the demo venue (GST): with
  prices that hold the tax, the cart and the order placed at 390 say
  "Prices include any taxes." and the till's requests at 1024 and 1366 "1
  dish"; with `tax.pricing` exclusive, "Before tax, which is added on the
  bill." and "1 dish, before tax"; a paid table's receipt at 390 says
  "GSTIN 29ABCDE1234F1Z5". **Not seen:** the words to collect ("added
  before you pay", "which was in what you paid", "To pay, with tax"),
  which need a payment gateway (tested in vitest and the API's tests); a
  VAT venue's "VAT number" (tested); the landing where the venue's taxes
  cannot be read (tested in Go: it opens and says "before tax"). Left: the
  menu's prices themselves say nothing of tax.

- **One venue clock** (ADR-385, Phase 17 Task 14; 2026-10-31). Seen in a
  scripted Chromium: the back office's bar at 1366 ("Sat, 31 Oct 2026 ·
  12:24 PM"), at a faked 1:05 AM ("Trading day 31 Oct · 1:05 AM") and on a
  390 phone ("31 Oct"); Sales, Audit, Staff, Devices, Printers, Z-reports
  and a report with no ISO date left on screen but the exports' file
  names; close of day's rail at 1024 and 1366 with the trading day and
  "Opened 12:25 PM" and no shift id. **Not seen:** a venue with a day
  start other than 04:00 (tested: the ranges, the heatmap's order and the
  notes follow it); a laptop set to another zone (tested in the clock's
  own tests). Left: the mirror note, the kitchen board's clock and the
  sales heatmap's hour labels keep 24-hour times; a back office opened
  straight onto Sales before the venue is read takes its first range on
  the device's clock.

- **Close of day says what is open; the Z-report what it carried**
  (ADR-384, Phase 17 Task 13; 2026-10-31). Seen in a scripted Chromium at
  1024x768 and 1366x768: the note over the counter with a saved counter
  order and an unpaid T3 open (a paid T5 rightly left out), See open orders
  landing on Orders' Open tab with both, the confirm's line, and Z-report 1
  saying "Open orders carried 2 · ₹250.00 to pay" in the report and its
  slip. **Not seen:** the printed slip on paper (tested in Go: "Open orders
  carried (2)" at the foot of SALES on 80 and 58 mm); the back office's
  Z-report, which is the same view; the line when the day's list cannot be
  read. Left out: the owner's WhatsApp close-of-day message, the CSV
  exports and the X-report do not carry the figure.

- **A counter sale sent and closed by its settling payment; a paid table
  reads Paid** (ADR-383, Phase 17 Tasks 2 and 3; 2026-10-31). Seen in a
  scripted Chromium: a takeaway paid in cash without Send reached the
  kitchen board, the paid panel stayed until Next order, Order History
  said Paid, a table paid at the till read Paid on the floor and on the
  waiter's home, and a table's unsent lines showed the warning and Send
  now. **Not seen:** a counter sale split in shares (the last share is the
  one that sends and closes); card and UPI waits ending in Mark as paid;
  a till still on the old bundle pressing New Order after the payment
  (tested in Go: the close is answered as done); the paid panel left by
  Back to Order or a nav pill (the order is let go, and the register
  starts afresh). Left over: the waiter's bill (T05) does not warn of
  unsent lines yet; a table's ticket paid in full still waits for Next
  order or Close order to become To clear, by design (ADR-372).

- **Phase 17's device-fit pass** (Tasks 4 to 12, ADR-381, ADR-382;
  2026-10-31). Seen in a scripted Chromium at 1024x768, 1366x768 and
  1920x1080: the lock, open shift, register and its rail, the item modal,
  tender (cash and change in the foot), split by item, the paid step,
  Orders and the order panel, refund, Tables, the waiter's home and order,
  close of day, the kitchen board, the back office's menu, item editor
  (sticky save), floor (chosen table beside the grid) and the setup
  wizard. **Not seen:** the kitchen board with many tickets, a late one,
  notes (the new "Note:" box), courses, a void and the "+N more" All day
  strip; the waiter's Confirm and send through to the bill; a guest's
  table request with the new top toasts; the lock's session-ended notice
  at 1024 (it adds about 50px and pushes the footer below the fold, the pad
  stays); paging of profiles (needs 6 staff at 1366, 9 at 1024); touch on
  a real tablet; a real kitchen screen at two metres. Left over:
  - The waiter's order bar at 1024 can overflow by about 70px when a
    table-requests pill also shows in the header; the guests pill could
    become an icon below xl.
  - `sales/SalesScreen.tsx` keeps a `minmax(0,29rem)` column: no width
    token holds the category ring's legend at 1366. Needs a token or a
    decision.
  - The order panel's title breaks an invoice number across two lines at
    1024 ("INV/26-" / "27/000001"); Orders' search placeholder is cut.
  - The register's sidebar still truncates "Tea and co..." at 1024.
  - The kitchen board's ink is now the default; `index.html` still says
    `data-surface="paper"`, so a parchment frame can show for an instant
    before the script runs.

- **The 2026-10-24 stability audit and the venue PC's three complaints**
  (ADR-366 to ADR-370). Built, tested in Go and vitest, **not yet seen on a
  venue PC or in a browser**: the human walks, on the installed PC, a
  restart and the shortcut (the till opens by itself within a minute;
  `mez service status` says running), the shortcut with the service
  stopped (`mez service stop` as administrator, then the shortcut, with no
  prompt), `mez.exe` double-clicked (the till, not a console), Pair a
  device (the address and QR code beside the code; a phone scanning it
  lands paired), and a tablet on a wifi Windows calls Public reaching the
  till. **Done on 2026-10-25 at the human's word** (ADR-372 to ADR-375,
  none seen in a browser or on a venue PC): a paid, invoiced ticket rolls
  when the party orders again; `discount.removed`, with a cross on the
  till's order discounts; an order carries the taxes it opened under; the
  shift close projects the day once; the Windows service reports
  checkpoints while it stops and has a minute at shutdown; idempotency keys
  pruned daily, the rate limiter capped, the event log quieter. To walk: a
  prepaid table ordering twice (the second round's ticket is new, the
  first closed); a 200 discount on a 440 bill, void one line (refused),
  remove the discount with a PIN, void it (allowed); flip prices to
  inclusive with a ticket open and see its total unchanged while a new
  ticket's differs; `shutdown /s` on the venue PC with the event log's
  "mez stopped." after it. **Done on 2026-10-26 at the human's word**
  (ADR-376 to ADR-379, none seen in a browser or on a venue PC): a stuck
  print job requeues itself; a waiter's new ticket at a paid table rolls
  it; the waiter's bill keeps a lost payment across a reload; retries
  replay under the same key on the till and the kitchen; the orders sheet
  says when its read failed; cash rounds once, at the end, to the venue's
  step; a shift's cash sales are kept as they happen; the server answers
  for `mez.local`; a body is read under a deadline; an open's replay is
  answered before the shift is asked; the sign-in screen has Back; Tips'
  Back stays in the app; the station picker leads back to its board. To
  walk: set cash rounding to 50 under Settings, ring up 101.25 and see the
  cash tile ask 101.00 or 101.50 and the invoice say the rounding; open
  `http://mez.local:7777` on a phone on the venue wifi; let a kitchen
  printer go off, print, and see the job come back after two minutes;
  ADR-371's eleven fixes as well (a count of the drawer with a cashier on
  another till, Enter after a tile, the Till pill). Still owed:
  - The printed receipt's layout does not show the cash rounding row
    (ADR-377); its figures add up through the tender line.
  - `pair` sends no idempotency key, so a code consumed with its answer
    lost needs a new code (one `ErrorBoundary` at the root no longer takes
    the whole till down: ADR-402); the lock screen's "Add staff in the back
    office" has no link, since nobody is signed in to carry one; the
    open-shift sold-out card and a few screens still not seen by eye.
  - Android before 12 does not resolve `mez.local`; those tablets use the
    address and the QR code (ADR-379).

- **Nothing finds a server for a till** (ADR-338, found walking "The PC
  died"; 2026-10-13). Phase 12's discovery is of printers and of this
  server's own addresses for the pairing card
  (`internal/device/discover`). A pairing is kept by the browser for one
  address (`localStorage`), so a new computer at another address means
  every tablet is paired again. Open question 14 in `STATE.md`. What the
  same walk found of a till left on, which forgot its pairing at a
  computer with no venue, is cured (ADR-339): **not seen** there, the
  waiting screen on the back office and on a kitchen display, and at
  400px; and its pill says "Server unreachable" for the seconds between
  the venue coming back and the stream's next try.

- **Two mirrors of one venue share a state file loosely** (ADR-327, found
  in ADR-331's review; 2026-10-12). `internal/mirror/keeper.go`: `refresh`
  passes over a file it cannot read just then, and `save` marks the file
  as seen after renaming it, so in the same few milliseconds one mirror
  can write over a browser the other has just paired or a lock it has
  just set, and may then not look again until the file next changes. A
  lock lost this way is one more guess at a PIN, of three a day. The
  cloud's spent assertions are not kept in that file and are not affected.
  Not fixed: nobody runs two mirrors of one venue yet (`mez-cloud`'s 8e
  will), and the cure is a lock on the file or a file to each thing.

- **The audit between phases** (ADR-287; 2026-10-07). Four reviews and
  one walk against the demo venue; what they changed is in the ADR.
  **Found and left**, for the owner: any paired terminal or back office
  device may read the venue's whole event stream over the WebSocket with
  no staff session (`internal/api/stream.go`), so a stolen till's device
  token reads every payment until it is revoked under Devices; a leaked
  relay key (`mzr_`) is believed until the relay restarts with it gone;
  a person with `staff.manage` may reset their own PIN without the old
  one; `-relay-key`, `-gateway-key-secret` and `-gateway-webhook-secret`
  may still be given as flags, which a process list shows; the Windows
  spooler send ignores its deadline, so a hung USB port holds that
  printer's queue until the spooler answers; a guest's checkout started
  twice in the same instant leaves one at the gateway to lapse unpaid,
  logged; the venue's rate limits key on the client address, so behind a
  reverse proxy every client is one address. **Walked and seen**: all
  eleven back office screens open without a console error; the till's
  sale, kitchen, guest, UPI and close flows above. **Not walked this
  time**: card, split, void, refund, credit note, reprint, the Menu
  editor's edits, Inventory, Printers with a device, the setup wizard
  (`/setup`), the relay. The demo server logged four ERRORs in the
  walk, all a cancelled read, which `internalError` now keeps out.

- **The README** (ADR-286; phase 12 Task 7f). The screenshot
  (`docs/img/till.png`) is the register at 1366 by 900 with the demo
  menu, taken from `bin/mez` on 2026-10-07: it will go stale as the
  screen changes, and nothing retakes it. The demo menu has no photos, so
  it shows placeholders where a venue's would be. The guest app has no
  licence file of its own and is under the AGPL; whether it becomes MIT
  is the owner's to decide. **Phase 12's "done when" has not been
  tried**: a clean Windows machine, from the downloaded installer to a
  first printed invoice in under ten minutes. It needs a release, a
  spare machine and a printer.

- **The relay's guides** (ADR-284; phase 12 Task 7d; `deploy.md`,
  "Guests ordering from their phones"; `build/relay/README.md`). Checked
  against the code, the relay's binary and two answers from
  `relay.jskin.in` (retired since, ADR-411). **Not done for it**: a venue set up by following the
  new section from nothing; the three settings through the compose file
  or `/etc/mez/mez.env`. On an installed Windows venue the key sits in
  `C:\Program Files\mez\config.toml`, which every account on the PC can
  read. The relay's `/v/<venue>/` addresses (no venue domain) are left
  undescribed: not run with the guest app as it is.

- **Troubleshooting** (ADR-283; phase 12 Task 7c; `troubleshooting.md`).
  The screens' words are quoted from the code. **None of the five has
  been followed at a venue**: no printer, no drawer, no second device
  held back by a real firewall, no relay taken down on purpose. What it
  says of Smart App Control, client isolation and a drawer's lock is how
  those things generally behave, not something seen here. The back
  office cannot search for a printer; only the setup wizard can.

- **Hardware** (ADR-282; phase 12 Task 7b; `hardware.md`). Written from
  the print code. **No printer and no drawer has been tried**, so no
  model is named. Left as they are and now written down: nothing opens
  the drawer without a cash sale or a cash refund (no button, and not
  cash in, cash out or the count); the pulse is `DLE DC4`, pin 2, 200 ms,
  with no setting, so a printer that acts only on `ESC p` will not open
  it; paper is 48 or 32 columns, with no setting for a 42-column printer;
  text on paper is ASCII, and any other script prints as `?`.

- **Releases** (ADR-280; phase 12 Task 6; `releasing.md`). Every
  piece run by hand on 2026-10-07: the four binaries, the installer, the
  checksums, the notes, the image for both architectures. **Not run: the
  workflow.** No tag has been pushed, so Inno Setup on the Windows
  runner, the push to `ghcr.io` and the draft release are as written.
  Nothing is signed. The image's package will start private. The notes
  are commit subjects and want editing in the draft. The
  `docker-compose.yml` a release carries builds from source: with no
  clone beside it, its `build:` block has to go and its image be named
  by hand (`deploy.md`, Docker; ADR-281). The `docker run` lines for the
  release's image have not been run against `ghcr.io`.

- **Linux and Docker** (ADR-279; phase 12 Task 5; `deploy.md`). Run on
  2026-10-07 in containers on Docker Desktop: `build/mez.service` under
  systemd 252 on Debian 12, and the image through setup, a backup to a
  mounted folder, stop, restore and start. **Not run**: arm64 (it
  compiles); a real Linux machine; the compose file's own `up`; a host
  folder with the wrong owner; a relay through compose. The setup wizard
  does not work in a container (every browser is elsewhere to it): use
  `setup`. No healthcheck in the image. A backup to a folder under
  systemd needs `ReadWritePaths`. If a destination answers and is gone a
  moment later, or the database is held for over ten seconds just then,
  Litestream can still close its handle on the database as it starts,
  which on Linux drops the server's locks: once for each look at the
  destination, where it was once a second. Narrow now, not shut.

- **Restoring a venue** (ADR-278; phase 12 Task 4f; `deploy.md`,
  "Getting a venue back"). Done on 2026-10-07 on Windows with the built
  binary: three sales, the server killed, `mez restore` from the folder
  into an empty directory, mez started on it, the paired till signed in,
  the next sale invoice four. **Not done**: a restore from a bucket by
  the command; on Linux by hand (its tests ran in a container); on
  another computer; of a venue with months of
  sales; of a photo a menu item points at (a file was put in the folder
  by hand). `backup.json` and `config.toml` are not in the backup. A
  generation whose data directory was lost is never tidied away. The
  tidying counts a week by this machine's clock: one set far ahead on
  the day it looks removes last week's generation early. The
  restored copy's printers are whatever the old one had, and jobs not
  printed when it died are still waiting.

- **The Backups section** (ADR-277; phase 12 Task 4e;
  `backoffice-settings.md`). Seen on 2026-10-07 in a browser against the
  built binary, at 1366 and 1024: every state of a folder backup, the
  refusals under their fields and at the foot, the bucket's form and a
  bucket that does not answer, a drive replaced by an empty one refilled
  by itself, off. **Not seen**: a real bucket saved from the screen;
  `restart_needed` (never shown on Windows); a screen reader; night. The
  status is up to 15 seconds behind. A reason is shown as the system
  gave it, which on Windows is English and a long path. There is no
  restore on the screen, by design.

- **Setting the backup from the back office** (ADR-276; phase 12 Task
  4d). The API; the screen is the entry above. Seen on 2026-10-07 in tests on
  Windows: a destination saved, tried and backing up at once; swapped
  while selling; refused when it cannot be written to; over HTTP as an
  owner and a cashier. Seen on Linux in a container later that day:
  the backup's, database's, server's, settings' and API's tests, a real
  MinIO, and the race detector for the first time (clean in mez; one
  race inside Litestream as it closes, which costs a whole copy in
  flight at shutdown and nothing else). **Not seen**: `mez pair` beside
  a Linux server whose backup was just changed; a bucket saved through
  the API. A
  save can take twenty seconds. The destination left behind is not
  cleaned up. Old generations under `db/` are removed a week after they
  were replaced, by the data directory that began them (ADR-278), and
  `mez restore -list` shows them. Generations sort by
  this machine's clock. A drive swapped for an empty one is refilled by
  itself on Windows, after the next look at it (15 seconds for a folder,
  a minute for a bucket, and reading ok until then), and at the next
  restart elsewhere. Until this task a stop could leave the last sales
  out of the backup (about one stop in ten on a busy machine) and a
  reused destination could restore short; both were in 4a to 4c as
  committed. The Linux file-lock hole this recorded is ADR-279's,
  and mostly closed there.

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
  the data directory. The screen is ADR-277's and `mez restore` ADR-278's. Litestream
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
  service; signing (no certificate). Built since by ADR-366 and owed a
  walk: the firewall rule for every profile, the shortcut that starts the
  service and follows `listen_addr` from `config.toml`. **Seen by the human
  on 2026-10-24 on a venue PC**: after a restart the shortcut opened a
  browser on an error page, and `mez.exe` double-clicked ran in a console
  that stopped mez when closed, which ADR-366 answers.

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
- **What the walk found and left, for the owner to decide** (ADR-199).
  **All five were decided on 2026-10-13 (ADR-342) and built (ADR-343),
  unseen in a browser.** As they were found:
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

- The back office from away, and the relay's limit for one address (seen
  2026-10-08): thirteen page loads in half a minute passed 120 requests a
  minute and the relay answered 429 with the guests' sentence, "A lot of
  people are ordering at once". A person is slower than that, but coming
  back to a tab reads every query again (ADR-309), and a venue's owner and
  its guests on one office Wi-Fi share an address. Watch for it on the
  real-phone walk; the relay could count a paired back office apart.
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
- Shift, tips and receipt screen, **Open on** (ADR-360, ADR-361; phase 15
  Task 4b): built 2026-10-22 and **not seen in a browser**: the seven rows,
  a tick starting a day at 09:00 to 23:00, "next morning" on 17:00 to
  02:00, the refusal naming the day, a save and a reload. The rows wrap at
  a narrow width and were not checked at 1024.
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
