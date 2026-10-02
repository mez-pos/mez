// The PIN dots shared by the lock screen and a manager's approval.

import { cx } from "../ui/cx";

export const minPin = 4; // ADR-056: 4 to 8 digits

// PINs are 4 to 8 digits (ADR-056), so the design's four fixed boxes cannot
// hold one. Four dots to start, and one more for each digit past four.
export function PinDots({ length }: { length: number }) {
  const slots = Math.max(minPin, length);
  return (
    <div aria-label={`${length} of ${slots} digits entered`} className="mt-4 flex h-12 items-center justify-center gap-3">
      {Array.from({ length: slots }, (_, i) => (
        <span key={i} className={cx("size-3.5 rounded-round", i < length ? "bg-accent" : "bg-line-strong")} />
      ))}
    </div>
  );
}
