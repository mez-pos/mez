// The Sales reports screen (D15): a range of trading days added up by the
// server (ADR-118). The top is the design's: the title with the range and
// Export beside it, four figures set against the days before, the trend by
// how it was served, sales by category, the top sellers, orders by type and
// how it was paid. Under "In detail" are the reports the design does not
// draw and the back office has (rebuild.md, rule 4): the range's figures,
// each period, the channels, the full tables, the heatmap, the corrections
// and the closing stock, each with its CSV (ADR-119).

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { BanknoteIcon, ReceiptTextIcon, TagIcon, UsersIcon } from "lucide-react";
import { useState } from "react";
import type { ReportEvery, ReportSheet, SalesReport } from "../../../shared/api-types";
import { ApiError } from "../../../shared/api";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { change } from "../dashboard/dashboard-logic";
import { Panel } from "../dashboard/Panel";
import { ChangeNote, StatTile } from "../dashboard/StatTile";
import { beforeText, insightText, previousRange, usable } from "./overview-logic";
import { CategoryRing, OrderTypes, PaymentBars, SalesTrend, ServiceLegend, TopSelling } from "./SalesCharts";
import { ClosingStock } from "./SalesClosingStock";
import { Corrections } from "./SalesCorrections";
import { ExportMenu, RangeMenu } from "./SalesHead";
import { Headline, Mixes, OverTime } from "./SalesSections";
import { Heatmap, Sold } from "./SalesTables";
import { useCsvDownload, useCsvFile } from "./use-csv-download";
import { averageBill, csvPath, everyFor, quickRange, quickRangeOf, salesPath, type SalesRange } from "./sales-logic";

// Three across from 1280px, as the dashboard's rows are; at the design's
// width the two cards beside the wide one are its 420px.
const row = "grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,2fr)] min-[1800px]:grid-cols-[minmax(0,1fr)_420px_420px]";

export function SalesScreen() {
  const store = useAuthStore();
  // One download at a time, whether from Export or a table's own button.
  const file = useCsvFile();
  const button = useCsvDownload(file);
  const [range, setRange] = useState<SalesRange>(() => quickRange("week", new Date()));
  // Null until chosen by hand: until then the cut follows the range.
  const [chosen, setChosen] = useState<ReportEvery | null>(null);
  const every = chosen ?? everyFor(range);
  const quick = quickRangeOf(range, new Date());
  const picked = usable(range);
  const before = picked ? previousRange(range) : null;

  const report = useQuery({
    queryKey: ["sales", range, every],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath(range, every), { signal }),
    enabled: picked,
    // The last range's figures stay, dimmed, while the next is read.
    placeholderData: keepPreviousData,
  });
  // The same number of days just before, for "up on the 7 days before". Its
  // totals alone are read, so it is cut as coarsely as the server allows.
  const prior = useQuery({
    queryKey: ["sales", before, "month"],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath(before ?? range, "month"), { signal }),
    enabled: before !== null,
  });

  // Not while the figures shown are the last range's: a file must match its screen.
  const csv = (sheet: ReportSheet) => button(csvPath(range, every, sheet), report.isPlaceholderData);
  const shown = picked && report.data && !report.error ? report.data : null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Sales Reports</h1>
          <p className="mt-1 text-base text-fg-muted">{insightText(range)}</p>
        </div>
        <div className="flex flex-wrap gap-2.5">
          <RangeMenu range={range} quick={quick} every={every} onRange={setRange} onEvery={setChosen} />
          <ExportMenu file={file} path={(sheet) => csvPath(range, every, sheet)} disabled={!shown || report.isPlaceholderData} />
        </div>
      </div>

      {!picked && <p className="text-base text-fg-muted">Pick a first and a last day.</p>}
      {picked && report.isPending && <p className="text-base text-fg-muted">Adding up the range</p>}
      {picked && report.error && (
        <EmptyState
          icon={<InboxIcon />}
          line={
            report.error instanceof ApiError && !report.error.unreachable
              ? report.error.message
              : "Can't reach the mez server. Check this computer's connection."
          }
          action={{ label: "Try again", onClick: () => void report.refetch() }}
        />
      )}
      {shown && (
        <div aria-busy={report.isPlaceholderData} className={cx("flex flex-col gap-4", report.isPlaceholderData && "opacity-40")}>
          {/* The days before are compared only once the range on screen is the one asked for. */}
          <Figures report={shown} prior={report.isPlaceholderData ? null : (prior.data ?? null)} against={beforeText(range)} />

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,29rem)]">
            <Panel title="Sales trend" aside={<ServiceLegend />}>
              <SalesTrend report={shown} />
            </Panel>
            <Panel title="Sales by category">
              <CategoryRing report={shown} />
            </Panel>
          </div>

          <div className={row}>
            <Panel
              title="Top selling items"
              more={{ label: "View all", onClick: () => document.getElementById("what-sold")?.scrollIntoView({ behavior: "smooth" }) }}
              className="lg:col-span-2 xl:col-span-1"
            >
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
            <Sold report={shown} itemsCsv={csv("items")} categoriesCsv={csv("categories")} staffCsv={csv("staff")} />
            <Heatmap report={shown} csv={csv("hours")} />
          </div>
        </div>
      )}
      {/* Their own reads, so the sales above show while they add up. */}
      <div className="flex max-w-6xl flex-col gap-6">
        {picked && <Corrections range={range} />}
        {picked && <ClosingStock range={range} />}
      </div>
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
