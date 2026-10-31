// Beside the columns from 1536px wide: Coming next, and today's stats under it
// in one line. The figures are the server's (ADR-178); kitchen-logic.ts only
// words them.
//
// D10 gives the stats a card of their own. A cook at the pass reads the
// tickets, not the day's average, so the stats fold into one line here and
// leave the board below 1536px, where they cost a ticket's height (ADR-381,
// over ADR-178's line under the all-day strip).
//
// Coming next is where the design draws "Upcoming (next 5)" (ADR-181). The
// design lists orders about to arrive, each with minutes to go; nothing knows
// when an order will be sent. What is known is what a station will be sent
// when a later course is fired: the mains behind the starters that have gone.
// So the list is that, with the course where the design has minutes. It shows
// only when it has rows (ADR-381); under 1536px it is a strip (BoardHead.tsx).

import { TimerIcon } from "lucide-react";
import type { ComingRow, StatRow } from "./kitchen-logic";

interface BoardSideProps {
  rows: readonly StatRow[];
  coming: readonly ComingRow[];
  /** How many more courses wait behind the ones listed. */
  comingMore: number;
}

/** The column, from 1536px wide: there the three columns keep their width beside it. */
export function BoardSide({ rows, coming, comingMore }: BoardSideProps) {
  return (
    <aside aria-label="Coming next and today's stats" className="hidden min-h-0 w-aside shrink-0 flex-col gap-3.5 2xl:flex">
      {coming.length > 0 && (
        <section aria-label="Coming next" className="flex min-h-0 flex-col rounded-card border border-line bg-chrome p-5 short:p-4">
          <h2 className="flex items-center gap-2.5 font-serif text-xl">
            <TimerIcon aria-hidden="true" size={20} className="shrink-0 text-attention" />
            Coming next ({coming.length + comingMore})
          </h2>
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
          {comingMore > 0 && <p className="mt-2.5 text-base text-fg-muted">And {comingMore} more behind these.</p>}
        </section>
      )}
      <section
        aria-label="Today's stats"
        className="flex shrink-0 flex-wrap items-baseline gap-x-5 gap-y-1 rounded-card border border-line bg-chrome px-5 py-4 text-lg"
      >
        <h2 className="text-fg-muted">Today</h2>
        <dl className="contents">
          {rows.map((row) => (
            <div key={row.label} className="flex gap-2">
              <dt className="text-fg-2">{row.label}</dt>
              <dd className="font-bold tabular-nums">{row.value}</dd>
            </div>
          ))}
        </dl>
      </section>
    </aside>
  );
}
