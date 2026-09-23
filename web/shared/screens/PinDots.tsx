// The PIN slots shared by the lock screen and a manager's approval.

import { cx } from "../ui/cx";

export const minPin = 4; // ADR-056: 4 to 8 digits

// PINs are 4 to 8 digits (ADR-056), so the design's four fixed boxes cannot
// hold one. Four slots to start, and one more for each digit past four.
export function PinDots({ length }: { length: number }) {
  const slots = Math.max(minPin, length);
  return (
    <div aria-label={`${length} of ${slots} digits entered`} className="mt-4 flex h-12 items-center gap-2">
      {Array.from({ length: slots }, (_, i) => (
        <span
          key={i}
          className={cx("flex h-12 flex-1 items-center justify-center rounded-tight border", i < length ? "border-line-strong bg-canvas" : "border-line bg-canvas")}
        >
          {i < length && <span className="size-2.5 rounded-round bg-accent" />}
        </span>
      ))}
    </div>
  );
}
