// A keypad for amounts and PINs. Phone layout, 1 at the top left, which is
// what card terminals use. Keys are 56px: over the 48px minimum, because a
// PIN is typed with one thumb while the other hand holds something, and the
// whole pad is within a thumb's reach (docs/phases/phase-05.md, Task 5).
//
// It reports keys and holds no value: numpad-keys.ts turns keys into an
// amount or a PIN, so the screen owns what is typed and can show it however
// it must (a PIN as dots, an amount as <Money>).

import type { KeyboardEvent, ReactNode } from "react";
import { cx } from "./cx";
import { focusButton } from "./focus";
import { BackspaceIcon } from "./icons";
import { keyFromKeyboard, type NumKey } from "./numpad-keys";

export interface NumPadProps {
  /** `amount` has a 00 key; `pin` has clear there instead. */
  layout: "amount" | "pin";
  onKey: (key: NumKey) => void;
  /** Names the pad for screen readers: "PIN", "Cash tendered". */
  label: string;
  /**
   * `lg` is 64px keys, for a pad that is the whole point of its screen.
   * `signin` is the sign-in screen's (D01): keys that follow the screen's
   * height from 48px to 76px, the digits in the total's size.
   */
  size?: "md" | "lg" | "signin";
  /**
   * The twelfth key, where a pad with none has delete: the sign-in screen's
   * Login. With it the bottom row is delete, 0, and this.
   */
  action?: ReactNode;
  disabled?: boolean;
  className?: string;
}

const rows: NumKey[][] = [
  ["1", "2", "3"],
  ["4", "5", "6"],
  ["7", "8", "9"],
];

function face(key: NumKey, size: NonNullable<NumPadProps["size"]>): { content: ReactNode; name: string } {
  switch (key) {
    case "back": {
      const side = size === "signin" ? "26" : "24";
      return { content: <BackspaceIcon width={side} height={side} />, name: "Delete last digit" };
    }
    case "clear":
      return { content: "Clear", name: "Clear" };
    default:
      return { content: key, name: key };
  }
}

/** A key's height and shape at each size. */
const keySize = {
  md: "h-14 rounded",
  lg: "h-16 rounded",
  // Taller on a tall screen, never under the 48px a thumb needs (ADR-014).
  signin: "h-[clamp(3rem,7vh,4.75rem)] rounded-card hover:bg-raised-high disabled:hover:bg-sunken",
} as const;

export function NumPad({ layout, onKey, label, size = "md", action, disabled = false, className }: NumPadProps) {
  const bottom: NumKey[] = action ? ["back", "0"] : layout === "amount" ? ["00", "0", "back"] : ["clear", "0", "back"];

  // A keyboard works too: the back office has one, and so do most tills.
  const keyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const key = keyFromKeyboard(event.key);
    if (!key || disabled) return;
    event.preventDefault();
    onKey(key);
  };

  return (
    <div
      role="group"
      aria-label={label}
      tabIndex={-1}
      onKeyDown={keyDown}
      // The sign-in pad's gaps close to the 8px two targets need on a short screen, and no further.
      className={cx("grid grid-cols-3", size === "signin" ? "gap-3.5 short:gap-2" : "gap-tap-gap", className)}
    >
      {[...rows, bottom].flat().map((key) => {
        const { content, name } = face(key, size);
        return (
          <button
            key={key}
            type="button"
            aria-label={name}
            disabled={disabled}
            onClick={() => onKey(key)}
            className={cx(
              // A well-coloured key with no border; pressed, it darkens one step.
              "flex min-w-tap select-none items-center justify-center bg-sunken text-fg",
              focusButton,
              "active:bg-line-strong disabled:opacity-40",
              keySize[size],
              key === "clear"
                ? "text-base font-semibold"
                : key === "back"
                  ? "text-fg-muted"
                  : cx("font-mono font-semibold tabular-nums", size === "signin" ? "text-total" : "text-xl"),
            )}
          >
            {content}
          </button>
        );
      })}
      {action}
    </div>
  );
}
