// A switch (TECH-23, ADR-406): the word, a pill whose knob slides, and On or
// Off said as well as shown. The whole thing is the target, 48px tall. One
// for every app: the kitchen's board settings and a role's permissions were
// two copies.

import type { ReactNode } from "react";
import { cx } from "./cx";
import { focusButton } from "./focus";

export interface SwitchProps {
  label: ReactNode;
  on: boolean;
  onChange: (on: boolean) => void;
  disabled?: boolean;
  /** `row` fills its line, the word at the left and the switch at the right: a list of settings. `inline` sits among other controls. */
  layout?: "row" | "inline";
  className?: string;
}

export function Switch({ label, on, onChange, disabled = false, layout = "inline", className }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      disabled={disabled}
      onClick={() => onChange(!on)}
      className={cx(
        "min-h-tap items-center gap-3 rounded text-left text-base",
        focusButton,
        layout === "row" ? "flex w-full" : "inline-flex",
        disabled && "text-fg-muted",
        className,
      )}
    >
      <span className={cx("min-w-0", layout === "row" && "flex-1")}>{label}</span>
      <span
        aria-hidden="true"
        className={cx(
          "flex h-7.5 w-13 shrink-0 items-center rounded-pill p-0.75",
          on ? "justify-end bg-accent" : "justify-start bg-line-strong",
          disabled && "opacity-50",
        )}
      >
        <span className="size-6 rounded-round bg-accent-fg" />
      </span>
      {/* The word is for the eye; aria-checked says it to a screen reader. */}
      <span aria-hidden="true" className="w-8 text-left text-base font-semibold text-fg-muted">
        {on ? "On" : "Off"}
      </span>
    </button>
  );
}
