// Sales through the day (D12, "Sales overview"): a bar for each hour of the
// trading day against a round axis. The design stacks each bar by dine-in,
// takeaway and delivery; the report's hours are not cut that way, so a bar is
// one green and the legend is not drawn.

import type { SalesHour } from "../../../shared/api-types";
import { formatMoney, formatMoneyCompact } from "../../../shared/money";
import { useVenueClock } from "../../../shared/use-venue";
import { dayStartHour } from "../../../shared/venue-clock";
import { axisTicks, hourChart, hourLabel, type HourBar } from "./dashboard-logic";

interface SalesOverviewProps {
  hours: readonly SalesHour[];
  currency: string;
}

export function SalesOverview({ hours, currency }: SalesOverviewProps) {
  const chart = hourChart(hours, currency, dayStartHour(useVenueClock()));
  const sold = chart.bars.some((b) => b.bills > 0);
  // Every hour has a bar; every other one is named, so the names do not run together.
  const named = (i: number) => chart.bars.length <= 8 || i % 2 === 0;

  return (
    <div>
      <div className="flex gap-4">
        <div aria-hidden="true" className="flex h-56 flex-col justify-between text-right font-mono text-meta tracking-normal tabular-nums text-fg-muted">
          {axisTicks(chart.top).map((minor) => (
            <span key={minor}>{formatMoneyCompact({ minor, currency })}</span>
          ))}
        </div>
        <div className="min-w-0 flex-1">
          <ul aria-label="Sales by the hour" className="flex h-56 items-end gap-1.5 border-b border-line xl:gap-2.5">
            {chart.bars.map((b) => (
              <li key={b.hour} title={barText(b)} className="flex h-full min-w-0 flex-1 flex-col justify-end">
                <span className="sr-only">{barText(b)}</span>
                <span aria-hidden="true" className="block rounded-t-tight bg-accent" style={{ height: `${b.percent}%` }} />
              </li>
            ))}
          </ul>
          <div aria-hidden="true" className="mt-2 flex gap-1.5 font-mono text-meta tracking-normal tabular-nums text-fg-muted xl:gap-2.5">
            {chart.bars.map((b, i) => (
              <span key={b.hour} className="min-w-0 flex-1 whitespace-nowrap text-center">
                {named(i) ? hourLabel(b.hour) : ""}
              </span>
            ))}
          </div>
        </div>
      </div>
      {!sold && <p className="mt-3 text-sm text-fg-2">No bill has been paid yet today.</p>}
    </div>
  );
}

/** An hour's bar in words: "1 PM, 3 bills, ₹3,400.00". */
function barText(b: HourBar): string {
  if (!b.total) return `${hourLabel(b.hour)}, no bills`;
  return `${hourLabel(b.hour)}, ${b.bills} ${b.bills === 1 ? "bill" : "bills"}, ${formatMoney(b.total)}`;
}
