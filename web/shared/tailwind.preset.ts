// Tailwind preset shared by all three apps. Every value is a CSS variable from
// tokens.css; a hex value in this file is a bug. See docs/design/tokens.md.
//
// Colours, font sizes, radii and shadows *replace* Tailwind's defaults rather
// than extending them, so `bg-red-500`, `text-xs` and `rounded-lg` do not
// exist and cannot slip in. Colour classes name roles (`bg-raised`,
// `text-accent`), which resolve per [data-surface], never paper or night hexes.
//
// Spacing is left as Tailwind's native scale. The allowed steps (2, 4, 6, 8,
// 10, 12, 14, 16, 20, 24, 32, 48, 64 px) are native classes already; the
// limit is enforced by review, as tokens.md explains.
//
// Each app's index.css loads this with `@config`, Tailwind v4's bridge to a
// JS config. `satisfies Config` means a misspelt theme key fails `tsc`.

import type { Config } from "tailwindcss";

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
        heading: v("fg-heading"),
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
        tint: v("accent-tint"),
      },

      // Terracotta: the second voice. New order, export, the active nav item.
      attention: {
        DEFAULT: v("attention"),
        hover: v("attention-hover"),
        fg: v("attention-fg"),
        text: v("attention-text"),
        tint: v("attention-tint"),
      },

      highlight: v("highlight"),
      // The logo's arches and wordmark; its top is `highlight`.
      logo: v("logo-ink"),

      // Status chips: a background, border and text per tone. See tokens.md.
      chip: Object.fromEntries(
        (["neutral", "success", "warning", "danger", "accent", "info"] as const).map((tone) => [
          tone,
          { bg: v(`chip-${tone}-bg`), border: v(`chip-${tone}-border`), fg: v(`chip-${tone}-fg`) },
        ]),
      ),

      success: v("status-success"),
      warning: v("status-warning"),
      danger: {
        DEFAULT: v("status-danger"),
        fill: v("danger-fill"),
        tint: v("danger-tint"),
        "tint-hover": v("danger-tint-hover"),
        line: v("danger-line"),
      },
      info: v("status-info"),
    },

    fontFamily: {
      serif: v("font-serif"),
      sans: v("font-sans"),
      // The numeral face. The class keeps its name; it is not a monospace.
      mono: v("font-mono"),
      script: v("font-script"),
    },

    fontSize: {
      meta: [v("text-meta"), { letterSpacing: v("tracking-meta") }],
      sm: v("text-sm"),
      base: v("text-base"),
      lg: v("text-lg"),
      xl: v("text-xl"),
      total: v("text-total"),
      kitchen: v("text-kitchen"),
      display: v("text-display"),
      hero: v("text-hero"),
    },

    borderRadius: {
      none: v("radius-none"),
      DEFAULT: v("radius"),
      tight: v("radius-tight"),
      card: v("radius-card"),
      sheet: v("radius-sheet"),
      pill: v("radius-pill"),
      round: v("radius-round"),
    },

    // Warm on paper, nearly absent on night: each resolves per surface.
    boxShadow: {
      none: "none",
      rest: v("shadow-rest"),
      card: v("shadow-card"),
      modal: v("shadow-modal"),
      action: v("shadow-action"),
    },

    extend: {
      spacing: {
        tap: v("tap-min"),
        charge: v("tap-charge"),
        "tap-gap": v("tap-gap"),
      },
    },
  },
} satisfies Config;

export default preset;
