// The pictures at the top of the Sales reports screen (D15): the trend by how
// it was served, sales by category and orders by type as rings, the top
// selling items and how it was paid. Each is a picture of figures the tables
// further down give in full, and says them in words for a screen reader.

import type { CSSProperties } from "react";
import type { SalesReport } from "../../../shared/api-types";
import { formatMoney, formatMoneyCompact } from "../../../shared/money";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { methodName } from "../../../shared/zreport/zreport-logic";
import { axisTicks, topItems } from "../dashboard/dashboard-logic";
import { Nothing } from "../dashboard/Panel";
import { byBills, byTotal, namedBars, ringGradient, serviceName, services, slices, trend, type Slice, type TrendBar } from "./overview-logic";
import { periodText, shares } from "./sales-logic";

// The chart colours are the sheet's own, in the design's order: green,
// terracotta, mustard, then the quieter inks for what is left (tokens.md).
const serviceFills = { dine_in: "bg-accent", takeaway: "bg-attention", delivery: "bg-highlight" } as const;
const serviceColours = { dine_in: "var(--accent)", takeaway: "var(--attention)", delivery: "var(--highlight)" } as const;
const ringColours = ["var(--accent)", "var(--attention)", "var(--highlight)", "var(--chip-warning-fg)", "var(--fg-faint)", "var(--line-strong)"];

/** The dot beside a legend's word, in a colour that is data. */
function Dot({ colour, className }: { colour: string; className?: string }) {
  return <span aria-hidden="true" className={cx("size-2.5 shrink-0 rounded-round", className)} style={{ background: colour }} />;
}

/** Which colour is which, beside the trend's title. */
export function ServiceLegend() {
  return (
    <ul className="ml-auto flex flex-wrap gap-x-4 gap-y-1 text-sm text-fg-2">
      {services.map((s) => (
        <li key={s.id} className="flex items-center gap-1.5">
          <Dot colour={serviceColours[s.id]} />
          {s.name}
        </li>
      ))}
    </ul>
  );
}

/** A period's bar in words: "1 Oct, ₹3,400.00: Dine-in ₹2,000.00, Takeaway ₹1,400.00". */
function barText(b: TrendBar): string {
  const split = b.parts.flatMap((p) => (p.total ? [`${serviceName(p.id)} ${formatMoney(p.total)}`] : []));
  return split.length > 0 ? `${periodText(b)}, ${formatMoney(b.total)}: ${split.join(", ")}` : `${periodText(b)}, no sales`;
}

/** A whole bar's height against the axis: its parts, one on another. */
const barHeight = (b: TrendBar): number =>
  Math.min(
    100,
    b.parts.reduce((sum, p) => sum + p.percent, 0),
  );

export function SalesTrend({ report }: { report: SalesReport }) {
  const chart = trend(report.series, report.totals.currency);
  const named = namedBars(chart.bars.length);
  const currency = report.totals.currency;
  if (report.totals.sales.bills === 0) return <Nothing>No sales in these days.</Nothing>;

  return (
    <div className="flex gap-4">
      <div aria-hidden="true" className="flex h-52 flex-col justify-between text-right font-mono text-meta tracking-normal tabular-nums text-fg-muted">
        {axisTicks(chart.top).map((minor) => (
          <span key={minor}>{formatMoneyCompact({ minor, currency })}</span>
        ))}
      </div>
      <div className="min-w-0 flex-1">
        <ul aria-label="Sales by period" className="flex h-52 items-end justify-around gap-px border-b border-line sm:gap-1 xl:gap-1.5">
          {chart.bars.map((b) => (
            <li key={b.start} title={barText(b)} className="flex h-full min-w-0 max-w-16 flex-1 flex-col justify-end">
              <span className="sr-only">{barText(b)}</span>
              {/* Dine-in sits on the axis, then takeaway, then delivery on top, as drawn. */}
              <span aria-hidden="true" className="flex flex-col-reverse overflow-hidden rounded-t-tight" style={{ height: `${barHeight(b)}%` }}>
                {b.parts.map((p) => (
                  <span key={p.id} className={cx("block min-h-0", serviceFills[p.id])} style={{ flexGrow: p.percent }} />
                ))}
              </span>
            </li>
          ))}
        </ul>
        <div aria-hidden="true" className="mt-2 flex justify-around gap-px font-mono text-meta tracking-normal tabular-nums text-fg-muted sm:gap-1 xl:gap-1.5">
          {chart.bars.map((b, i) => (
            // A name is wider than its bar, so it is let out over the bars beside it.
            <span key={b.start} className="flex min-w-0 max-w-16 flex-1 justify-center overflow-visible whitespace-nowrap">
              {named[i] ? periodText({ start: b.start, end: b.start }) : ""}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

interface RingProps {
  parts: readonly Slice[];
  /** One colour a slice. */
  colours: readonly string[];
  /** The figure in the middle and the word under it. */
  centre: React.ReactNode;
  label: string;
  /** What each slice says after its name: its share, and its count if it has one. */
  detail: (s: Slice) => string;
}

/** A ring and its legend. The ring is decoration: the legend says every slice. */
function Ring({ parts, colours, centre, label, detail }: RingProps) {
  const ring: CSSProperties = { background: ringGradient(parts, colours) };
  return (
    <div className="flex flex-wrap items-center gap-6">
      <div aria-hidden="true" className="relative size-40 shrink-0 rounded-round" style={ring}>
        <div className="absolute inset-8 flex flex-col items-center justify-center rounded-round bg-raised text-center">
          <div className="font-serif text-lg tabular-nums leading-tight">{centre}</div>
          <div className="text-meta tracking-normal text-fg-muted">{label}</div>
        </div>
      </div>
      <ul className="grid min-w-36 flex-1 gap-2.5 text-base">
        {parts.map((s, i) => (
          <li key={s.key} className="flex items-center gap-2.5">
            <Dot colour={colours[Math.min(i, colours.length - 1)] ?? "transparent"} />
            <span className="min-w-0 truncate">{s.name}</span>
            <strong className="ml-auto whitespace-nowrap font-mono tabular-nums">{detail(s)}</strong>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function CategoryRing({ report }: { report: SalesReport }) {
  const parts = slices(byTotal(report.totals.by_category), ringColours.length);
  if (parts.length === 0) return <Nothing>Nothing sold in these days.</Nothing>;
  return (
    <Ring parts={parts} colours={ringColours} centre={formatMoneyCompact(report.totals.sales.total)} label="Total sales" detail={(s) => `${s.percent}%`} />
  );
}

export function OrderTypes({ report }: { report: SalesReport }) {
  // In the services' own order, so a colour means the same here as on the trend.
  const rows = byBills(report.totals.by_service, (g) => serviceName(g.key));
  const parts = slices(rows, services.length + 1).sort((a, b) => order(a.key) - order(b.key));
  if (parts.length === 0) return <Nothing>No bills in these days.</Nothing>;
  const colours = parts.map((s) => serviceColours[s.key as keyof typeof serviceColours] ?? "var(--fg-faint)");
  return <Ring parts={parts} colours={colours} centre={report.totals.sales.bills} label="Orders" detail={(s) => `${s.percent}% (${s.value})`} />;
}

const order = (key: string): number => {
  const at = services.findIndex((s) => s.id === key);
  return at === -1 ? services.length : at;
};

const head = "pb-2.5 text-meta font-bold uppercase text-fg-muted";

/** How many of the top sellers the card lists; the table further down has them all. */
const topRows = 5;

export function TopSelling({ report }: { report: SalesReport }) {
  const top = topItems(report.totals.by_item, topRows);
  if (top.length === 0) return <Nothing>Nothing sold in these days.</Nothing>;
  return (
    <table className="w-full text-base">
      <thead>
        <tr className="border-b border-line text-left">
          <th scope="col" className={cx(head, "w-8")}>
            #
          </th>
          <th scope="col" className={head}>
            Item
          </th>
          <th scope="col" className={cx(head, "text-right")}>
            Qty
          </th>
          <th scope="col" className={cx(head, "text-right")}>
            Sales
          </th>
        </tr>
      </thead>
      <tbody>
        {top.map((g, i) => (
          <tr key={g.key} className="border-b border-line-soft last:border-b-0">
            <td className="py-2.5 font-mono tabular-nums text-fg-muted">{i + 1}</td>
            <td className="py-2.5">
              <strong>{g.name}</strong>
            </td>
            <td className="py-2.5 text-right font-mono tabular-nums">{g.qty}</td>
            <td className="py-2.5 text-right">
              <Money amount={g.total} className="font-bold" />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

const methodFills = ["bg-accent", "bg-attention", "bg-highlight", "bg-fg-faint"];

export function PaymentBars({ report }: { report: SalesReport }) {
  const payments = report.totals.payments;
  // Null when a method refunded more than it took: a share of that means nothing.
  const share = shares(payments.map((p) => p.net.minor));
  if (payments.length === 0) return <Nothing>Nothing was paid in these days.</Nothing>;
  return (
    <ul className="grid gap-4 text-base">
      {payments.map((p, i) => (
        <li key={p.method} className="flex items-center gap-3">
          <span className="w-14 shrink-0 text-fg-2">{methodName(p.method)}</span>
          <span aria-hidden="true" className="h-2.5 min-w-8 flex-1 overflow-hidden rounded-pill bg-sunken">
            <span className={cx("block h-full rounded-pill", methodFills[Math.min(i, methodFills.length - 1)])} style={{ width: `${share?.[i] ?? 0}%` }} />
          </span>
          {share && <span className="w-10 shrink-0 text-right font-mono font-bold tabular-nums">{share[i]}%</span>}
          <Money amount={p.net} className="w-28 shrink-0 text-fg-2" />
        </li>
      ))}
    </ul>
  );
}
