# Back office: Venue settings

The frame every venue setting sits in, and its first section, the venue's own
details. Built in phase 10 Task 6g (ADR-136). Sections 6h to 6j (tax and
numbering; shift, tips and receipt; stations and courses) fill the frame
later, each with the same behaviour.

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
  | Stations and courses | `/settings/stations` | Task 6j |
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
- Both need `settings.edit`.
- Later sections add their own routes and keys (`/api/v1/settings/tax`,
  `/numbering`, `/rules`, `/receipt`); each answers with its section as it now
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
