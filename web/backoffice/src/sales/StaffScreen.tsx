// Staff reports (D17): who opened the range's bills, what they sold and the
// tips on the payments they took. Under it, the lines each person rang up in
// error and took off, and under "In detail" the full table with its CSV
// (rebuild.md, rule 4).
//
// The design's Rating column and its customers, regulars and feedback cards are
// not drawn: nothing rates anyone until the guest channel (phase 11), and
// customers are out of v1.

import type { SalesReport } from "../../../shared/api-types";
import { initials, roleText } from "../../../shared/auth/format";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Nothing, Panel } from "../dashboard/Panel";
import { rangeText } from "./overview-logic";
import { useReportRange } from "./report-range";
import { ReportState } from "./ReportState";
import { ExportMenu, ReportHead } from "./SalesHead";
import { WhoSold } from "./SalesTables";
import { csvPath } from "./sales-logic";
import { linesText, removedLines, staffLines, tipTakers } from "./staff-logic";
import { useCsvDownload, useCsvFile } from "./use-csv-download";
import { useRangeSales } from "./use-range-sales";

export function StaffScreen() {
  const { range, every, picked } = useReportRange();
  const { report, shown, stale } = useRangeSales();
  const file = useCsvFile();
  const button = useCsvDownload(file);
  const staffCsv = csvPath(range, every, "staff");

  return (
    <div className="flex flex-col gap-4">
      <ReportHead tab="staff" title="Staff Reports">
        <ExportMenu file={file} files={[{ label: "Staff", path: staffCsv }]} disabled={!shown || stale} />
      </ReportHead>

      <ReportState picked={picked} pending={report.isPending} error={report.error} onRetry={() => void report.refetch()} />
      {shown && (
        <div aria-busy={stale} className={cx("flex flex-col gap-4", stale && "opacity-40")}>
          <Panel
            title={`Staff performance · ${rangeText(range)}`}
            aside={<p className="ml-auto text-right text-sm text-fg-muted">Tips are those on the payments each person took</p>}
          >
            <Performance report={shown} />
          </Panel>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Panel title="Removed in error">
              <Removed report={shown} />
            </Panel>
            <Panel title="Tips">
              <TipsCard report={shown} />
            </Panel>
          </div>

          <h2 className="mt-4 text-meta font-bold uppercase text-fg-faint">In detail</h2>
          <div className="flex max-w-6xl flex-col gap-6">
            <WhoSold report={shown} csv={button(staffCsv, stale)} />
          </div>
        </div>
      )}
    </div>
  );
}

const head = "pb-2.5 text-meta font-bold uppercase text-fg-muted";

function Avatar({ name }: { name: string }) {
  return (
    <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-round bg-raised-high text-sm font-bold text-fg-2">
      {initials(name) || "?"}
    </span>
  );
}

function Performance({ report }: { report: SalesReport }) {
  const rows = staffLines(report.totals, report.names, report.roles);
  if (rows.length === 0) return <Nothing>Nobody opened a bill or took a tip in these days.</Nothing>;
  return (
    <div className="relative overflow-x-auto">
      <table className="w-full min-w-160 text-base">
        <thead>
          <tr className="border-b border-line text-left">
            <th scope="col" className={cx(head, "w-8")}>
              #
            </th>
            <th scope="col" className={head}>
              Staff
            </th>
            <th scope="col" className={head}>
              Role
            </th>
            <th scope="col" className={cx(head, "text-right")}>
              Orders
            </th>
            <th scope="col" className={cx(head, "text-right")}>
              Sales
            </th>
            <th scope="col" className={cx(head, "text-right")}>
              Average
            </th>
            <th scope="col" className={cx(head, "text-right")}>
              Tips
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((s, i) => (
            <tr key={s.id} className="border-b border-line-soft last:border-b-0">
              <td className="py-3 font-mono tabular-nums text-fg-muted">{i + 1}</td>
              <td className="py-3">
                <span className="flex items-center gap-2.5">
                  <Avatar name={s.name} />
                  <strong>{s.name}</strong>
                </span>
              </td>
              <td className="py-3 text-fg-2">{s.guest ? "Ordered for themselves" : s.role ? roleText(s.role) : "No longer on record"}</td>
              <td className="py-3 text-right font-mono tabular-nums">{s.bills}</td>
              <td className="py-3 text-right">
                <Money amount={s.sales} className="font-bold" />
              </td>
              <td className="py-3 text-right">{s.average ? <Money amount={s.average} /> : <span className="text-sm text-fg-muted">No bills</span>}</td>
              <td className="py-3 text-right">
                <Money amount={s.tips} className="font-bold text-chip-warning-fg" />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const row = "flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0";

function Removed({ report }: { report: SalesReport }) {
  const rows = removedLines(report.totals.removed, report.names);
  if (rows.length === 0) return <Nothing>Nobody took a line off in these days.</Nothing>;
  return (
    <>
      <ul>
        {rows.map((r) => (
          <li key={r.id} className={row}>
            <Avatar name={r.name} />
            <span className="min-w-0 flex-1 truncate text-base font-semibold">{r.name}</span>
            <span className="shrink-0 font-mono text-sm tabular-nums text-fg-muted">{linesText(r.count)}</span>
            <Money amount={r.amount} className="w-28 shrink-0 font-bold" />
          </li>
        ))}
      </ul>
      <p className="pt-3 text-sm text-fg-muted">
        Lines rung up by mistake and taken off before the bill. Voids, comps and refunds are under Corrections on Sales.
      </p>
    </>
  );
}

function TipsCard({ report }: { report: SalesReport }) {
  const takers = tipTakers(report.totals.tips_by);
  return (
    <div>
      <Money amount={report.totals.tips} variant="total" />
      <p className="mt-1 text-sm text-fg-muted">
        {takers === 0
          ? "No tips were taken in these days."
          : takers === 1
            ? "Taken by one person, on their payments."
            : `Taken by ${takers} people, on their payments.`}
      </p>
      <p className="mt-3 text-sm text-fg-3">Tips are owed to staff and are never in sales. How a shift's tips were shared out is on its Z-report.</p>
    </div>
  );
}
