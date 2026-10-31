// One input (TECH-23, ADR-406): the look of every field a person types in or
// chooses from, in every app. A strong hairline on the raised ground, the
// field's focus (focus.ts), the danger border when it is refused, and a
// placeholder in the muted ink.

import { focusField } from "./focus";

/** Any field's look, with no size: add the height, width and padding where it stands. */
export const fieldClass = `rounded border border-line-strong bg-raised text-fg placeholder:text-fg-muted aria-invalid:border-danger disabled:opacity-60 ${focusField}`;

/** A one-line field filling its row, 48px tall: the form's input or select. */
export const inputClass = `min-h-tap w-full px-4 ${fieldClass}`;
