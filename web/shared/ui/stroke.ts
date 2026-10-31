// Icon strokes (tokens.md, Icons; TECH-22, ADR-405). Every app sets the
// default once, on its LucideProvider: 1.75px, the design's line. These are
// the only others, each for one reason; an icon that needs none of them
// takes the default and says nothing.

export const stroke = {
  /** The default, set on each app's LucideProvider. */
  base: 1.75,
  /** A large watermark drawn faint behind a figure: the leaf. */
  hairline: 1,
  /** An icon drawn at 30px or more, so its line weighs what a 24px one's does. */
  large: 1.5,
  /** A mark read at a glance: an arrow or a plus on a filled button, a check, a cross. */
  bold: 2.25,
} as const;
