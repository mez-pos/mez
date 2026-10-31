// The range as one pill (D15): it opens the quick ranges and a first and a
// last day. The reports share one range (report-range.tsx); the Audit screen
// keeps its own, so the pill is given its range and told when it changes.

import { CalendarRangeIcon, ChevronDownIcon } from "lucide-react";
import type { ReactNode } from "react";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { useVenueClock } from "../../../shared/use-venue";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { inputClass } from "../FormFields";
import { rangeText, usable } from "./overview-logic";
import { quickRange, quickRangeOf, quickRanges, type SalesRange } from "./sales-logic";

/** A pill in the head of a screen: the range, and what sits beside it. */
export const headPill =
  "inline-flex min-h-tap items-center gap-2 whitespace-nowrap rounded-pill px-5 text-base font-semibold outline-offset-2 outline-attention focus-visible:outline-3";
/** What a head pill opens. */
export const headPanel = "absolute right-0 top-full z-20 mt-2 rounded-card border border-line bg-raised p-4 shadow-card";

interface RangePillProps {
  range: SalesRange;
  onChange: (range: SalesRange) => void;
  /** What the days mean on this screen, under the choices. */
  note: string;
  /** More choices about the range, between the days and the note. */
  children?: ReactNode;
}

export function RangePill({ range, onChange, note, children }: RangePillProps) {
  const { open, toggle, box, trigger, panel, panelId, leaves } = useDisclosure();
  // Worked out as it is drawn: which quick range these days are depends on today.
  const clock = useVenueClock();
  const quick = quickRangeOf(range, new Date(), clock);

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(headPill, "border border-line-strong bg-raised hover:bg-raised-high")}
      >
        <CalendarRangeIcon aria-hidden="true" size={17} />
        <span className="font-mono tabular-nums">{usable(range) ? rangeText(range) : "Pick the days"}</span>
        <ChevronDownIcon aria-hidden="true" size={16} className={cx("text-fg-muted transition-transform", open && "rotate-180")} />
      </button>
      {open && (
        // tabIndex: a click on the panel itself keeps focus inside it, so it does not close.
        <div
          ref={panel}
          id={panelId}
          tabIndex={-1}
          role="group"
          aria-label="Range"
          className={cx(headPanel, "flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-4")}
        >
          <div role="group" aria-label="Quick ranges" className="flex flex-wrap gap-tap-gap">
            {quickRanges.map((q) => (
              <button
                key={q.id}
                type="button"
                aria-pressed={quick === q.id}
                className={pillClass(quick === q.id)}
                onClick={() => onChange(quickRange(q.id, new Date(), clock))}
              >
                {q.label}
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1.5 text-sm font-semibold">
              From
              <input
                type="date"
                className={cx(inputClass, "font-mono")}
                value={range.from}
                max={range.to}
                onChange={(e) => onChange({ ...range, from: e.target.value })}
              />
            </label>
            <label className="flex flex-col gap-1.5 text-sm font-semibold">
              To
              <input
                type="date"
                className={cx(inputClass, "font-mono")}
                value={range.to}
                min={range.from}
                onChange={(e) => onChange({ ...range, to: e.target.value })}
              />
            </label>
          </div>
          {children}
          <p className="text-sm text-fg-3">{note}</p>
        </div>
      )}
    </div>
  );
}
