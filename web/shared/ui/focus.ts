// The one focus rule (TECH-12, ADR-403), as tokens.md draws it. Every
// pressable thing, a button, a tile, a row, a link, a radio, takes
// focusButton: a 3px outline in the focus colour (terracotta on paper,
// mustard at night) off its edge. Every field takes focusField: its border
// turns green and a 3px green-tint halo stands around it.
//
// Selection is never drawn with these: a chosen tile or table is a green
// ring inside its edge (selectedRing), so a keyboard's place and a choice
// can both be seen at once and never be mistaken for each other.
//
// A filled button whose outline would sit on its own fill takes
// `outline-current` instead, the colour of its label.

/** A button's focus: a 3px outline in the focus colour, 2px off its edge. */
export const focusButton = "outline-offset-2 outline-focus focus-visible:outline-3";

/** A field's focus: a green border and a 3px green-tint halo. */
export const focusField = "outline-accent-tint focus-visible:border-accent focus-visible:outline-3";

/** A chosen tile or table: a 3px green ring inside its edge, apart from focus. */
export const selectedRing = "ring-3 ring-inset ring-accent";
