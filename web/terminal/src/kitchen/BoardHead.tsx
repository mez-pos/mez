// The top of the kitchen's board: the station's name as its title, the clock
// and the connection; then, each when it has something to say, the all-day
// counts and what is coming next, one line each.
//
// The station is the title because it is what a cook checks from across the
// pass: which board this is. D10's "Kitchen Display", its tagline and its
// script lines are gone (ADR-381): decoration is for screens that wait.
//
// A strip holds to one line and ends "+3 more" when its items do not fit, so
// a busy hour never pushes the tickets down a 768 screen (ADR-381).

import { ChefHatIcon } from "lucide-react";
import { useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { ConnectionIndicator } from "../../../shared/ConnectionIndicator";
import { Logo } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { clockText } from "../../../shared/venue-text";
import type { AllDay, ComingRow } from "./kitchen-logic";

interface BoardHeaderProps {
  stationName: string;
  now: number;
}

export function BoardHeader({ stationName, now }: BoardHeaderProps) {
  return (
    <header className="flex min-h-23 shrink-0 items-center gap-6 border-b border-line bg-chrome px-7 py-2 short:min-h-14 short:gap-4 short:px-5 short:py-1">
      <Logo className="h-9 shrink-0 short:h-8" />
      <h1 className="flex min-w-0 flex-1 items-center justify-center gap-3 font-serif text-total leading-tight 2xl:text-display">
        <ChefHatIcon aria-hidden="true" size={30} className="shrink-0 text-attention" />
        <span className="truncate">{stationName}</span>
      </h1>
      <div className="flex shrink-0 items-center gap-4">
        <span className="hidden font-mono text-base font-semibold tabular-nums xl:block">{clockText(new Date(now), null)}</span>
        <ConnectionIndicator />
      </div>
    </header>
  );
}

/** How many of each dish are still to make, most first. */
export function AllDayStrip({ counts }: { counts: readonly AllDay[] }) {
  const items = counts.map((c) => ({
    key: c.name,
    node: (
      <>
        <span className="font-mono font-bold tabular-nums text-attention-text">{c.qty}</span> {c.name}
      </>
    ),
  }));
  return <Strip label="All day" items={items} measure={counts.map((c) => `${c.qty} ${c.name}`).join("\n")} className="text-xl" />;
}

interface ComingStripProps {
  coming: readonly ComingRow[];
  /** How many more courses wait behind the ones listed. */
  comingMore: number;
}

/** Coming next under 1536px wide, where there is no column for it. Only drawn with rows. */
export function ComingStrip({ coming, comingMore }: ComingStripProps) {
  const items = coming.map((c) => ({
    key: c.id,
    node: (
      <span className="flex items-baseline gap-2">
        <span className="font-bold">{c.where}</span>
        <span className="rounded-pill bg-chip-neutral-bg px-2.5 text-sm font-bold text-chip-neutral-fg">{c.course}</span>
        <span className="text-fg-2">{c.items}</span>
      </span>
    ),
  }));
  const measure = coming.map((c) => `${c.id} ${c.course} ${c.items}`).join("\n");
  return <Strip label="Coming next" items={items} more={comingMore} measure={measure} className="text-lg 2xl:hidden" />;
}

interface StripProps {
  label: string;
  items: readonly { key: string; node: ReactNode }[];
  /** Items the server holds back beyond the list: added to the count the line ends with. */
  more?: number;
  /** Changes when the items' words do, so the line is measured again. */
  measure: string;
  /** The strip's type size, and where it shows. */
  className: string;
}

function Strip({ label, items, more = 0, measure, className }: StripProps) {
  const list = useRef<HTMLUListElement>(null);
  const cut = useOverflowCount(list, measure);
  const count = cut + more;
  return (
    <section
      aria-label={label}
      className={cx("flex shrink-0 items-baseline gap-x-6 border-b border-line bg-chrome px-7 py-2 short:gap-x-4 short:px-5 short:py-1", className)}
    >
      <h2 className="shrink-0 text-fg-muted">{label}</h2>
      {/* One row that never wraps: an item past its end is hidden whole and counted, not cut in half. */}
      <ul ref={list} className="relative flex min-w-0 flex-1 gap-x-6 overflow-hidden short:gap-x-4">
        {items.map((item, i) => (
          <li key={item.key} className={cx("shrink-0 whitespace-nowrap", i > 0 && i >= items.length - cut && "invisible")}>
            {item.node}
          </li>
        ))}
      </ul>
      {count > 0 && <span className="shrink-0 font-semibold whitespace-nowrap text-fg-muted">+{count} more</span>}
    </section>
  );
}

/**
 * How many of a row's items, after the first, run past its end. Measured
 * before the paint, when the fonts arrive and when the row's width changes. A
 * hidden item keeps its place, so hiding one moves nothing; a "+N more" that
 * appears narrows the row, which can only hide more, so it settles.
 */
function useOverflowCount(list: RefObject<HTMLElement | null>, measure: string): number {
  const [cut, setCut] = useState(0);
  useLayoutEffect(() => {
    const el = list.current;
    if (!el) return;
    let live = true;
    const count = () => {
      if (!live) return;
      const items = Array.from(el.children).slice(1) as HTMLElement[];
      setCut(items.filter((item) => item.offsetLeft + item.offsetWidth > el.clientWidth + 1).length);
    };
    count();
    void document.fonts?.ready.then(count);
    const watch = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(count);
    watch?.observe(el);
    return () => {
      live = false;
      watch?.disconnect();
    };
  }, [list, measure]);
  return cut;
}
