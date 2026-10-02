// The Sales reports screen (docs/design/screens/backoffice-reports.md): a
// range of trading days added up by the server (ADR-118), and each report
// as a CSV file (ADR-119).

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { ReportEvery, ReportSheet, SalesReport } from "../../../shared/api-types";
import { ApiError } from "../../../shared/api";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { inputClass } from "../FormFields";
import { ClosingStock } from "./SalesClosingStock";
import { Corrections } from "./SalesCorrections";
import { Headline, Mixes, OverTime } from "./SalesSections";
import { Heatmap, Sold } from "./SalesTables";
import { useCsvDownload } from "./use-csv-download";
import { csvPath, everyFor, everyWords, quickRange, quickRangeOf, quickRanges, salesPath, type SalesRange } from "./sales-logic";

const everys = Object.keys(everyWords) as ReportEvery[];

const toggleClass = pillClass;

export function SalesScreen() {
  const store = useAuthStore();
  const button = useCsvDownload();
  const [range, setRange] = useState<SalesRange>(() => quickRange("week", new Date()));
  // Null until chosen by hand: until then the cut follows the range.
  const [chosen, setChosen] = useState<ReportEvery | null>(null);
  const every = chosen ?? everyFor(range);
  const quick = quickRangeOf(range, new Date());

  const report = useQuery({
    queryKey: ["sales", range, every],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath(range, every), { signal }),
    enabled: range.from !== "" && range.to !== "",
    // The last range's figures stay, dimmed, while the next is read.
    placeholderData: keepPreviousData,
  });

  const picked = range.from !== "" && range.to !== "";
  // Not while the figures shown are the last range's: a file must match its screen.
  const csv = (sheet: ReportSheet) => button(csvPath(range, every, sheet), report.isPlaceholderData);

  return (
    <div className="flex max-w-6xl flex-col gap-6">
      <div>
        <h1 className="font-serif text-total">Sales reports</h1>
        <p className="mt-1 text-sm text-fg-3">A sale counts on the trading day its bill was invoiced. A trading day runs 04:00 to 04:00.</p>
      </div>

      <div role="group" aria-label="Range" className="flex flex-wrap items-end gap-4">
        <div role="group" aria-label="Quick ranges" className="flex flex-wrap gap-tap-gap">
          {quickRanges.map((q) => (
            <button key={q.id} type="button" aria-pressed={quick === q.id} className={toggleClass(quick === q.id)} onClick={() => setRange(quickRange(q.id, new Date()))}>
              {q.label}
            </button>
          ))}
        </div>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          From
          <input type="date" className={cx(inputClass, "font-mono")} value={range.from} max={range.to} onChange={(e) => setRange((r) => ({ ...r, from: e.target.value }))} />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          To
          <input type="date" className={cx(inputClass, "font-mono")} value={range.to} min={range.from} onChange={(e) => setRange((r) => ({ ...r, to: e.target.value }))} />
        </label>
        <div role="group" aria-label="By" className="flex flex-col gap-1.5 text-sm font-semibold">
          By
          <div className="flex gap-tap-gap">
            {everys.map((e) => (
              <button key={e} type="button" aria-pressed={every === e} className={toggleClass(every === e)} onClick={() => setChosen(e)}>
                {everyWords[e]}
              </button>
            ))}
          </div>
        </div>
      </div>

      {!picked && <p className="text-base text-fg-muted">Pick a first and a last day.</p>}
      {picked && report.isPending && <p className="text-base text-fg-muted">Adding up the range</p>}
      {picked && report.error && (
        <EmptyState
          icon={<InboxIcon />}
          line={report.error instanceof ApiError && !report.error.unreachable ? report.error.message : "Can't reach the mez server. Check this computer's connection."}
          action={{ label: "Try again", onClick: () => void report.refetch() }}
        />
      )}
      {picked && report.data && !report.error && (
        <div aria-busy={report.isPlaceholderData} className={cx("flex flex-col gap-6", report.isPlaceholderData && "opacity-40")}>
          <Headline report={report.data} csv={csv("summary")} />
          <OverTime report={report.data} csv={csv("over_time")} />
          <Mixes report={report.data} paymentsCsv={csv("payments")} channelsCsv={csv("channels")} />
          <Sold report={report.data} itemsCsv={csv("items")} categoriesCsv={csv("categories")} staffCsv={csv("staff")} />
          <Heatmap report={report.data} csv={csv("hours")} />
        </div>
      )}
      {/* Its own read, so the sales above show while it adds up. */}
      {picked && <Corrections range={range} />}
      {picked && <ClosingStock range={range} />}
    </div>
  );
}
