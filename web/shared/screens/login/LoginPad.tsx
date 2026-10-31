// The PIN and the keypad of the sign-in screen (D01): dots in a well, the
// kit's keypad at its sign-in size, and the green Login as the twelfth key
// where a Clear would be (TECH-23, ADR-406: one keypad). Keys report
// themselves; the screen owns what is typed and what Login does. Keys are
// 76px on a tall screen and shrink with its height, never below the 48px a
// thumb needs (ADR-014).
//
// The 900px trims stop at 821px: an arbitrary media variant is emitted after
// `short:`, and would otherwise win over it on a 768 screen.

import { ArrowRightIcon } from "lucide-react";
import { cx } from "../../ui/cx";
import { focusButton } from "../../ui/focus";
import { NumPad } from "../../ui/NumPad";
import type { NumKey } from "../../ui/numpad-keys";
import { stroke } from "../../ui/stroke";
import { minPin } from "../PinDots";

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
      <NumPad
        layout="pin"
        size="signin"
        label="PIN"
        disabled={disabled}
        onKey={onKey}
        className="mt-5 [@media(max-height:900px)]:mt-3"
        action={
          <button
            type="button"
            disabled={!canSubmit}
            onClick={onSubmit}
            className={cx(
              "flex h-[clamp(3rem,7vh,4.75rem)] items-center justify-center gap-2.5 rounded-card bg-accent text-lg font-semibold text-accent-fg shadow-action",
              focusButton,
              "transition-colors hover:bg-accent-hover disabled:opacity-50 disabled:shadow-none disabled:hover:bg-accent",
            )}
          >
            {busy ? "Signing in" : "Login"}
            {!busy && <ArrowRightIcon aria-hidden="true" size={20} strokeWidth={stroke.bold} />}
          </button>
        }
      />
    </div>
  );
}
