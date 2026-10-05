// What sold, who sold it, and when (backoffice-reports.md, phase 10 Task
// 5d): product mix, category performance, staff performance and the hour
// by weekday heatmap. Every figure is the server's (ADR-118); sorting only
// reorders its rows.

import { useState, type ReactNode } from "react";
import type { Group, SalesReport } from "../../../shared/api-types";
import { Card, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import type { Sort } from "../../../shared/ui/table-state";
import { Section, Share, count } from "./SalesSections";
import { averageBill, cellText, defaultGroupSort, heatGrid, hourText, shares, sortGroups, tint, weekdayName } from "./sales-logic";

interface GroupTableProps {
  label: string;
  /** The first column's heading: "Item", "Category", "Opened by". */
  heading: string;
  /** What Qty counts for these groups: units, or bills. */
  qty: string;
  groups: readonly Group[];
  name: (g: Group) => string;
  /** The columns between Qty and the share, which differ by table. */
  figures: Column<Group>[];
}

/** A table of groups, sorted by its headers, each row with its share of the total. */
function GroupTable({ label, heading, qty, groups, name, figures }: GroupTableProps) {
  const [sort, setSort] = useState<Sort>(defaultGroupSort);
  // Shares are of the whole table, whatever order it is shown in.
  const percents = shares(groups.map((g) => g.total.minor));
  const share = new Map(groups.map((g, i) => [g.key, percents?.[i]]));
  const columns: Column<Group>[] = [
    { id: "name", header: heading, sortable: true, cell: (g) => name(g) },
    { id: "qty", header: qty, align: "right", sortable: true, cell: (g) => count(g.qty) },
    ...figures,
    { id: "total", header: "Total", align: "right", sortable: true, cell: (g) => <Money amount={g.total} /> },
    { id: "share", header: "Share", align: "right", cell: (g) => <Share percent={share.get(g.key)} /> },
  ];
  return (
    <div className="max-h-96 overflow-y-auto">
      <DataTable
        label={label}
        columns={columns}
        rows={sortGroups(groups, sort)}
        rowKey={(g) => g.key}
        sort={sort}
        onSortChange={setSort}
        empty={<EmptyState icon={<InboxIcon />} line="Nothing sold." />}
      />
    </div>
  );
}

const money = (id: string, header: string, pick: (g: Group) => Group["total"]): Column<Group> => ({
  id,
  header,
  align: "right",
  cell: (g) => <Money amount={pick(g)} />,
});

const soldFigures: Column<Group>[] = [
  money("gross", "Gross", (g) => g.gross),
  money("discounts", "Discounts", (g) => g.discounts),
  money("net", "Net", (g) => g.net),
  money("tax", "Tax", (g) => g.tax),
];

const staffFigures: Column<Group>[] = [
  money("gross", "Gross", (g) => g.gross),
  money("discounts", "Discounts", (g) => g.discounts),
  {
    id: "average",
    header: "Average bill",
    align: "right",
    cell: (g) => {
      const average = averageBill(g.total, g.qty);
      return average ? <Money amount={average} /> : null;
    },
  },
];

/** What sold, by item and by category, in full (D16, under the item performance card). */
export function SoldItems({ report, itemsCsv, categoriesCsv }: { report: SalesReport; itemsCsv: ReactNode; categoriesCsv: ReactNode }) {
  const t = report.totals;
  return (
    <>
      <Section title="What sold" csv={itemsCsv} id="what-sold">
        <GroupTable label="Sales by item" heading="Item" qty="Qty" groups={t.by_item} name={(g) => g.name} figures={soldFigures} />
      </Section>
      <Section title="By category" csv={categoriesCsv}>
        <GroupTable label="Sales by category" heading="Category" qty="Qty" groups={t.by_category} name={(g) => g.name} figures={soldFigures} />
      </Section>
    </>
  );
}

/** Sales by who opened the bill. It stays on Sales reports until the staff reports have their screen (16e). */
export function WhoSold({ report, csv }: { report: SalesReport; csv: ReactNode }) {
  return (
    <Section title="Who sold it" csv={csv}>
      <GroupTable
        label="Sales by who opened the bill"
        heading="Opened by"
        qty="Bills"
        groups={report.totals.by_staff}
        name={(g) => g.name || "A guest"}
        figures={staffFigures}
      />
    </Section>
  );
}

export function Heatmap({ report, csv }: { report: SalesReport; csv: ReactNode }) {
  const grid = heatGrid(report.hours);
  const top = grid.busiest?.total.minor ?? 0;
  return (
    <Section title="When it sold" csv={csv}>
      {grid.busiest === null ? (
        <Card>
          <EmptyState icon={<InboxIcon />} line="No bills to place." />
        </Card>
      ) : (
        <Card className="flex flex-col gap-3">
          <div className="overflow-x-auto">
            <table aria-label="Bills by weekday and hour" className="border-collapse text-sm">
              <thead>
                <tr>
                  <th scope="col">
                    <span className="sr-only">Weekday</span>
                  </th>
                  {grid.hours.map((h) => (
                    <th key={h} scope="col" className="px-1 pb-1.5 text-right font-mono font-normal tabular-nums text-fg-muted">
                      {hourText(h)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {grid.rows.map((row) => (
                  <tr key={row.weekday}>
                    <th scope="row" className="whitespace-nowrap pr-3 text-left font-semibold text-fg-2">
                      {weekdayName(row.weekday)}
                    </th>
                    {row.cells.map((c, i) => (
                      <td key={grid.hours[i]} className="p-0.5">
                        {/* An empty cell keeps its size, so the rows and columns stay a grid. */}
                        {c === null ? (
                          <div className="h-8 min-w-10 rounded-tight bg-sunken" />
                        ) : (
                          <div title={cellText(c)} className="relative flex h-8 min-w-10 items-center justify-end overflow-hidden rounded-tight px-1.5">
                            {/* The tint repeats the count beside it; its strength is data. */}
                            <span aria-hidden className="absolute inset-0 bg-accent" style={{ opacity: tint(c.total.minor, top) * 0.45 }} />
                            <span aria-hidden className="relative font-mono tabular-nums text-fg">
                              {c.bills}
                            </span>
                            <span className="sr-only">{cellText(c)}</span>
                          </div>
                        )}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-sm text-fg-3">
            Busiest: <span className="font-mono tabular-nums">{cellText(grid.busiest)}</span>
          </p>
        </Card>
      )}
    </Section>
  );
}
