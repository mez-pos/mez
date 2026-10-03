# Back office: Venue settings

The frame every venue setting sits in, its first section (the venue's own
details, Task 6g, ADR-136), its second (tax and numbering, Task 6h, ADR-137)
and its third (shift, tips and receipt, Task 6i, ADR-138). Backup (Task 6l)
fills the frame later, with the same behaviour. Stations and courses are not
here: they are the menu editor's (`backoffice-menu.md`, ADR-139).

## Purpose

An owner changes how the venue is set up in one place, one section at a time,
and always knows whether what is on the screen is what the venue is running
on.

## Layout

Paper surface, inside the back office shell, reached from **Venue settings**
(`/settings`, key `g v`). Content is `max-w-5xl`.

- **Title** "Venue settings" in the serif (`text-total`). No "Back office"
  link above it: the shell's navigation is the way out.
- **Section rail**, a column 220px wide left of the form; a wrapped row above
  it on a narrow screen. One entry per section, 48px, sentence case, as pills
  like the shell's navigation: the current one `aria-current="page"`,
  terracotta fill, semibold. An entry whose section is not built is muted with
  a faint uppercase **SOON**, and is not a link (the shell's rule, for the
  same reason).
  | Section | Path | Built in |
  |---|---|---|
  | Venue | `/settings/venue` | this task |
  | Tax and numbering | `/settings/tax` | Task 6h |
  | Shift, tips and receipt | `/settings/shifts` | Task 6i |
  | Backup | `/settings/backup` | Task 6l |
  `/settings` goes to `/settings/venue`; a path that is none of these goes
  there too.
- **The section**: one `Card` (padding 32) with
  - the section's title in the serif (`text-xl`) and one line saying what
    it is for (`text-base`, secondary);
  - its fields, one per row, in a column no wider than 560px;
  - a footer row, a hairline above it: the state at the left, **Discard**
    (secondary) and **Save** (primary) at the right.
  The footer is the same for every section (`SectionCard`).

### Venue section

- **Venue name**, text, required, 60 characters at most. "Shown on the lock
  screen and on receipts."
- **Legal name**, text, 120 at most. "Printed on invoices. Leave empty to use the
  venue's name."
- **Timezone**, text with the browser's list of zones as suggestions. "Like
  Asia/Kolkata. Decides where a day starts and what time a receipt shows."
- **Currency**, read only: the code in the numeral face ("INR") and "Cannot
  be changed: every amount recorded would change what it means." It is not a
  control.

### Tax and numbering section

Three cards one below the other. The profile and the numbering each save on
their own, as a section does (below); the tax classes save at once.

- **Tax profile** (a `SectionCard`). **Tax**, a choice: No tax, GST (India),
  VAT (UK), US sales tax. What the choice needs follows it:
  - GST and VAT: **Prices**, a choice: "Prices include tax" or "Tax is added
    on top".
  - GST: **GSTIN** (numeral face; "Invoices print it. Capitals and spaces are
    tidied for you.") and **Place of supply**, the two digit state code, short
    ("Empty takes the first two digits of the GSTIN.").
  - VAT: **VAT number**.
  - US sales tax: "Tax is always added on top of the price under US sales
    tax." and **Rates where the venue is**: one row per authority, a name and
    a percentage, **Remove** on each, **Add a rate**. "Each is added together
    on the bill."
  - When the draft is another profile than the saved one: "Changing the
    profile is refused while any ticket is open. Settle or void them first.
    Tickets already paid keep the tax they were charged." The refusal itself
    (`tickets_open`) arrives in the footer.
  - **Items without an HSN.** While the form is the saved profile and that
    profile prints an HSN or SAC code on every line, any item without a valid
    one is listed in a sunken panel: "3 items have no HSN or SAC code. They
    still sell, but their invoices fall short until each is edited.", each a
    link (a pill) to `/menu?item=<id>`, which opens that item's form; at most
    eight, then "and N more in the menu". The list is the server's answer to
    the last save, so a change of profile shows what it leaves owing as soon as
    it is saved. A draft of another profile hides the list.
- **Tax classes** (a `Card`, no draft). A heading, one line ("A rate that
  items share. A new rate is for lines rung up from then on: tickets already
  open keep theirs.") and **Add a tax class**. One row per class: its name, its
  rate in the numeral face ("5%", "2.5%", "Exempt") and **Edit** and **Retire**
  (danger) or, for a retired class, "Retired" under a struck name and **Bring
  back**. **Add** and **Edit** open a modal: name; **Tax**, a choice "Charged at
  a rate" or "Exempt"; **Rate** as a percentage (hidden when exempt; "The whole
  rate, like 5 or 12. Under GST it is split in half into CGST and SGST on the
  invoice."). The server's refusals are shown as sent: a name in use, a rate
  outside 0 to 100, and for Retire the items that still charge the class (the
  first eight, then "and N more"), in a line under the list.
- **Invoice numbering** (a `SectionCard`). **Invoices** and **Credit notes**
  each a **Series code** (capitals and digits, 8 at most, its own counter) and
  a **Prefix** ("Follows the code unless you change it": typing a code carries
  the prefix with it while the two were the same). **Number format**, the
  template in the numeral face, with what its parts mean and, under GST, the
  length limit. **Financial year starts in**, a month; disabled once a number
  has been issued, with the reason. **Round cash to the nearest**, in the
  currency's major units (0.50, not 50), the range beside it, and the honest
  note that payments do not apply it yet.
  - **Next numbers.** A sunken panel: "The next numbers are" the invoice and
    credit note numbers as saved, in the numeral face. As soon as the draft
    differs it reads "If you save, the next numbers are" and shows what the
    server works out for the draft (`POST /api/v1/settings/numbering/preview`,
    350ms after typing pauses; nothing is saved). A draft the server would
    refuse shows "No preview yet:" and the server's reason, the same words Save
    would give under the field.

### Shift, tips and receipt section

Two cards, each a `SectionCard` saving on its own.

- **Shifts, tips and tickets.**
  - **The trading day starts at**, a time input (24 hour value, like 04:00). "Sales
    before this time belong to the day before. A shift keeps its day when this
    changes: a new start applies from the next shift. Reports use the new start
    for every day." When a shift is open its day is named: "The shift open now
    keeps its day, 2026-10-02."
  - **A drawer may be out by (INR)**, an amount in major units in the numeral
    face, with the currency's decimals ("0.00"; yen has none). "Over or short
    by more than this, closing the shift needs a reason. 0 asks for a reason on
    any difference."
  - **Tips are shared**, a choice: Shared evenly, By hours worked, By role
    weight. By hours: "Each person's share follows the hours entered when the
    tips are pooled." By role weight: **Weight of each role**, one row per role
    the venue has (a label and a short number, placeholder 1): "A whole number
    from 1 to 100. A role left empty counts as 1: a manager at 2 takes twice a
    cashier's share." Tips shared by role need at least one weight. Weights kept
    while another rule is chosen are saved and unused. A weight stored for a
    role that has since been renamed or removed is not shown, and is gone at the
    next save: the server refuses a weight for a role it does not know.
  - **A ticket is late after (minutes)**, a whole number from 1 to 600: "The till
    shows its time as late once it has been open this long."
- **Receipt.** What the receipt, a bill printed before payment and a credit
  note say beyond what the tax profile requires, which always prints.
  - **Under the venue's name** and **At the bottom**: a text area each, one line
    per row, up to four lines of 48 characters; blank lines are dropped, a
    pasted block is its lines. The printer prints plain letters and digits:
    accents print without them, other scripts cannot be printed yet, and such
    text is refused with the line and the character.
  - **Print**, three boxes (the whole row is the target): the invoice number as a
    barcode, the order reference (the table number always prints), the choices
    under each item. They are ticked as the receipt is today; the server stores
    what is turned off (`hide_*`).
  - **Preview.** A sunken panel: **Paper**, a choice of 80mm or 58mm, and a sample
    receipt drawn on a paper strip as wide as the printer's line (48 or 32
    characters, in a monospace face so the columns line up): the venue's real
    name, registration, tax profile and next invoice number, made-up items and
    amounts, and the draft template. The lines come from the server
    (`POST /api/v1/settings/receipt/preview`), laid out by the code that prints a
    real receipt and replayed as paper: centred and bold lines as printed, the
    barcode drawn as bars over its number, wrapping where the printer wraps.
    It updates 350ms after typing pauses and keeps the last receipt on screen
    while the next is worked out. A draft the server would refuse (too many
    lines, text it cannot print) shows "No preview yet:" and the server's
    reason, the same words Save would give. The credit note takes the same header
    and footer; only the receipt is previewed.

## How a section saves

Every section behaves the same, so that the next ones need no new rules.

- **Nothing is saved until Save.** Typing changes a draft held on the screen.
  Save and Discard are disabled while the draft equals what the server has.
- **Unsaved changes** shows at the left of the footer, in words ("Unsaved
  changes"), as soon as the draft differs. It is not a `StatusChip`: it is
  not the status of anything the venue owns.
- **Save** sends the whole section. The button reads "Saving" and everything
  is disabled while it is out. On success the form shows what the server
  answered, the footer says **Saved** with a check, in green, and stays until
  the next edit. On refusal the draft is kept and the server's words appear
  under the field they are about; a refusal about no field appears in the
  footer.
- **Discard** puts the draft back to what the server has, and clears errors.
- **A section may have several forms** (Tax and numbering, and Shift, tips and
  receipt, have two with a draft): each is dirty, saves and discards on its own, and leaving the section
  asks when any of them is dirty.
- **Leaving a section with unsaved changes** (another entry in the rail) asks:
  "Leave without saving?", "Your changes to Venue are not saved.", **Keep
  editing** (secondary) and **Leave** (primary). Leaving by the shell's
  navigation does not ask, and loses the draft; closing or reloading the
  browser tab asks, through the browser's own prompt.
- A save moves nothing on the till until it reads the venue again, which it
  does on its next sign-in or reload (the venue's name on the lock screen is
  read when it is shown).

## Data

- `GET /api/v1/settings`: `venue` (`name`, `legal_name`, `currency`,
  `timezone`) and `settings[]` (every key, for later sections). React Query
  key `["settings"]`.
- `PUT /api/v1/settings/venue` `{ name, legal_name, timezone }`: answers with
  the same shape as the GET; the answer replaces the cache.
- Tax and numbering: `GET`/`PUT /api/v1/settings/tax` (`profile`,
  `registration_id`, `pricing`, `state_code`, `jurisdiction[]`; answers with
  those and `needs_hsn`, `items_without_hsn[]`, `classes[]`), `POST
  /api/v1/settings/tax-classes`, `PUT /api/v1/settings/tax-classes/{id}`,
  `POST .../{id}/retire` and `.../restore` (each answers with the tax settings
  as they now stand; rates are thousandths of a percent, shown as percent),
  `GET`/`PUT /api/v1/settings/numbering` and `POST
  /api/v1/settings/numbering/preview` (the PUT body; answers with the numbering
  and the next numbers, saving nothing). Cache keys `["settings","tax"]` and
  `["settings","numbering"]`.
- Shift, tips and receipt: `GET`/`PUT /api/v1/settings/rules` (`day_start`,
  `variance_threshold` in minor units, `tip_rule`, `tip_role_weights` by role
  name, `ticket_late_minutes`; answers with those and `currency`,
  `minor_per_major`, `roles`, `open_shift_day`), `GET`/`PUT
  /api/v1/settings/receipt` (`header[]`, `footer[]`, `hide_barcode`,
  `hide_order_ref`, `hide_modifiers`; answers with those and `max_lines`,
  `max_line_chars`) and `POST /api/v1/settings/receipt/preview` (the same body
  and `paper_mm` 80 or 58; answers with `columns` and `lines[]` of `text`,
  `align`, `width`, `height`, `bold`, `barcode`; saves nothing). Cache keys
  `["settings","rules"]` and `["settings","receipt"]`.
- All need `settings.edit`.
- Later sections add their own routes; each answers with its section as it now
  stands, as these do.

## Interactions

| Action | Does | Then |
|---|---|---|
| Edit a field | Changes the draft | "Unsaved changes"; Save and Discard enable |
| Save | Sends the section | "Saved"; the draft is what the server holds |
| Save, refused | Keeps the draft | The reason under its field |
| Discard | Restores the server's values | Errors clear |
| Another section in the rail | Opens it | Asks first when there are unsaved changes |
| Enter in a field | Saves | As Save (the section is a form) |

## States

- **Loading:** "Loading the settings" in place of the section; the rail is
  already there.
- **Could not load:** the reason, with **Try again**.
- **Refused:** "Give the venue a name.", "Keep the name to 60 characters.",
  "That is not a timezone name. Use one like Asia/Kolkata or Europe/London.",
  "Choose a timezone, like Asia/Kolkata." (the server's own words, shown as
  sent). "Only someone who can change the venue's settings can do this." for a
  `forbidden`.
- **Offline / unreachable:** "Can't reach the mez server. Check this
  computer's connection." The draft is kept.
- **Not allowed:** the shell's "That is not yours to open", with
  `settings.edit` in the numeral face.
- **Tax and numbering, loading:** "Loading the tax settings"; if either read
  fails, the reason with **Try again**.
- **No tax classes:** "No tax classes yet. Items need one, so add the first."
- **Shift, tips and receipt, loading:** "Loading the shift and receipt
  settings"; if either read fails, the reason with **Try again**.

## Constraints

- One primary on the screen: **Save**. Nothing else is green. **Discard** is
  secondary.
- Every control 48px, form controls 16px minimum.
- Errors in words under the field and `role="alert"`; the saved message
  `role="status"`.
- The currency is shown, never edited.
- Every section's title is the serif; nothing in a section is smaller than
  13px.

## Reference

The archive's Settings screen (`mez Desktop Management`, "D22 Settings") has a
left list of sections and cards of fields, with one **Save settings** and
**Discard** at the very end. Here each section has its own Save and Discard,
because the sections write through different routes with different checks and
one button over all of them would save some and refuse others. The archive's
Outlet card (name, GSTIN, address, phone, hours) maps to the Venue section
here, the GSTIN to the tax section (6h); address, phone and hours print on
receipts through the receipt template (6i) and are not venue fields.
