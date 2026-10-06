# Back office: Settings

How the venue is set up: who it is, how it taxes and numbers its invoices,
when its day starts, how tips are shared and what a receipt says. Built in
phase 10 Task 6 (ADR-129 to ADR-138) and given D22's frame in the rebuild's
Task 16i (ADR-193). `web/backoffice/src/settings/`: `SettingsScreen.tsx`,
`SectionCard.tsx`, `VenueSection.tsx`, `TaxSection.tsx` (`TaxProfileCard`,
`TaxClassesCard`, `NumberingCard`), `ShiftsSection.tsx` (`RulesCard`,
`ReceiptCard`, `ReceiptPaper`), `use-section-draft.ts`, `settings-logic.ts`.
Section **Settings**, `settings.edit`, keys `g v`. Stations and courses are
the menu's (`backoffice-menu.md`).

## Purpose

An owner changes how the venue is set up in one place, one section at a time,
and always knows whether what is on the screen is what the venue is running
on.

## Layout

Inside the frame (`backoffice-shell.md`). "Settings" in the serif, then a rail
beside the section open; under 1024px the rail sits above it.

- **The rail, 290px, a card:**
  - The sections, an icon and a name each, 48px, the current one the
    terracotta fill. One not built is muted with a faint **SOON** and is not
    a link.

    | Section | Path |
    |---|---|
    | Venue | `/settings/venue` |
    | Tax and numbering | `/settings/tax` |
    | Shift, tips and receipt | `/settings/shifts` |
    | Backup | `/settings/backup` (phase 12; Soon) |

    `/settings`, and a path that is none of these, go to Venue.
  - **On their own screens**, links out, each shown to whoever may open it:
    **Printers**, **Devices**, **Tables and floor plan**.
  - **The version box:** "mez v1.0.0" and "Open source · AGPL-3.0 ·
    self-hosted on your own hardware."
- **The section**, across the rest: one or more cards, each with its title
  in the serif, one line saying what it is for, its fields, and a foot with
  the state at the left and **Discard** and **Save** (the primary) at the
  right (`SectionCard`).

### Venue

"Who the venue is, as it appears on the lock screen and on invoices." Fields
two across.

- **Venue name**, required, 60 characters at most. "Shown on the lock screen
  and on receipts."
- **Legal name**, 120 at most. "Printed on invoices. Leave empty to use the
  venue's name."
- **Timezone**, with the browser's list as suggestions. "Like Asia/Kolkata.
  Decides where a day starts and what time a receipt shows."
- **Currency**, read only: the code and "Cannot be changed: every amount
  recorded would change what it means."

Under it, a card of its own, **Paid from a phone** (ADR-215, ADR-216, ADR-229,
ADR-231;
`PaymentsCard.tsx`, `payments-logic.ts`, `use-payments.ts`): "A guest reading
their bill on their phone can pay it by UPI, straight to your account. They
show you the payment, and you record it on the terminal as you would one
made at the counter." It reads and saves on its own
(`GET`/`PUT /api/v1/settings/payments`), so a failure to read it leaves the
venue's card usable. Fields one across.

- **UPI address**, in the monospace. "Like cafe@okbank: the one on your
  counter's UPI sticker. Leave empty to offer no UPI on a guest's phone."
  Refused unless it is written like one: "Enter a UPI address like
  cafe@okbank: a name, an @, and the bank's handle."
- **Name the guest sees**, 60 characters at most, the venue's name as its
  placeholder. "What a guest's UPI app shows before they pay. Leave empty to
  use the venue's name, Demo Cafe."
- **A warning, always there**, in the warning tone: "mez cannot check this
  address. It only checks that it is written like one. A wrong address sends
  your guests' money to someone else. After saving, open a bill on your own
  phone and pay it one rupee: your UPI app should show Filter Coffee, and
  the rupee should arrive in your account." The name in it follows what is
  typed.
- **Where things stand**, from what is saved and not what is typed: "A
  guest's bill offers "Pay by UPI" to cafe@okbank.", or "No address saved: a
  guest's bill offers no UPI, and says to pay staff or at the counter.", or
  for a venue not in rupees, "UPI pays in rupees only, and this venue trades
  in USD: a guest's bill will not offer it, whatever is entered here."
- **Paying online**, a block that is read and not edited, from `online` on
  the settings' answer. A gateway and its keys are the venue's
  configuration, so nothing here changes them and no key is shown.
  - no gateway, plain: "Guests cannot pay online. No payment gateway is set
    for this venue. One is set in the venue's configuration, with its keys,
    and not on this screen."
  - a gateway, titled "Guests can pay online through Stripe", then three
    lines:
    - "The last payment through it arrived Tue, 6 Oct 2026 · 10:02 AM.", or,
      with the whole block in the warning tone, "No payment has arrived
      through it yet. Before relying on it, pay a small bill yourself from a
      phone and see it land here."
    - "Stripe tells this venue as soon as a payment is made.", or "This
      venue asks Stripe about each payment every few seconds, so one can
      take a moment to show."
    - "After paying, a guest is brought back to https://cafe.example/order/.",
      or "This venue's address is not set, so a guest is not brought back
      here after paying. It is set in the venue's configuration."
- **A guest who orders from their phone pays**, under a rule, two choices
  side by side: **When they are ready to leave**, the default, and **Before
  the order is sent**. It saves with the rest of the card: the address and
  the mode go in one request, so saving one never loses the other. Under
  the choices, what the one chosen does, following what is chosen and not
  what is saved:
  - on leaving: "A guest's first order waits for a member of staff to
    accept it on the terminal. They pay when they are ready to leave, as
    they would with a waiter."
  - before, with a gateway: "A guest pays online for each order as they
    place it. Nothing reaches the kitchen until it is paid for, and a paid
    order goes straight there: staff do not accept it first."
  - before, with no gateway, in the warning tone: "This has no effect yet:
    this venue has no payment gateway, so a guest cannot pay online. Orders
    still wait for staff, and guests pay when they leave. A gateway is set
    in the venue's configuration, not on this screen." It can still be
    chosen and saved, and takes effect when a gateway is set.

### Tax and numbering

Three cards. The profile and the numbering each save on their own; the tax
classes save at once.

- **Tax profile.** **Tax**, a choice: No tax, GST (India), VAT (UK), US
  sales tax. What it needs follows it:
  - GST and VAT: **Prices**, "Prices include tax" or "Tax is added on top".
  - GST: **GSTIN** and **Place of supply**, the two digit state code.
  - VAT: **VAT number**.
  - US sales tax: **Rates where the venue is**, a row per authority (a name
    and a percentage), **Add a rate**.
  - A draft of another profile warns: "Changing the profile is refused
    while any order is open. Settle or void them first."
  - **Items without an HSN**, on a profile that prints one on every line:
    "3 items have no HSN or SAC code. They still sell, but their invoices
    fall short until each is edited.", each a link to `/menu?item=<id>`; at
    most eight, then "and N more in the menu".
- **Tax classes.** A row per class: its name, its rate ("5%", "Exempt"),
  and **Edit** and **Retire**, or **Bring back** for a retired one. **Add a
  tax class** opens a modal: name; "Charged at a rate" or "Exempt"; the rate
  ("The whole rate, like 5 or 12. Under GST it is split in half into CGST
  and SGST on the invoice."). A retire is refused while items still charge
  the class, and names the first eight.
- **Invoice numbering.** For **Invoices** and **Credit notes**, a **Series
  code** and a **Prefix**; the **Number format**; **Financial year starts
  in**, a month, shut once a number has been issued; **Round cash to the
  nearest**, with the honest note that payments do not apply it yet.
  - **Next numbers**, a sunken panel: "The next numbers are" as saved; with
    a draft, "If you save, the next numbers are", from the server, 350ms
    after typing pauses. A draft it would refuse shows "No preview yet:" and
    why.

### Shift, tips and receipt

Two cards, each saving on its own.

- **Shifts, tips and orders.**
  - **The trading day starts at**, a time. "Sales before this time belong
    to the day before." A shift keeps the day it opened under.
  - **A drawer may be out by (INR)**: "Over or short by more than this,
    closing the shift needs a reason. 0 asks for a reason on any
    difference."
  - **Tips are shared**: Shared evenly, By hours worked, By role weight.
    By role weight: **Weight of each role**, a whole number from 1 to 100;
    one left empty counts as 1.
  - **An order is late after (minutes)**, 1 to 600: "The order screen
    shows its time as late once it has been open this long."
- **Receipt.**
  - **Under the venue's name** and **At the bottom**: up to four lines of 48
    characters each. The printer prints plain letters and digits; text it
    cannot print is refused with the line and the character.
  - **Print**, three boxes: the invoice number as a barcode, the order
    reference, the choices under each item.
  - **Preview**, a sunken panel: **Paper**, 80mm or 58mm, and a sample
    receipt on a paper strip as wide as the printer's line, laid out by the
    code that prints a real one: the venue's real name, registration and
    next invoice number, made-up items, and the draft template.

## How a section saves

Every card with a foot behaves the same.

- **Nothing is saved until Save.** Typing changes a draft held on the
  screen. Save and Discard are shut while the draft equals what the server
  has.
- **Unsaved changes** shows at the left of the foot, in words, as soon as
  the draft differs.
- **Save** sends the whole card. The button reads "Saving". On success the
  form shows what the server answered and the foot says **Saved**, in green,
  until the next edit. On refusal the draft is kept and the server's words
  appear under the field they are about; a refusal about no field appears in
  the foot.
- **Discard** puts the draft back and clears errors.
- **Leaving with unsaved changes**, by another section or a link out: "Leave
  without saving?", "Your changes to Venue are not saved.", **Keep editing**
  and **Leave**. Closing or reloading the tab asks through the browser's own
  prompt. Leaving by the sidebar does not ask, and loses the draft.
- Enter in a field saves: a card is a form.

## Data

All need `settings.edit`.

- `GET /api/v1/settings` and `PUT /api/v1/settings/venue` `{name,
  legal_name, timezone}`.
- `GET` and `PUT /api/v1/settings/tax`; `POST /api/v1/settings/tax-classes`,
  `PUT .../{id}`, `POST .../{id}/retire` and `.../restore`. Rates are
  thousandths of a percent, shown as percent.
- `GET` and `PUT /api/v1/settings/numbering`, and `POST
  /api/v1/settings/numbering/preview`, which saves nothing.
- `GET` and `PUT /api/v1/settings/rules` (`day_start`, `variance_threshold`,
  `tip_rule`, `tip_role_weights`, `ticket_late_minutes`).
- `GET` and `PUT /api/v1/settings/receipt`, and `POST
  /api/v1/settings/receipt/preview` with `paper_mm`.
- Each write answers with its section as it now stands, which replaces the
  cache.

## States

- **Loading:** "Loading the settings", "Loading the tax settings", "Loading
  the shift and receipt settings"; the rail is already there.
- **Could not load:** the reason, with **Try again**.
- **Refused:** the server's own words, as sent: "Give the venue a name.",
  "That is not a timezone name. Use one like Asia/Kolkata or
  Europe/London."
- **Offline:** "Can't reach the mez server. Check this computer's
  connection." The draft is kept.
- **No tax classes:** "No tax classes yet. Items need one, so add the
  first."
- **Not allowed:** the frame's "That is not yours to open".

## Constraints

- One primary per card: **Save**. Nothing else is green.
- Every control 48px, form controls 16px.
- Errors in words under the field; the currency is shown, never edited.
- A save reaches an order screen when it next reads the venue, at its next
  sign-in or reload.
- A renamed role orphans its tip weight; it is not shown and drops at the
  next save.
- The cash rounding step is saved and nothing applies it yet (ADR-132).

## Reference

`mez Desktop Management.dc.html`, D22 Settings (line 602). Left out, and
why:

- **Integrations, and Language & theme:** there are none. The kitchen's
  night theme is the display's own switch.
- **Backup & restore** ("Download backup now", "Restore from file"): phase
  12. The rail holds its place, as Soon.
- **Address, Phone and Service hours** as venue fields: they print through
  the receipt's own lines.
- **Service charge:** not in v1.
- **Generate QR and Print sheet:** phase 11.

Changed: the design draws every section on one screen with one **Save
settings**. Here a section is a page and each card saves itself (ADR-129):
the sections write through different routes with different checks, and one
button over all of them would save some and refuse others. Its "Printers &
devices" and "Tables & QR codes" are screens of their own, so the rail
links out to them. "Outlet" is **Venue**; the GSTIN is on the tax profile.
CGST and SGST are not typed as two rates: a tax class has one rate and GST
splits it.
