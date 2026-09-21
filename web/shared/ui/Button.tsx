// The button. Four variants, four sizes (docs/design/components.md).
//
// Every label is --text-base, even on `sm`: the 15px floor for anything
// interactive is not relaxed for density (tokens.md, ADR-014). `sm` and `md`
// are for keyboard-and-mouse screens in the back office; anything touched
// during service is `tap` or larger, which is why `tap` is the default.

import type { ButtonHTMLAttributes } from "react";
import { cx } from "./cx";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "tap" | "charge";

const variants: Record<ButtonVariant, string> = {
  // Vermilion means money, action or now: one per screen.
  primary: "bg-accent text-accent-fg hover:bg-accent-hover",
  secondary: "border border-line-strong bg-raised-high text-fg hover:border-line-focus",
  ghost: "text-fg-2 hover:bg-raised-high hover:text-fg",
  // Outlined, not filled: a void is serious, and a filled red button next to
  // a vermilion one reads as two calls to action.
  danger: "border border-chip-danger-border text-danger hover:bg-chip-danger-bg",
};

const sizes: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-base",
  md: "h-9 px-4 text-base",
  tap: "min-h-tap min-w-tap px-4 text-base",
  charge: "min-h-charge px-6 text-lg",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function Button({ variant = "secondary", size = "tap", type = "button", className, ...rest }: ButtonProps) {
  return (
    <button
      // "button", not the HTML default "submit": a stray Enter in a form must
      // never charge a card.
      type={type}
      className={cx(
        "inline-flex select-none items-center justify-center gap-2 rounded font-semibold",
        "outline-offset-2 outline-line-focus focus-visible:outline-2",
        "disabled:pointer-events-none disabled:opacity-40",
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    />
  );
}
