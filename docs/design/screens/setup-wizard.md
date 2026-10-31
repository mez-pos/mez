# Setup wizard

The first thing a new venue sees: the questions that make the venue, asked
once, on the computer mez runs on. Built from D24 in phase 12, Task 3b
(ADR-267), its printer step in Task 3c (ADR-268) and its last in 3d and
3f (ADR-269, ADR-271).
`web/shared/screens/setup/`: `SetupWizard.tsx`, `steps.tsx`,
`PrinterStep.tsx`, `ReadyStep.tsx`, `SetupFrame.tsx`, `fields.tsx`,
`setup-logic.ts`, `printer-logic.ts`, `ready-logic.ts` and
`use-setup-status.ts`; and, for a venue brought back (phase 14, Task 7c,
ADR-336), `RecoverFlow.tsx` and `recover-logic.ts`.
It is the terminal's, at `/setup`, outside the guard.

## Purpose

Take an owner who has just run the installer from an empty machine to a
till they are signed in on, with no terminal window and nothing to read
but the screen.

## Layout

One column, 58rem at most, centred on the sunken ground with D24's pale
round behind the head. It scrolls as a page; the one thing held still is
the card's row of buttons, below.

At 768px high (`short:`, 820px or less) the head gives way, never its text
size: the small-caps line goes, the logo steps down from 48px to 36px, and
the lead shows only on the first step of a run (step 1, the printer step
after Create, and the number of I had mez before). The card's last row, its
buttons, stays at the foot of the window while the card runs past it, on
the card's own ground and over its padding, and comes to rest at the card's
foot as before; the page keeps room above it, so a field that takes the
focus is never under it. So Continue, Skip for now and Add your menu are in
view on every step at 1366x768 and 1024x768 (SET-4).

- **Head:** the logo with "Good food brings people together" in small caps;
  "Welcome. Let's set up your venue." in the serif; one line saying how many
  steps there are and that everything can be changed later in Settings.
- **The steps:** a numbered round for each, joined by a rule, with its name
  under it. Done is the green fill and a tick, and can be gone back to; the
  one in hand is green with its number and its name in green; the rest are
  sunken. A step passed over (the printer or mez Cloud, skipped) is a dashed
  round with a dash and "Skipped" under its name, never a tick (SET-2).
- **The card:** the step's title in the serif, a line under it, its fields,
  and along the foot "Step 2 of 6", **Back** and the green **Continue**
  with its arrow (**Create the venue** on the last of the questions).
- **Foot:** a script line, and where the wizard is being served from.

The steps:

1. **Your venue.** Venue name; name on invoices (optional, "if it differs");
   currency; time zone, chosen from a list in words ("Kolkata · India
   Standard Time (GMT+5:30)"), listed by city, this computer's own chosen
   already, and typed only where the browser cannot list the zones (SET-8);
   what this till is called.
2. **Taxes and invoices.** The tax profile, as five cards (No tax, GST
   India, GST composition India, VAT UK, Sales tax US), each with a
   radio's dot, filled on the one chosen (SET-9); the composition card says
   "No GST is added. Bills show your GSTIN and say you are on the scheme."
   (SET-7, ADR-401) and asks the GSTIN, not how prices are quoted; the
   registration number, where the profile
   has one; prices include tax or tax is added, where the profile lets that
   be chosen; the month the financial year starts; the invoice prefix.
3. **You.** Your name; a PIN of 6 to 8 digits; the PIN again. What the PIN
   is for, and that more people are added later in the back office.
4. **Printer**, after Create, when the venue exists and the owner is signed
   in. **Printers found**, with Search again: this computer's own printers
   first (by Windows' name), then each address on the network that answers
   on the printer port, each a row to pick, marked "already added" where
   the venue has it; under them always **Enter its address**, which opens
   an address field. What was searched is said under the list, in words
   ("Searched this network and this computer's own printers"), never a
   network's number (SET-8). Once one is picked: **Paper**, 80mm or 58mm.
   Then what became of the test page. The foot has **Skip for now** and the
   green **Print a test page**; after a page is sent, **Nothing came out,
   skip**, **Print again** and the green **It printed** (SET-5).
5. **mez Cloud** (phase 14, Task 4c2, ADR-322; `CloudStep.tsx`, with its
   words in `web/shared/cloud-pair.ts`). "Connect to mez Cloud": a line
   saying what a code from a mez Cloud account does, that the till works
   the same without it, and that it can be done any time later in the back
   office under Devices. One field, **Pairing code**, in the numeral face
   and capitals, half the card wide, with "From your mez Cloud account,
   under this venue. It works once, for an hour." under it. The foot has
   **Skip for now** and the green **Connect**. A code that could not be
   one is refused under the field before anything is sent, and what the
   server refuses is said there in its words. While it works the buttons
   are held, the green one reads **Connecting**, and the hint says it can
   take up to twenty seconds. Then the card is **Connected to mez Cloud**:
   whose account, a line for the backup, "Restart mez to finish" if part
   of it waits for a start, the address the back office opens at from
   away, in the numeral face, and the green **Continue**.
6. **More devices.** "Add another device", as three numbered things to do
   on it. **1. What is it?** Another till, Kitchen display or Back office,
   as cards. **2. On that device, scan this or open the address**: first, where the
   server answers for its name (ADR-379), a sunken band holding a QR code
   of `mez.local:7777` with that app's path, 132px on white, and beside it
   the address in the numeral face (the display size from 1280px wide, the
   total size under), with "By name, on an iPhone, iPad, Mac, Windows PC or
   Android 12 and later. The name stays the same when this computer's
   address changes." (SET-1). Then "Or by number, on an older Android
   tablet", and a band like it for each address. A computer on
   more than one real network has a band for each, each headed "On
   Wi-Fi", "On Ethernet", and a line saying to use the one the other
   device is on: none is put first as the likely one. Virtual adapters'
   addresses are named under the bands, small. Then what to check if it
   does not open.
   **3. Enter this code there**: the terracotta **Show a pairing code**,
   then the code at the same size on the same band, what it is for and
   until when. The foot says "Next, your menu: the till sells what is on
   it.", and has **Another code** once one is shown, **Go to the till**,
   and the green **Add your menu** (SET-6).

### I had mez before

The wizard's other way through, for an owner whose venue was on another
computer and paired to mez Cloud. It is entered from step 1: a sunken
band above "Venue name" with one sentence and the ghost **I had mez
before**. It then has four steps of its own in the same frame, and the
six above are not shown. **Set up a new venue instead** is on each form
and goes back to step 1.

1. **Your number.** "Bring your venue back", what comes back and that the
   account's phone is needed. One field, **Phone number of your mez Cloud
   account**, in the numeral face, beginning "+91 ". The green **Send the
   code**.
2. **The code.** "Enter the code": that a code is on its way to that
   number if it has an account. One field, **Code from your phone**, six
   digits in the numeral face. **Change the number**, **Send another
   code**, and the green **Continue**.
3. **Your venue.** The owner's name and what bringing a venue back does.
   The account's venues, each a card with its name and "Last backed up 12
   Oct 2026, 21:32" in this computer's time; the one chosen has the green
   border and tint. A venue with no backup is listed, says so, and cannot
   be chosen. The only one that can be is chosen already. Under them
   **What this till is called**. The green **Bring it back**. An account
   with nothing to bring back says so, and has only the way back.
4. **Bring it back.** Three cards in turn, the first two with nothing to
   press. "Bringing Corner Cafe back": downloading and checking, a few
   minutes on a slow line, leave the page open. "Corner Cafe is here":
   mez is starting again, a few seconds. "Corner Cafe is back": what is
   on this computer now; a line for photos that did not come, if any; a
   line for its pairing to the account, which changes from being paired
   to paired, with the address the back office opens at in the numeral
   face, or to not paired and how to; "Sign in with your PIN, as you did
   before"; and the green **Sign in**.

## Data

- The setup token (ADR-270), from the address's fragment
  (`/setup#token=...`): taken out of the address at once, kept in
  `sessionStorage` under `mez.setup.token` so a reload still holds it,
  sent as `X-Setup-Token`, and forgotten when the venue exists.
- `GET /api/v1/setup`, key `["setup", <holds a token>]`: `{needed, here,
  unlocked}`. No device token.
- `POST /api/v1/setup` (`SetupRequest`): makes the venue. No device token.
- `POST /api/v1/devices/pair` with the code it answered, then
  `POST /api/v1/sessions` with the owner's id and the PIN just typed:
  through `AuthStore.pair` and `AuthStore.signIn`, so the device and the
  session are stored as any other.
- `GET` and `PUT /api/v1/settings/numbering`: only when the prefix asked
  for is not the one the venue starts with.
- The printer step, as the signed-in owner: `POST /api/v1/printers/discover`
  on arriving and on Search again; `GET` and `POST /api/v1/printers`,
  `PUT /api/v1/printers/receipt`, `POST /api/v1/printers/{id}/test`, and
  `GET /api/v1/printers` again every 600ms for up to 12 seconds to read
  the test page's job. `PUT /api/v1/printers/{id}`, `/restore` and
  `/retire` where a printer is reused or replaced.
- The mez Cloud step, as the signed-in owner: `POST /api/v1/cloud/pair`
  `{code}`, once, when Connect is pressed. Nothing is asked on arriving,
  and nothing at all if the step is skipped.
- The last step: `GET /api/v1/devices/addresses` on arriving:
  `{named, addresses}`. `named` is each kind's address by the server's
  name and its `svg`, empty where no name is announced; it comes even when
  the addresses could not be read. Each of `addresses` has its `adapter`,
  whether it is `virtual`, and for
  each kind of device the `url` it opens and that address as an `svg` QR
  code (none where the server listens on this computer alone); and
  `POST /api/v1/devices/codes` `{name, kind}` for each code.

- **I had mez before**, with the setup token: `POST
  /api/v1/setup/recover/start` `{phone}`, `/verify` `{attempt, code}`
  and `/restore` `{venue_id}`, the last allowed thirty-one minutes. Then,
  with the claim the restore answered in `X-Recover-Claim` and no token:
  `POST /api/v1/setup/recover/outcome` `{}` every second and a half
  until it answers, once more with `{device_name, device_kind}` for the
  code this browser pairs with (`AuthStore.pair`), and again every
  second and a half for up to forty-five seconds while `cloud` is
  `pairing`. The claim is held in the page's memory and nowhere else.

Nothing is subscribed to. Nothing typed is kept anywhere but the page: the
PIN is in memory until the wizard is left.

## Interactions

- **Continue** checks the step's own fields and says what is missing under
  each, and the first field refused takes the focus. It does not ask the
  server until the last step.
- **A step arriving** (Continue, Back, a done step's round, each step after
  Create, and each of I had mez before) brings the page to its top and puts
  the focus on the step's heading (SET-3). A step that focuses its own first
  field (Your venue, You, the phone number) keeps it there.
- **Changing the tax profile** sets the currency, the financial year's
  first month and how prices are quoted to that profile's usual, unless
  they have been changed by hand.
- **Create the venue** runs four stages in order: make the venue, pair this
  device, sign in, set the prefix. The card says which it is at. A stage
  that fails stops there, says why, and **Try again** carries on from that
  stage, never from the start: the venue is made once.
- A refusal that names a field (`invalid_setup`) goes back to that field's
  step and says it under the field.
- When the last stage is done the wizard shows the printer step.
- **Print a test page** adds the printer picked as "Receipt printer" (or
  uses the one the venue already has at that address), makes it the
  receipt printer, and queues a test page. Sent means the bytes reached
  it: the screen asks whether a page came out, because a printer cannot
  say. A send that failed says the queue's reason and offers Try again.
- Picking another printer and printing again retires the one the first try
  added. **Skip for now** and **Continue without it** leave no printer
  behind: one that was added and never confirmed is retired.
- **It printed**, and each way of leaving the printer (**Skip for now**,
  **Continue without it**, **Nothing came out, skip**), go to the mez Cloud
  step; **Skip for now** there, and **Continue** once paired, go to the
  last. A step left without its thing done is marked Skipped in the row.
- **Show a pairing code** makes a code for one device of the kind chosen,
  named Counter 2, Kitchen or Office until it is renamed. Choosing another
  kind takes the code off the screen: a code pairs the kind it was made
  for. No code is made until asked for, because it lasts ten minutes.
- **Add your menu** goes to the back office's Menu (`/backoffice/menu`)
  in the same tab, where the owner is still signed in: a new venue has no
  items, and a till with none sells nothing (SET-6). It has the station and
  course every item needs, a Kitchen and course 1, Main (ADR-400), so its
  first items ask for neither. **Go to the till** goes
  to `/`, where the owner lands as an owner does.
- **Send the code** and **Continue** (I had mez before) check what was
  typed first: a number needs its plus and country code, a code its six
  digits. What the server refuses is said in its words, under the field
  it names or at the head of the card.
- **Bring it back** needs a venue chosen and a name for the till. If the
  backup cannot be brought down the venue step is shown again with why,
  and nothing on the computer was changed. A sign-in that ran out goes
  back to the number.
- While mez starts again the page asks how it went and takes no answer,
  and "nothing to tell" for the first twenty seconds, as the start. When
  the venue is back it pairs this browser and shows the last card.
- **Sign in** goes to `/`, which shows Staff login: the owner's PIN is
  the one the venue had.

## States

- **Checking:** the splash, until `GET /api/v1/setup` answers.
- **Not needed** (a venue exists): the wizard is not shown. `/setup` goes to
  `/`, and the guard sends that on to Pair or Staff login.
- **Needed and here, without the setup token** (the desktop shortcut, or
  `localhost` typed into a browser, before the venue exists): no
  questions. "Open mez from the installer to set up", and how: run the
  installer again and leave Open mez now ticked, or use the setup link
  mez printed, or `mez setup-link`. Check again. The setup link opened in
  this tab afterwards is taken without a reload.
- **Needed, not here** (a tablet, or the computer by its LAN address): no
  questions. "This venue is not set up yet", and to open mez on the
  computer it runs on, by its shortcut or at `localhost`. Pair says the
  same in place of its code field (`terminal-lock.md`), in every app; on
  the server's own machine the terminal's Pair goes straight to the wizard
  and the back office's links to it.
- **The server cannot be reached:** says so, with Try again.
- **Someone else got there first** (`already_set_up`): says the venue is
  already set up and offers Pair this device.
- **The venue was made and the code was not** (`setup_no_code`), or the code
  will no longer pair: says the venue exists and that this device needs a
  code, from `mez pair` on this computer.
- **The prefix was refused** after everything else worked: says so and
  offers Continue; it is changed in Settings.
- **Searching:** "Looking on this network and this computer", for the few
  seconds it takes. **Nothing found:** says what to check. **The search
  failed:** says why; Enter its address still works. One find is picked
  for the owner, who still has to press the button.
- **A typed address** that is not an IP, or that the server refuses as not
  on this network, is said under the field.
- **No address to show** (the server listens on this computer alone, or
  could not read its own): says to find the computer's IP address in
  Windows and gives the port and path to put after it. With the name to
  show, the name comes first and the line says that is for an older
  Android tablet.
- **A code that could not be made** says why under the button.
- **Brought back, and this browser could not be paired:** the last card
  says the venue is back and that this till needs a code from `mez pair`
  on this computer; its button is **Open mez**.
- **Told nothing** (the claim ran out, the page was reloaded, or a venue
  was made here meanwhile): "Open mez again", with what to do if the
  venue is there and if it asks to pair. **Heard nothing** for ninety
  seconds: "Can't reach mez", to check it is running and open it again.
- **A venue is already here** when the restore is asked for: "Already set
  up", and **Open mez**.

## Constraints

- Opened only at `localhost` on the server's own machine, by a browser
  holding the setup token: the server refuses the post otherwise
  (ADR-266, ADR-270).
- A keyboard is assumed: a venue's name cannot be typed without one. The
  PIN fields are ordinary fields, masked, digits only, not the PIN pad.
- Fields are 48px tall and 16px text; the buttons are `tap`.
- Bringing a venue back needs the internet, for the account and the
  backup. Nothing else in the wizard does.
- Reloading while a venue is being brought back loses the claim: the
  venue is put in place all the same, and the page that comes up is the
  lock or Pair.
- Leaving or reloading the page before Create loses what was typed.
  Reloading after it leaves a venue: the device pairs with a new code.
  Reloading on the printer step or the last lands on the till; printers
  are then the back office's Printers screen, which has no search of its
  own, and codes its Devices screen.

## Reference

`docs/design/reference/mez/mez Desktop Management.dc.html`, line 770, D24.

**Left out:**

- **Tables and areas, and Menu import** (D24's steps 3 and 4). The phase
  asks for neither; the floor plan and the menu are back office screens,
  and no import exists.
- **The QR codes toggle.** Table codes need tables.
- **"About four minutes"**: nothing has timed it.
- **"Read the self-hosting guide"**: there is nowhere to send an owner who
  is offline. Task 7 writes the guide.

**Changed:**

- **Five steps, as D24 has, and they are ours:** D24's Outlet and Taxes,
  its Staff reduced to the one person who must exist, and a printer and a
  second device, which D24 does not draw.
- **The QR code is the server's.** It draws the table codes already
  (`internal/domain/qr`), so the terminal's bundle carries no encoder.
- **"your venue", not "your restaurant":** a cafe and a bar are venues.
- **The page scrolls.** D24 is a fixed 940 by 880 card; a 768px screen has
  no room for it.
- **It ends on the menu** (SET-6, ADR-398): "Start selling", on a till
  with nothing to sell, is "Add your menu", with "Go to the till" beside
  it.
- **A new venue starts with a Kitchen and course 1, Main** (ADR-400):
  every item needs a station and a course, and with exactly one of each
  nothing asks which. `mez setup` makes the same; the demo cafe brings its
  own. Under a profile that charges tax no tax class is made: a rate is
  the owner's to set. Under No tax and the GST composition scheme the venue
  starts with one, "No tax", exempt, since every item needs a class and
  there is no rate to guess (ADR-401).
- **The mez Cloud step says guests can order from their phones**, as it
  did: a table's code prints only with the public address mez Cloud gives
  (ADR-320). SET-8's "Cloud claim guests do not need" was a false reading;
  ADR-399 corrects ADR-398 on it.

- **No preview of the first invoice number.** The pattern differs by tax
  profile and the server that knows it cannot be asked before there is a
  venue.

**Kept, not drawn:** what this till is called; the stages of Create and
their failures; every state above but the questions themselves.
