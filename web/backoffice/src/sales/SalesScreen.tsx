// The Sales reports screen (D15): a range of trading days added up by the
// server (ADR-118). The top is the design's: the title with the range and
// Export beside it, four figures set against the days before, the trend by
// how it was served, sales by category, the top sellers, orders by type and
// how it was paid. Under "In detail" are the reports the design does not
// draw and the back office has (rebuild.md, rule 4): the range's figures,
// each period, the channels, the heatmap and the corrections, each with its
// CSV (ADR-119). What sold, in full, and the stock are on Items and inventory
// (D16); who sold it is on Staff (D17).

import { BanknoteIcon, ReceiptTextIcon, TagIcon, UsersIcon } from "lucide-react";
import type { ReportSheet, SalesReport } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { change } from "../dashboard/dashboard-logic";
import { Panel } from "../dashboard/Panel";
import { ChangeNote, StatTile } from "../dashboard/StatTile";
import { beforeText } from "./overview-logic";
import { useReportRange } from "./report-range";
import { ReportState } from "./ReportState";
import { CategoryRing, OrderTypes, PaymentBars, SalesTrend, ServiceLegend, TopSelling } from "./SalesCharts";
import { Corrections } from "./SalesCorrections";
import { ExportMenu, ReportHead, sheets } from "./SalesHead";
import { Headline, Mixes, OverTime } from "./SalesSections";
import { Heatmap } from "./SalesTables";
import { useCsvDownload, useCsvFile } from "./use-csv-download";
import { useRangeSales } from "./use-range-sales";
import { averageBill, csvPath } from "./sales-logic";

// Three across from 1280px, as the dashboard's rows are; at the design's
// width the two cards beside the wide one are its 420px.
const row = "grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,2fr)] min-[1800px]:grid-cols-[minmax(0,1fr)_420px_420px]";

export function SalesScreen() {
  const { range, every, picked } = useReportRange();
  const { report, shown, prior, stale } = useRangeSales();
  // One download at a time, whether from Export or a table's own button.
  const file = useCsvFile();
  const button = useCsvDownload(file);
  // Not while the figures shown are the last range's: a file must match its screen.
  const csv = (sheet: ReportSheet) => button(csvPath(range, every, sheet), stale);

  return (
    <div className="flex flex-col gap-4">
      <ReportHead tab="sales" title="Sales Reports" cut>
        <ExportMenu file={file} files={sheets.map((s) => ({ label: s.label, path: csvPath(range, every, s.id) }))} disabled={!shown || stale} />
      </ReportHead>

      <ReportState picked={picked} pending={report.isPending} error={report.error} onRetry={() => void report.refetch()} />
      {shown && (
        <div aria-busy={stale} className={cx("flex flex-col gap-4", stale && "opacity-40")}>
          <Figures report={shown} prior={prior} against={beforeText(range)} />

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,29rem)]">
            <Panel title="Sales trend" aside={<ServiceLegend />}>
              <SalesTrend report={shown} />
            </Panel>
            <Panel title="Sales by category">
              <CategoryRing report={shown} />
            </Panel>
          </div>

          <div className={row}>
            <Panel title="Top selling items" more={{ label: "View all", href: "/sales/items" }} className="lg:col-span-2 xl:col-span-1">
              <TopSelling report={shown} />
            </Panel>
            <Panel title="Order type breakdown">
              <OrderTypes report={shown} />
            </Panel>
            <Panel title="Sales by payment method">
              <PaymentBars report={shown} />
            </Panel>
          </div>

          <h2 className="mt-4 text-meta font-bold uppercase text-fg-faint">In detail</h2>
          <div className="flex max-w-6xl flex-col gap-6">
            <Headline report={shown} csv={csv("summary")} />
            <OverTime report={shown} csv={csv("over_time")} />
            <Mixes report={shown} paymentsCsv={csv("payments")} channelsCsv={csv("channels")} />
            <Heatmap report={shown} csv={csv("hours")} />
          </div>
        </div>
      )}
      {/* Its own read, so the sales above show while it adds up. */}
      {picked && (
        <div className="flex max-w-6xl flex-col gap-6">
          <Corrections range={range} />
        </div>
      )}
    </div>
  );
}

interface FiguresProps {
  report: SalesReport;
  /** The same number of days just before, or null while it is read or when it could not be. */
  prior: SalesReport | null;
  /** What `prior` is, in words: "the 7 days before". */
  against: string;
}

/** The four figures along the top (D15): sales, orders, the average order and the guests. */
function Figures({ report, prior, against }: FiguresProps) {
  const now = report.totals.sales;
  const then = prior?.totals.sales ?? null;
  const average = averageBill(now.total, now.bills);
  const priorAverage = then ? averageBill(then.total, then.bills) : null;
  const note = (today: number, before: number | undefined) => before !== undefined && <ChangeNote change={change(today, before)} against={against} />;

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <StatTile icon={BanknoteIcon} tone="success" label="Total sales" featured note={note(now.total.minor, then?.total.minor)}>
        <Money amount={now.total} variant="total" />
      </StatTile>
      <StatTile icon={ReceiptTextIcon} tone="attention" label="Total orders" note={note(now.bills, then?.bills)}>
        {now.bills}
      </StatTile>
      <StatTile icon={TagIcon} tone="warning" label="Average order value" note={average && note(average.minor, then ? (priorAverage?.minor ?? 0) : undefined)}>
        {average ? <Money amount={average} variant="total" /> : <span className="font-sans text-lg font-normal text-fg-muted">No bills</span>}
      </StatTile>
      <StatTile icon={UsersIcon} tone="info" label="Total guests" note={note(now.covers, then?.covers)}>
        {now.covers}
      </StatTile>
    </div>
  );
}
