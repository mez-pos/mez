// Items and inventory (D16): how each item sold beside the days before, what
// was written off, and how the stock stands. Under "In detail" are the full
// tables the design does not draw (rebuild.md, rule 4): what sold by item and
// by category, and the closing stock counts.
//
// Cost and Margin are the server's (ADR-189): what the stock ledger moved for
// each dish, at the cost it had then. "Raise purchase order" is not drawn:
// purchasing is out of v1.

import { useQuery } from "@tanstack/react-query";
import { CheckIcon, CircleXIcon, MinusIcon, PackageIcon, TrendingDownIcon, TrendingUpIcon, TriangleAlertIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import type { ItemMargin, ItemsReport, ReportSheet, SalesReport, WasteReport } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { formatQty } from "../../../shared/stock-qty";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { DietMark } from "../../../shared/ui/DietMark";
import { useMenu } from "../../../shared/use-menu";
import { changeText, type Change } from "../dashboard/dashboard-logic";
import { Nothing, Panel, ReadState } from "../dashboard/Panel";
import { usePermissions } from "../shell/BackOfficeShell";
import { useStockItems } from "../stock/use-stock";
import {
  entriesText,
  itemRows,
  itemsText,
  namesText,
  reasonName,
  stockHealth,
  wasteBars,
  wasteLeftOut,
  wastePath,
  worstReason,
  costingNote,
  itemsPath,
  marginText,
} from "./items-logic";
import { beforeText } from "./overview-logic";
import { useReportRange } from "./report-range";
import { ReportState } from "./ReportState";
import { ClosingStock } from "./SalesClosingStock";
import { ExportMenu, ReportHead } from "./SalesHead";
import { SoldItems } from "./SalesTables";
import { closingCsvPath } from "./closing-logic";
import { csvPath } from "./sales-logic";
import { useCsvDownload, useCsvFile } from "./use-csv-download";
import { useRangeSales } from "./use-range-sales";

/** How many items the performance card lists; "What sold" under it has them all. */
const performanceRows = 8;
/** How many write-offs have a bar. */
const wasteRows = 5;

export function ItemsScreen() {
  const store = useAuthStore();
  const held = usePermissions();
  const { range, every, picked } = useReportRange();
  const { report, shown, prior, priorFailed, stale } = useRangeSales();
  const file = useCsvFile();
  const button = useCsvDownload(file);
  const csv = (sheet: ReportSheet) => button(csvPath(range, every, sheet), stale);

  const waste = useQuery({
    queryKey: ["wastage", range],
    queryFn: ({ signal }) => store.api.get<WasteReport>(wastePath(range), { signal }),
    enabled: picked,
  });
  // The stock list is the stock keeper's: without stock.manage the card is not drawn.
  const canStock = held.includes("stock.manage");
  const stock = useStockItems(canStock);

  return (
    <div className="flex flex-col gap-4">
      <ReportHead tab="items" title="Item and Inventory Reports">
        <ExportMenu
          file={file}
          files={[
            { label: "Items", path: csvPath(range, every, "items") },
            { label: "Categories", path: csvPath(range, every, "categories") },
            { label: "Closing stock", path: closingCsvPath(range) },
          ]}
          disabled={!shown || stale}
        />
      </ReportHead>

      <ReportState picked={picked} pending={report.isPending} error={report.error} onRetry={() => void report.refetch()} />
      {shown && (
        <div aria-busy={stale} className={cx(stale && "opacity-40")}>
          <Panel
            title="Item performance"
            more={{ label: "View all", onClick: () => document.getElementById("what-sold")?.scrollIntoView({ behavior: "smooth" }) }}
          >
            <ItemPerformance report={shown} prior={prior} against={beforeText(range)} failed={priorFailed} />
          </Panel>
        </div>
      )}

      {picked && (
        <div className={cx("grid grid-cols-1 gap-4", canStock && "lg:grid-cols-[minmax(0,1fr)_minmax(0,22rem)]")}>
          <Panel title="Wastage">
            {waste.data ? (
              <Wastage report={waste.data} />
            ) : (
              <ReadState what="what was written off" failed={waste.isError} onRetry={() => void waste.refetch()} />
            )}
          </Panel>
          {canStock && (
            <Panel title="Stock health">
              {stock.data ? (
                <StockHealthCard health={stockHealth(stock.data.items)} />
              ) : (
                <ReadState what="the stock" failed={stock.isError} onRetry={() => void stock.refetch()} />
              )}
            </Panel>
          )}
        </div>
      )}

      {picked && (
        <>
          <h2 className="mt-4 text-meta font-bold uppercase text-fg-faint">In detail</h2>
          <div className="flex max-w-6xl flex-col gap-6">
            {shown && (
              <div aria-busy={stale} className={cx("flex flex-col gap-6", stale && "opacity-40")}>
                <SoldItems report={shown} itemsCsv={csv("items")} categoriesCsv={csv("categories")} />
              </div>
            )}
            {/* Its own read, so the sales above show while it adds up. */}
            <ClosingStock range={range} />
          </div>
        </>
      )}
    </div>
  );
}

const head = "pb-2.5 text-meta font-bold uppercase text-fg-muted";

interface ItemPerformanceProps {
  report: SalesReport;
  prior: SalesReport | null;
  /** What `prior` is, in words: "the 7 days before". */
  against: string;
  /** The days before could not be read. */
  failed: boolean;
}

function ItemPerformance({ report, prior, against, failed }: ItemPerformanceProps) {
  const store = useAuthStore();
  const menu = useMenu();
  const range = { from: report.from, to: report.to };
  // What each item cost to make and what was left, worked out by the server (ADR-189).
  const costs = useQuery({
    queryKey: ["items-report", range],
    queryFn: ({ signal }) => store.api.get<ItemsReport>(itemsPath(range), { signal }),
  });
  const rows = itemRows(report.totals.by_item, prior?.totals.by_item ?? null, menu.data?.items ?? [], costs.data?.items ?? null, performanceRows);
  if (rows.length === 0) return <Nothing>Nothing sold in these days.</Nothing>;
  const note = costs.data ? costingNote(costs.data) : "";
  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[46rem] text-base">
          <thead>
            <tr className="border-b border-line text-left">
              <th scope="col" className={head}>
                Item
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Qty
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Net sales
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Cost
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Margin
              </th>
              <th scope="col" className={cx(head, "pl-6")}>
                Trend
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.key} className="border-b border-line-soft last:border-b-0">
                <td className="py-2.5">
                  <span className="flex items-center gap-2.5">
                    <DietMark diet={r.diet} className="" />
                    <strong>{r.name}</strong>
                  </span>
                </td>
                <td className="py-2.5 text-right font-mono tabular-nums">{r.qty}</td>
                <td className="py-2.5 text-right">
                  <Money amount={r.net} className="font-bold" />
                </td>
                <td className="py-2.5 text-right">
                  <Cost margin={r.margin} failed={costs.isError} />
                </td>
                <td className="py-2.5 text-right">
                  <Margin margin={r.margin} />
                </td>
                <td className="w-44 py-2.5 pl-6">
                  <Trend change={r.trend} against={against} failed={failed} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="pt-3 text-sm text-fg-muted">
        Net sales are after discounts and before tax. Cost is what the stock ledger moved for the dish, at what the stock cost then.
        {note && <> {note}</>}
      </p>
    </div>
  );
}

const quiet = "text-sm text-fg-muted";

/** What a dish cost to make: the figure, the figure marked as short of the whole, or why there is none. */
function Cost({ margin, failed }: { margin: ItemMargin | undefined; failed: boolean }) {
  if (!margin) return failed ? <span className={quiet}>Not read</span> : null;
  if (margin.costing === "none" || !margin.cost) return <span className={quiet}>Not costed</span>;
  if (margin.costing === "part") {
    // The cost covers fewer units than sold when the dish also sold before it had a recipe.
    const covers = margin.costed_qty !== margin.qty ? `${margin.costed_qty} of ${margin.qty} costed` : "Part costed";
    return (
      <span className="flex flex-col items-end">
        <Money amount={margin.cost} className="text-fg-2" />
        <span className={quiet}>{covers}</span>
      </span>
    );
  }
  return <Money amount={margin.cost} className="text-fg-2" />;
}

/** What was left: the amount and its share of net sales, in green above zero and brick below, with the sign as well. */
function Margin({ margin }: { margin: ItemMargin | undefined }) {
  if (!margin?.margin) return null;
  const ink = margin.margin.minor < 0 ? "text-danger" : "text-success";
  return (
    <span className="flex flex-col items-end">
      <Money amount={margin.margin} className={cx("font-bold", ink)} />
      {margin.margin_bp !== undefined && <span className={cx("font-mono text-sm tabular-nums", ink)}>{marginText(margin.margin_bp)}</span>}
    </span>
  );
}

const trends: Record<Change["direction"], { icon: LucideIcon; ink: string }> = {
  up: { icon: TrendingUpIcon, ink: "text-success" },
  down: { icon: TrendingDownIcon, ink: "text-danger" },
  same: { icon: MinusIcon, ink: "text-fg-muted" },
};

/** How many sold beside the days before: an arrow and the percentage, with the words for a screen reader. */
function Trend({ change, against, failed }: { change: Change | null | undefined; against: string; failed: boolean }) {
  if (change === undefined) return failed ? <span className="text-sm text-fg-muted">Not compared</span> : null;
  if (change === null) return <span className="text-sm text-fg-muted">New in these days</span>;
  const { icon: Icon, ink } = trends[change.direction];
  return (
    <span className={cx("flex items-center gap-1.5 text-sm font-bold", ink)} title={changeText(change, against)}>
      <span className="sr-only">{changeText(change, against)}</span>
      <Icon aria-hidden="true" size={15} strokeWidth={2.2} />
      <span aria-hidden="true">{change.direction === "same" ? "Level" : `${change.percent}%`}</span>
    </span>
  );
}

const wasteFills = ["bg-attention", "bg-highlight", "bg-fg-faint", "bg-accent", "bg-line-strong"];

function Wastage({ report }: { report: WasteReport }) {
  const { bars, more } = wasteBars(report.items, wasteRows);
  if (report.entries === 0) return <Nothing>Nothing was written off in these days.</Nothing>;
  const worst = worstReason(report.reasons);
  const leftOut = wasteLeftOut(report);
  return (
    <div>
      <ul className="grid gap-3.5 text-base">
        {bars.map((b, i) => (
          <li key={b.item.stock_item_id} className="flex items-center gap-3">
            <span className="w-32 shrink-0 truncate text-fg-2">{b.item.name}</span>
            <span aria-hidden="true" className="h-2.5 min-w-8 flex-1 overflow-hidden rounded-pill bg-sunken">
              <span className={cx("block h-full rounded-pill", wasteFills[Math.min(i, wasteFills.length - 1)])} style={{ width: `${b.percent}%` }} />
            </span>
            <span className="w-24 shrink-0 text-right font-mono text-sm tabular-nums text-fg-muted">{formatQty(b.item.qty, b.item.unit)}</span>
            {b.notCosted ? (
              <span className="w-28 shrink-0 text-right text-sm text-fg-muted">Not costed</span>
            ) : (
              <Money amount={b.item.value} className="w-28 shrink-0 font-bold" />
            )}
          </li>
        ))}
      </ul>
      {more > 0 && (
        <p className="pt-3 text-sm text-fg-muted">
          <span className="font-mono tabular-nums">{more}</span> more {more === 1 ? "item" : "items"} written off
        </p>
      )}
      <p className="mt-5 rounded bg-chip-warning-bg p-3.5 text-sm leading-relaxed text-fg">
        Total wastage <Money amount={report.value} className="font-bold" /> over {entriesText(report.entries)}.
        {worst && (
          <>
            {" "}
            {reasonName(worst.reason)} stock cost the most, <Money amount={worst.value} />.
          </>
        )}
        {leftOut && <> {leftOut}</>}
      </p>
    </div>
  );
}

const healthTiles = {
  healthy: { icon: CheckIcon, round: "bg-chip-success-bg text-chip-success-fg" },
  low: { icon: TriangleAlertIcon, round: "bg-chip-warning-bg text-chip-warning-fg" },
  out: { icon: CircleXIcon, round: "bg-chip-danger-bg text-chip-danger-fg" },
} as const;

function HealthRow({ kind, title, note }: { kind: keyof typeof healthTiles; title: string; note: string }) {
  const { icon: Icon, round } = healthTiles[kind];
  return (
    <li className="flex items-center gap-3">
      <span aria-hidden="true" className={cx("flex size-11 shrink-0 items-center justify-center rounded", round)}>
        <Icon size={20} />
      </span>
      <span className="min-w-0">
        <span className="block text-base font-bold">{title}</span>
        <span className="block truncate text-sm text-fg-muted">{note}</span>
      </span>
    </li>
  );
}

function StockHealthCard({ health }: { health: ReturnType<typeof stockHealth> }) {
  if (health.healthy + health.low + health.out === 0) return <Nothing>No stock items yet.</Nothing>;
  return (
    <div className="flex h-full flex-col">
      <ul className="grid gap-3.5">
        <HealthRow kind="healthy" title={`${itemsText(health.healthy)} healthy`} note="Above their low level" />
        <HealthRow kind="low" title={`${itemsText(health.low)} low`} note="At or under their low level" />
        <HealthRow kind="out" title={`${itemsText(health.out)} out`} note={health.out > 0 ? namesText(health.outNames) : "Nothing has run out"} />
      </ul>
      <Link
        href="/stock"
        className="mt-6 inline-flex min-h-tap items-center justify-center gap-2 rounded-pill bg-attention px-5 text-base font-semibold text-attention-fg outline-offset-2 outline-attention hover:bg-attention-hover focus-visible:outline-3"
      >
        <PackageIcon aria-hidden="true" size={17} />
        Go to stock
      </Link>
      <p aria-hidden="true" className="mt-auto pt-5 text-center font-script text-xl leading-tight text-fg-2">
        Great Food,
        <br />
        Brighter Days ♡
      </p>
    </div>
  );
}
