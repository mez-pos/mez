// "Wastage this week" beside the stock table (D19): what was written off in
// the last seven trading days, by ingredient, at the cost it had then
// (ADR-186), with the total set against the seven days before. It reads a
// report, so it is for whoever holds reports.view.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { MinusIcon, TrendingDownIcon, TrendingUpIcon } from "lucide-react";
import { Link } from "wouter";
import type { WasteReport } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { formatQty } from "../../../shared/stock-qty";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useVenueClock } from "../../../shared/use-venue";
import { lastDays } from "../../../shared/venue-clock";
import { change, changeText } from "../dashboard/dashboard-logic";
import { Nothing, Panel, ReadState } from "../dashboard/Panel";
import { entriesText, wasteLeftOut, wastePath } from "../sales/items-logic";
import { previousRange } from "../sales/overview-logic";
import { wastageKey } from "./use-stock";

/** How many ingredients the card lists; the report tab has the rest. */
const rows = 5;

const trends = {
  up: { icon: TrendingUpIcon, ink: "text-danger" },
  down: { icon: TrendingDownIcon, ink: "text-success" },
  same: { icon: MinusIcon, ink: "text-fg-muted" },
} as const;

export function WastageCard() {
  const store = useAuthStore();
  // The hour, so the week moves on a screen left open past the day's start.
  const now = useClock(60 * 60_000);
  const week = lastDays(new Date(now), 7, useVenueClock());
  const before = previousRange(week);
  const read = (range: { from: string; to: string } | null) => ({
    queryKey: [...wastageKey, range],
    queryFn: ({ signal }: { signal: AbortSignal }) => store.api.get<WasteReport>(wastePath(range ?? week), { signal }),
    enabled: range !== null,
    // The week before stays on the card while the next is read, when the day turns.
    placeholderData: keepPreviousData,
  });
  const report = useQuery(read(week));
  const prior = useQuery(read(before));

  return (
    <Panel title="Wastage this week" more={{ label: "View report", href: "/sales/items" }}>
      {report.data ? (
        <Wastage report={report.data} prior={prior.data ?? null} />
      ) : (
        <ReadState what="what was written off" failed={report.isError} onRetry={() => void report.refetch()} />
      )}
      <p aria-hidden="true" className="pt-5 text-center font-script text-xl leading-tight text-fg-2">
        Good Teams,
        <br />
        Great Service ♡
      </p>
    </Panel>
  );
}

function Wastage({ report, prior }: { report: WasteReport; prior: WasteReport | null }) {
  if (report.entries === 0) return <Nothing>Nothing was written off in the last seven days.</Nothing>;
  const shown = report.items.slice(0, rows);
  const more = report.items.length - shown.length;
  // Less wasted than the week before is the good direction, so it wears the green.
  const moved = prior ? change(report.value.minor, prior.value.minor) : null;
  const trend = moved ? trends[moved.direction] : null;
  const leftOut = wasteLeftOut(report);
  return (
    <>
      <ul>
        {shown.map((it) => (
          <li key={it.stock_item_id} className="flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-base font-semibold">{it.name}</span>
              <span className="block text-sm text-fg-muted">{entriesText(it.entries)}</span>
            </span>
            <span className="shrink-0 font-mono text-sm tabular-nums text-fg-2">{formatQty(it.qty, it.unit)}</span>
            {it.uncosted >= it.entries ? (
              <span className="w-24 shrink-0 text-right text-sm text-fg-muted">Not costed</span>
            ) : (
              <Money amount={it.value} tone="danger" className="w-24 shrink-0 font-bold" />
            )}
          </li>
        ))}
      </ul>
      {more > 0 && (
        <p className="pt-2.5 text-sm text-fg-muted">
          <span className="font-mono tabular-nums">{more}</span> more {more === 1 ? "ingredient" : "ingredients"} written off
        </p>
      )}
      <div className="mt-4 flex items-start gap-3 rounded bg-danger-tint p-3.5 text-sm leading-relaxed">
        {trend && <trend.icon aria-hidden="true" size={20} className={cx("mt-0.5 shrink-0", trend.ink)} />}
        <p>
          Total <Money amount={report.value} className="font-bold" /> over {entriesText(report.entries)}.
          {moved && <> {changeText(moved, "the 7 days before")}.</>}
          {leftOut && <> {leftOut}</>}
        </p>
      </div>
      <p className="pt-3 text-sm text-fg-muted">
        To write stock off, use <span className="font-semibold">Log wastage</span>. The full report is on{" "}
        <Link href="/sales/items" className="font-semibold text-attention-text underline">
          Items and inventory
        </Link>
        .
      </p>
    </>
  );
}
