// The PIN and the keypad of the sign-in screen (D01): dots in a well, a 3 by 4
// grid of keys, and the green Login as the twelfth key where a Clear would be.
// Keys report themselves; the screen owns what is typed and what Login does.
// Keys are 76px on a tall screen and shrink with its height, never below the 48px
// a thumb needs (ADR-014): clamp(48px, 7vh, 76px). On a short screen the gap
// between them closes to the 8px two targets need, and no further.
//
// The 900px trims stop at 821px: an arbitrary media variant is emitted after
// `short:`, and would otherwise win over it on a 768 screen.

import { ArrowRightIcon, DeleteIcon } from "lucide-react";
import { cx } from "../../ui/cx";
import type { NumKey } from "../../ui/numpad-keys";
import { minPin } from "../PinDots";
import { stroke } from "../../ui/stroke";

const digits: NumKey[][] = [
  ["1", "2", "3"],
  ["4", "5", "6"],
  ["7", "8", "9"],
];

interface LoginPadProps {
  /** Whose PIN this is, shown beside the dots: paging away from their tile must not lose it. */
  name: string | null;
  /** How many digits are typed. */
  length: number;
  disabled: boolean;
  /** Login is on offer: someone is chosen and a PIN long enough is typed. */
  canSubmit: boolean;
  busy: boolean;
  onKey: (key: NumKey) => void;
  onSubmit: () => void;
}

const keyShape = cx(
  "flex h-[clamp(3rem,7vh,4.75rem)] items-center justify-center rounded-card bg-sunken outline-offset-2 outline-focus transition-colors",
  "hover:bg-raised-high focus-visible:outline-3 active:bg-line disabled:opacity-50 disabled:hover:bg-sunken",
);

export function LoginPad({ name, length, disabled, canSubmit, busy, onKey, onSubmit }: LoginPadProps) {
  const slots = Math.max(minPin, length);
  return (
    <div className="mt-12 w-full max-w-120 [@media(min-height:821px)_and_(max-height:900px)]:mt-6 short:mt-4">
      <div className="relative flex h-[clamp(3rem,6.7vh,4.5rem)] items-center justify-center gap-5.5 rounded-card bg-sunken">
        <span className="absolute left-5 max-w-[30%] truncate text-sm font-semibold text-fg-2">{name ?? "Choose a profile"}</span>
        <span role="img" aria-label={`${length} of ${slots} digits entered`} className="flex gap-5.5">
          {Array.from({ length: slots }, (_, i) => (
            <span key={i} className={cx("size-4 rounded-round", i < length ? "bg-accent" : "bg-line-strong")} />
          ))}
        </span>
      </div>
      <div role="group" aria-label="PIN" className="mt-5 grid grid-cols-3 gap-3.5 [@media(max-height:900px)]:mt-3 short:gap-2">
        {digits.flat().map((key) => (
          <button
            key={key}
            type="button"
            disabled={disabled}
            onClick={() => onKey(key)}
            className={cx(keyShape, "font-mono text-total font-semibold tabular-nums")}
          >
            {key}
          </button>
        ))}
        <button type="button" disabled={disabled} onClick={() => onKey("back")} aria-label="Delete last digit" className={cx(keyShape, "text-fg-muted")}>
          <DeleteIcon aria-hidden="true" size={26} />
        </button>
        <button type="button" disabled={disabled} onClick={() => onKey("0")} className={cx(keyShape, "font-mono text-total font-semibold tabular-nums")}>
          0
        </button>
        <button
          type="button"
          disabled={!canSubmit}
          onClick={onSubmit}
          className={cx(
            "flex h-[clamp(3rem,7vh,4.75rem)] items-center justify-center gap-2.5 rounded-card bg-accent text-lg font-semibold text-accent-fg shadow-action",
            "outline-offset-2 outline-focus transition-colors hover:bg-accent-hover focus-visible:outline-3",
            "disabled:opacity-50 disabled:shadow-none disabled:hover:bg-accent",
          )}
        >
          {busy ? "Signing in" : "Login"}
          {!busy && <ArrowRightIcon aria-hidden="true" size={20} strokeWidth={stroke.bold} />}
        </button>
      </div>
    </div>
  );
}
