// The button. Six variants, four sizes (docs/design/components.md).
//
// Every label is --text-base, even on `sm`: the 15px floor for anything
// interactive is not relaxed for density (tokens.md, ADR-014). `sm` and `md`
// are for keyboard-and-mouse screens in the back office; anything touched
// during service is `tap` or larger, which is why `tap` is the default.

import type { ButtonHTMLAttributes } from "react";
import { cx } from "./cx";

export type ButtonVariant = "primary" | "attention" | "secondary" | "ghost" | "danger" | "tint";
export type ButtonSize = "sm" | "md" | "tap" | "charge";

const variants: Record<ButtonVariant, string> = {
  // Green commits: settle, send, save, confirm. One per screen.
  primary: "bg-accent text-accent-fg shadow-action hover:bg-accent-hover",
  // Terracotta asks for attention: new order, export, generate.
  attention: "bg-attention text-attention-fg hover:bg-attention-hover",
  secondary: "border border-line-strong bg-raised text-fg hover:bg-raised-high",
  ghost: "text-attention-text hover:bg-attention-tint",
  // Tinted, not filled: a void is serious, and a filled brick button next to
  // a green one reads as two calls to action.
  danger: "border border-danger-line bg-danger-tint text-danger hover:bg-danger-tint-hover",
  // Green words on the green tint: an add repeated down a list (the guest's
  // + Add on every dish), where a fill on each would outweigh the one primary.
  tint: "bg-accent-tint text-accent-text hover:bg-accent hover:text-accent-fg",
};

const sizes: Record<ButtonSize, string> = {
  sm: "h-8 px-4 text-base",
  md: "h-10 px-5 text-base",
  tap: "min-h-tap min-w-tap px-5 text-base",
  charge: "min-h-charge px-7 text-lg",
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
        "inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-pill font-semibold",
        "outline-offset-2 outline-attention focus-visible:outline-3",
        "disabled:pointer-events-none disabled:opacity-40",
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    />
  );
}
