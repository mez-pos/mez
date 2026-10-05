// Today's stats (D10): the station's trading day so far, in a column beside
// the board on a wide display and as one line on a narrower one. The figures
// are the server's (ADR-178); kitchen-logic.ts only words them.
//
// Under it, where the design draws "Upcoming (next 5)", is Coming next
// (ADR-181). The design lists orders about to arrive, each with minutes to
// go; nothing knows when an order will be sent. What is known is what a
// station will be sent when a later course is fired: the mains behind the
// starters that have gone. So the list is that, with the course where the
// design has minutes.

import { ChartColumnIcon, TimerIcon } from "lucide-react";
import type { ComingRow, StatRow } from "./kitchen-logic";

interface StatsProps {
  rows: readonly StatRow[];
  coming: readonly ComingRow[];
  /** How many more courses wait behind the ones listed. */
  comingMore: number;
}

/** The column, from 1536px wide: there the three columns keep their width beside it. */
export function BoardStats({ rows, coming, comingMore }: StatsProps) {
  const firstTiming = rows.findIndex((r) => r.timing);
  return (
    <aside aria-label="Today's stats" className="hidden min-h-0 flex-col gap-3.5 2xl:flex">
      <section className="rounded-card border border-line bg-chrome p-5">
        <h2 className="flex items-center gap-2.5 font-serif text-xl">
          <ChartColumnIcon aria-hidden="true" size={20} strokeWidth={1.9} className="shrink-0 text-attention" />
          Today&apos;s stats
        </h2>
        <dl className="mt-4 grid gap-3 text-lg">
          {rows.map((row, i) => (
            <div key={row.label} className={i === firstTiming ? "flex justify-between gap-3 border-t border-line pt-3" : "flex justify-between gap-3"}>
              <dt className="shrink-0 text-fg-2">{row.label}</dt>
              <dd className="min-w-0 text-right font-bold tabular-nums">{row.value}</dd>
            </div>
          ))}
        </dl>
      </section>
      <section aria-label="Coming next" className="flex min-h-0 flex-1 flex-col rounded-card border border-line bg-chrome p-5">
        <h2 className="flex items-center gap-2.5 font-serif text-xl">
          <TimerIcon aria-hidden="true" size={20} strokeWidth={1.9} className="shrink-0 text-attention" />
          Coming next{coming.length > 0 && ` (${coming.length + comingMore})`}
        </h2>
        {coming.length === 0 ? (
          <p className="mt-3 text-lg text-fg-muted">Nothing is waiting for a later course.</p>
        ) : (
          <ul className="mt-3.5 grid min-h-0 content-start gap-2.5 overflow-y-auto">
            {coming.map((c) => (
              <li key={c.id} className="rounded border border-line p-3">
                <div className="flex items-center gap-2.5">
                  <span className="min-w-0 flex-1 truncate text-lg font-bold">{c.where}</span>
                  <span className="shrink-0 rounded-pill bg-chip-neutral-bg px-3 py-1 text-sm font-bold text-chip-neutral-fg">{c.course}</span>
                </div>
                <p className="mt-1 text-base text-fg-2">{c.items}</p>
              </li>
            ))}
          </ul>
        )}
        {comingMore > 0 && <p className="mt-2.5 text-base text-fg-muted">And {comingMore} more behind these.</p>}
        <p aria-hidden="true" className="mt-auto pt-4 text-center font-script text-xl leading-tight text-fg-2">
          Great Food,
          <br />
          Happier People ♡
        </p>
      </section>
    </aside>
  );
}

/** The same figures on one line, under 1536px wide. */
export function StatsLine({ rows, coming, comingMore }: StatsProps) {
  return (
    <dl aria-label="Today's stats" className="flex shrink-0 flex-wrap gap-x-6 gap-y-1 border-b border-line bg-chrome px-7 py-2 text-lg 2xl:hidden">
      <span className="text-fg-muted">Today</span>
      {rows.map((row) => (
        <div key={row.label} className="flex gap-2">
          <dt className="text-fg-2">{row.label}</dt>
          <dd className="font-bold tabular-nums">{row.value}</dd>
        </div>
      ))}
      <div className="flex gap-2">
        <dt className="text-fg-2">Coming next</dt>
        <dd className="font-bold tabular-nums">{coming.length + comingMore}</dd>
      </div>
    </dl>
  );
}
