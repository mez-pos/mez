// Today's stats (D10): the station's trading day so far, in a column beside
// the board on a wide display and as one line on a narrower one. The figures
// are the server's (ADR-178); kitchen-logic.ts only words them.
//
// Left out: "Upcoming (next 5)". The design lists orders about to arrive, each
// with minutes to go. Nothing knows when an order will be sent, so there is
// no such list to draw (rule 3). What waits for a later course is a list the
// app could make; it is its own task (rebuild 15f).

import { ChartColumnIcon } from "lucide-react";
import type { StatRow } from "./kitchen-logic";

interface StatsProps {
  rows: readonly StatRow[];
}

/** The column, from 1536px wide: there the three columns keep their width beside it. */
export function BoardStats({ rows }: StatsProps) {
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
      <p aria-hidden="true" className="mt-auto pb-2 text-center font-script text-xl leading-tight text-fg-2">
        Good Food
        <br />
        In Progress ♡
      </p>
    </aside>
  );
}

/** The same figures on one line, under 1536px wide. */
export function StatsLine({ rows }: StatsProps) {
  return (
    <dl aria-label="Today's stats" className="flex shrink-0 flex-wrap gap-x-6 gap-y-1 border-b border-line bg-chrome px-7 py-2 text-lg 2xl:hidden">
      <span className="text-fg-muted">Today</span>
      {rows.map((row) => (
        <div key={row.label} className="flex gap-2">
          <dt className="text-fg-2">{row.label}</dt>
          <dd className="font-bold tabular-nums">{row.value}</dd>
        </div>
      ))}
    </dl>
  );
}
