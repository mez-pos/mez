// The closing stock section of the Sales reports screen (backoffice-reports.md,
// phase 10 Task 5f3): what each close's stock count found against what the
// ledger expected, in units and at the cost of that night (ADR-125).

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { ClosingStockItemTotal, ClosingStockReport, Money as Amount, StockUnit } from "../../../shared/api-types";
import { ApiError } from "../../../shared/api";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { formatDelta, formatQty } from "../../../shared/stock-qty";
import { Card, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { dayText } from "../audit/audit-logic";
import { costMeasures } from "../stock/stock-logic";
import { Section, count } from "./SalesSections";
import { closingCsvPath, closingPath, closingRows, countsIn, uncostedText, varianceWords, type ClosingRow, type VarianceWords } from "./closing-logic";
import type { SalesRange } from "./sales-logic";
import { useCsvDownload } from "./use-csv-download";

const tones: Readonly<Record<VarianceWords["tone"], string>> = {
  danger: "text-danger",
  warning: "text-warning",
  success: "text-success",
  muted: "text-fg-muted",
};

const notCosted = <span className="text-fg-muted">Not costed</span>;

/** A signed quantity; below zero, short, in the danger colour as well as by its sign. */
function Variance({ qty, unit }: { qty: number; unit: StockUnit }) {
  return <span className={cx("whitespace-nowrap font-mono tabular-nums", qty < 0 && "text-danger")}>{formatDelta(qty, unit)}</span>;
}

/** A variance at cost: a loss in the danger colour as well as by its sign. */
function Worth({ amount }: { amount: Amount }) {
  return <Money amount={amount} tone={amount.minor < 0 ? "danger" : "default"} />;
}

export function ClosingStock({ range }: { range: SalesRange }) {
  const store = useAuthStore();
  const csv = useCsvDownload();
  const report = useQuery({
    queryKey: ["closing-stock", range],
    queryFn: ({ signal }) => store.api.get<ClosingStockReport>(closingPath(range), { signal }),
    placeholderData: keepPreviousData,
  });
  const stale = report.isPlaceholderData;
  // Nothing of the last answer stays beside an error: no headline, no file.
  const data = report.error ? undefined : report.data;
  const counts = data ? countsIn(data) : 0;
  const words = data && counts > 0 ? varianceWords(data.value, counts, data.uncosted) : null;
  // When nothing was costed the headline already says so.
  const leftOut = data && counts > 0 && data.uncosted < counts ? uncostedText(data.uncosted) : "";

  return (
    <div aria-busy={stale} className={cx(stale && "opacity-40")}>
      <Section title="Closing stock" csv={data && counts > 0 ? csv(closingCsvPath(range), stale) : null}>
        <div>
          {words && <p className={cx("text-base font-semibold", tones[words.tone])}>{words.text}</p>}
          {leftOut && <p className="mt-1 text-sm text-fg-muted">{leftOut}</p>}
          <p className="mt-1 text-sm text-fg-3">Only shifts whose close counted stock are listed. Each count is valued at what the item cost that night.</p>
        </div>

        {report.isPending && <p className="text-base text-fg-muted">Reading the closing counts</p>}
        {report.error && (
          <EmptyState
            icon={<InboxIcon />}
            line={report.error instanceof ApiError && !report.error.unreachable ? report.error.message : "Can't reach the mez server. Check this computer's connection."}
            action={{ label: "Try again", onClick: () => void report.refetch() }}
          />
        )}
        {data && counts === 0 && (
          <Card>
            <EmptyState icon={<InboxIcon />} line={`No closing counts between ${dayText(data.from)} and ${dayText(data.to)}.`} />
          </Card>
        )}
        {data && counts > 0 && (
          <div className="flex flex-col gap-6">
            <ByItem items={data.items} />
            <ByShift rows={closingRows(data)} />
          </div>
        )}
      </Section>
    </div>
  );
}

function ByItem({ items }: { items: readonly ClosingStockItemTotal[] }) {
  const columns: Column<ClosingStockItemTotal>[] = [
    { id: "item", header: "Item", cell: (it) => <span className="font-semibold">{it.name}</span> },
    { id: "counts", header: "Counts", align: "right", cell: (it) => count(it.counts) },
    { id: "variance", header: "Variance", align: "right", cell: (it) => <Variance qty={it.variance} unit={it.unit} /> },
    {
      id: "value",
      header: "At cost",
      align: "right",
      cell: (it) =>
        it.uncosted >= it.counts ? (
          notCosted
        ) : (
          <span className="flex flex-col items-end">
            <Worth amount={it.value} />
            {it.uncosted > 0 && <span className="text-fg-muted">{it.uncosted} not costed</span>}
          </span>
        ),
    },
  ];
  return <DataTable label="Closing stock by item" columns={columns} rows={items} rowKey={(it) => String(it.stock_item_id)} />;
}

function ByShift({ rows }: { rows: readonly ClosingRow[] }) {
  const qty = (pick: (r: ClosingRow) => number) => (r: ClosingRow) => <span className="whitespace-nowrap font-mono tabular-nums">{formatQty(pick(r), r.line.unit)}</span>;
  const columns: Column<ClosingRow>[] = [
    { id: "day", header: "Day", cell: (r) => <span className="whitespace-nowrap font-mono tabular-nums">{dayText(r.tradingDay)}</span> },
    { id: "item", header: "Item", cell: (r) => r.line.name },
    { id: "expected", header: "Expected", align: "right", cell: qty((r) => r.line.expected) },
    { id: "counted", header: "Counted", align: "right", cell: qty((r) => r.line.counted) },
    { id: "variance", header: "Variance", align: "right", cell: (r) => <Variance qty={r.line.variance} unit={r.line.unit} /> },
    {
      id: "cost",
      header: "Cost",
      align: "right",
      cell: (r) =>
        r.line.cost ? (
          <span className="whitespace-nowrap">
            <Money amount={r.line.cost} /> <span className="text-fg-3">/ {costMeasures[r.line.unit].short}</span>
          </span>
        ) : (
          notCosted
        ),
    },
    { id: "value", header: "At cost", align: "right", cell: (r) => (r.line.value ? <Worth amount={r.line.value} /> : <span className="sr-only">Not costed</span>) },
  ];
  return (
    <div className="max-h-96 overflow-y-auto">
      <DataTable label="Closing stock by shift" columns={columns} rows={rows} rowKey={(r) => r.key} />
    </div>
  );
}
