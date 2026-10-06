# Setup wizard

The first thing a new venue sees: the questions that make the venue, asked
once, on the computer mez runs on. Built from D24 in phase 12, Task 3b
(ADR-267), its printer step in Task 3c (ADR-268).
`web/shared/screens/setup/`: `SetupWizard.tsx`, `steps.tsx`,
`PrinterStep.tsx`, `SetupFrame.tsx`, `fields.tsx`, `setup-logic.ts`,
`printer-logic.ts` and `use-setup-status.ts`.
It is the terminal's, at `/setup`, outside the guard.

## Purpose

Take an owner who has just run the installer from an empty machine to a
till they are signed in on, with no terminal window and nothing to read
but the screen.

## Layout

One column, 58rem at most, centred on the sunken ground with D24's pale
round behind the head. It scrolls as a page; nothing is fixed.

- **Head:** the logo with "Good food brings people together" in small caps;
  "Welcome. Let's set up your venue." in the serif; one line saying how many
  steps there are and that everything can be changed later in Settings.
- **The steps:** a numbered round for each, joined by a rule, with its name
  under it. Done is the green fill and a tick, and can be gone back to; the
  one in hand is green with its number and its name in green; the rest are
  sunken.
- **The card:** the step's title in the serif, a line under it, its fields,
  and along the foot "Step 2 of 4", **Back** and the green **Continue**
  with its arrow (**Create the venue** on the last of the questions).
- **Foot:** a script line, and where the wizard is being served from.

The steps:

1. **Your venue.** Venue name; name on invoices (optional, "if it differs");
   currency; time zone; what this till is called.
2. **Taxes and invoices.** The tax profile, as four cards (No tax, GST
   India, VAT UK, Sales tax US); the registration number, where the profile
   has one; prices include tax or tax is added, where the profile lets that
   be chosen; the month the financial year starts; the invoice prefix.
3. **You.** Your name; a PIN of 6 to 8 digits; the PIN again. What the PIN
   is for, and that more people are added later in the back office.
4. **Printer**, after Create, when the venue exists and the owner is signed
   in. **Printers found**, with Search again: this computer's own printers
   first (by Windows' name), then each address on the network that answers
   on the printer port, each a row to pick, marked "already added" where
   the venue has it; under them always **Enter its address**, which opens
   an address field. What was searched is said under the list. Once one is
   picked: **Paper**, 80mm or 58mm. Then what became of the test page. The
   foot has **Skip for now** and the green **Print a test page**; after a
   page is sent, **Print again** and the green **It printed**.

## Data

- `GET /api/v1/setup`, key `["setup"]`: `{needed, here}`. No token.
- `POST /api/v1/setup` (`SetupRequest`): makes the venue. No token.
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

Nothing is subscribed to. Nothing typed is kept anywhere but the page: the
PIN is in memory until the wizard is left.

## Interactions

- **Continue** checks the step's own fields and says what is missing under
  each. It does not ask the server until the last step.
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
- **It printed**, and either way of leaving, go to `/`, where the owner is
  signed in and lands as an owner does.

## States

- **Checking:** the splash, until `GET /api/v1/setup` answers.
- **Not needed** (a venue exists): the wizard is not shown. `/setup` goes to
  `/`, and the guard sends that on to Pair or Staff Login.
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

## Constraints

- Opened only at `localhost` on the server's own machine: the server
  refuses the post from anywhere else (ADR-266).
- A keyboard is assumed: a venue's name cannot be typed without one. The
  PIN fields are ordinary fields, masked, digits only, not the PIN pad.
- Fields are 48px tall and 16px text; the buttons are `tap`.
- Leaving or reloading the page before Create loses what was typed.
  Reloading after it leaves a venue: the device pairs with a new code.
  Reloading on the printer step lands on the till; printers are then the
  back office's Printers screen, which has no search of its own.

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

- **Four steps, not five, and they are ours:** D24's Outlet and Taxes, its
  Staff reduced to the one person who must exist, and a printer, which D24
  does not draw. The last screen is Task 3d's.
- **"your venue", not "your restaurant":** a cafe and a bar are venues.
- **The page scrolls.** D24 is a fixed 940 by 880 card; a 768px screen has
  no room for it.

- **No preview of the first invoice number.** The pattern differs by tax
  profile and the server that knows it cannot be asked before there is a
  venue.

**Kept, not drawn:** what this till is called; the stages of Create and
their failures; every state above but the questions themselves.
