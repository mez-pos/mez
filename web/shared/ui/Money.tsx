// The only way money is ever rendered (docs/design/components.md).
//
// Mono and tabular, so a column of amounts aligns on the decimal point; right-
// aligned inside whatever width it is given. `total` is the one exception to
// mono: the single total on a screen is Instrument Serif (tokens.md).

import type { Money as Amount } from "../api-types";
import { formatMoney } from "../money";
import { cx } from "./cx";

export interface MoneyProps {
  amount: Amount;
  /** `figure` for rows and columns; `total` for the one total on a screen. */
  variant?: "figure" | "total";
  /** Tone for a figure that means something: a refund, an amount due. */
  tone?: "default" | "muted" | "accent" | "danger";
  locale?: string;
  className?: string;
}

const tones = {
  default: "",
  muted: "text-fg-muted",
  accent: "text-accent-text",
  danger: "text-danger",
} as const;

export function Money({ amount, variant = "figure", tone = "default", locale, className }: MoneyProps) {
  let text: string;
  try {
    text = formatMoney(amount, locale);
  } catch {
    // A bad amount is a bug upstream. Say so where staff can see it, instead
    // of a blank or a rounded guess, and keep the rest of the screen working.
    return (
      <span role="alert" className={cx("font-mono text-danger", className)}>
        Invalid amount
      </span>
    );
  }

  return (
    <data
      value={String(amount.minor)}
      className={cx(
        "inline-block whitespace-nowrap text-right tabular-nums",
        variant === "total" ? "font-serif text-total" : "font-mono",
        tones[tone],
        className,
      )}
    >
      {text}
    </data>
  );
}
