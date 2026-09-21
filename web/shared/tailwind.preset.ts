// Tailwind preset shared by all three apps. Every value is a CSS variable from
// tokens.css; a hex value in this file is a bug. See docs/design/tokens.md.
//
// Colours, font sizes, radii and shadows *replace* Tailwind's defaults rather
// than extending them, so `bg-red-500`, `text-xs` and `rounded-lg` do not
// exist and cannot slip in. Colour classes name roles (`bg-raised`,
// `text-accent`), which resolve per [data-surface], never ink or paper hexes.
//
// Spacing is left as Tailwind's native scale. The allowed steps (2, 4, 6, 8,
// 10, 12, 14, 16, 20, 24, 32, 48, 64 px) are native classes already; the
// limit is enforced by review, as tokens.md explains.
//
// No Tailwind import yet: the apps that install Tailwind arrive in Task 2,
// which adds the `satisfies Config` check.

const v = (name: string): string => `var(--${name})`;

const preset = {
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      inherit: "inherit",

      canvas: v("canvas"),
      chrome: v("chrome"),
      raised: { DEFAULT: v("raised"), high: v("raised-high") },
      sunken: v("sunken"),

      fg: {
        DEFAULT: v("fg"),
        2: v("fg-2"),
        3: v("fg-3"),
        muted: v("fg-muted"),
        faint: v("fg-faint"),
      },

      line: {
        DEFAULT: v("line"),
        soft: v("line-soft"),
        strong: v("line-strong"),
        focus: v("line-focus"),
      },

      accent: {
        DEFAULT: v("accent"),
        hover: v("accent-hover"),
        fg: v("accent-fg"),
        text: v("accent-text"),
      },

      success: v("status-success"),
      warning: v("status-warning"),
      danger: v("status-danger"),
      info: v("status-info"),
    },

    fontFamily: {
      serif: v("font-serif"),
      sans: v("font-sans"),
      mono: v("font-mono"),
    },

    fontSize: {
      meta: [v("text-meta"), { letterSpacing: v("tracking-meta") }],
      sm: v("text-sm"),
      base: v("text-base"),
      lg: v("text-lg"),
      xl: v("text-xl"),
      total: v("text-total"),
    },

    borderRadius: {
      none: v("radius-none"),
      DEFAULT: v("radius"),
      tight: v("radius-tight"),
      round: v("radius-round"),
    },

    // Elevation on ink is surface lightness. tokens.md names no paper shadow
    // value yet; add one there before adding it here.
    boxShadow: {
      none: "none",
    },

    extend: {
      spacing: {
        tap: v("tap-min"),
        charge: v("tap-charge"),
        "tap-gap": v("tap-gap"),
      },
    },
  },
};

export default preset;
