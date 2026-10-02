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
import { BackspaceIcon } from "./icons";
import { keyFromKeyboard, type NumKey } from "./numpad-keys";

export interface NumPadProps {
  /** `amount` has a 00 key; `pin` has clear there instead. */
  layout: "amount" | "pin";
  onKey: (key: NumKey) => void;
  /** Names the pad for screen readers: "PIN", "Cash tendered". */
  label: string;
  /** `lg` is 64px keys, for a pad that is the whole point of its screen. */
  size?: "md" | "lg";
  disabled?: boolean;
  className?: string;
}

const rows: NumKey[][] = [
  ["1", "2", "3"],
  ["4", "5", "6"],
  ["7", "8", "9"],
];

function face(key: NumKey): { content: ReactNode; name: string } {
  switch (key) {
    case "back":
      return { content: <BackspaceIcon width="24" height="24" />, name: "Delete last digit" };
    case "clear":
      return { content: "Clear", name: "Clear" };
    default:
      return { content: key, name: key };
  }
}

export function NumPad({ layout, onKey, label, size = "md", disabled = false, className }: NumPadProps) {
  const bottom: NumKey[] = layout === "amount" ? ["00", "0", "back"] : ["clear", "0", "back"];

  // A keyboard works too: the back office has one, and so do most tills.
  const keyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const key = keyFromKeyboard(event.key);
    if (!key || disabled) return;
    event.preventDefault();
    onKey(key);
  };

  return (
    <div role="group" aria-label={label} tabIndex={-1} onKeyDown={keyDown} className={cx("grid grid-cols-3 gap-tap-gap", className)}>
      {[...rows, bottom].flat().map((key) => {
        const { content, name } = face(key);
        return (
          <button
            key={key}
            type="button"
            aria-label={name}
            disabled={disabled}
            onClick={() => onKey(key)}
            className={cx(
              // A well-coloured key with no border; pressed, it darkens one step.
              "flex min-w-tap select-none items-center justify-center rounded bg-sunken text-fg",
              "outline-offset-2 outline-line-focus focus-visible:outline-2 active:bg-line-strong disabled:opacity-40",
              size === "lg" ? "h-16" : "h-14",
              key === "clear" ? "text-base font-semibold" : "font-mono text-xl font-semibold tabular-nums",
            )}
          >
            {content}
          </button>
        );
      })}
    </div>
  );
}
