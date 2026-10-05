// The dashboard's four lists (D12): what sold most, the tables in use, who
// opened the day's bills, and what is running low. Each is a handful of rows;
// the screen it links to has the rest.

import { PackageIcon } from "lucide-react";
import type { Group, LiveTableResponse, SalesReport, StockItem } from "../../../shared/api-types";
import { initials, roleText } from "../../../shared/auth/format";
import { formatQty } from "../../../shared/stock-qty";
import { Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { activeTables, guestsText, lowRows, staffRows, topItems } from "./dashboard-logic";
import { Nothing } from "./Panel";

/** How many rows a list has room for beside the chart. */
const listRows = 5;

const row = "flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0";

export function TopItems({ items }: { items: readonly Group[] }) {
  const top = topItems(items, listRows);
  if (top.length === 0) return <Nothing>Nothing has sold yet today.</Nothing>;
  return (
    <ol>
      {top.map((g, i) => (
        <li key={g.key} className={row}>
          <span aria-hidden="true" className="w-4 shrink-0 font-mono text-sm font-bold tabular-nums text-fg-muted">
            {i + 1}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-base font-bold">{g.name}</p>
            <p className="font-mono text-sm tabular-nums text-fg-muted">{g.qty} sold</p>
          </div>
          <Money amount={g.total} className="shrink-0 text-lg font-semibold text-fg-heading" />
        </li>
      ))}
    </ol>
  );
}

export function ActiveTables({ tables, now }: { tables: readonly LiveTableResponse[]; now: number }) {
  const { rows, more } = activeTables(tables, now, listRows);
  if (rows.length === 0) return <Nothing>Every table is free.</Nothing>;
  return (
    <>
      <ul>
        {rows.map((t) => (
          <li key={t.id} className={row}>
            <span className="flex h-9 min-w-11 shrink-0 items-center justify-center rounded-tight bg-sunken px-2 text-sm font-bold text-fg-heading">
              {t.name}
            </span>
            {/* The wait sits under the guests until the card is the design's 400px, then at the row's end. */}
            <span className="min-w-0 flex-1 text-sm tabular-nums">
              <span className="block truncate">{guestsText(t.covers)}</span>
              <span className="block font-mono text-fg-muted min-[1800px]:hidden">{t.since}</span>
            </span>
            <StatusChip status={{ kind: "table", status: t.status }} />
            <span className="hidden min-w-16 shrink-0 text-right font-mono text-sm tabular-nums text-fg-muted min-[1800px]:block">{t.since}</span>
          </li>
        ))}
      </ul>
      <More count={more} one="more table in use" many="more tables in use" />
    </>
  );
}

const head = "pb-2.5 text-meta font-bold uppercase text-fg-muted";

export function StaffPerformance({ report }: { report: SalesReport }) {
  const rows = staffRows(report.totals.by_staff, report.roles, listRows);
  if (rows.length === 0) return <Nothing>Nobody has a paid bill yet today.</Nothing>;
  return (
    <table className="w-full text-base">
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
        </tr>
      </thead>
      <tbody>
        {rows.map((s, i) => (
          <tr key={s.id} className="border-b border-line-soft last:border-b-0">
            <td className="py-2.5 font-mono tabular-nums text-fg-muted">{i + 1}</td>
            <td className="py-2.5">
              <span className="flex items-center gap-2.5">
                <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-round bg-raised-high text-sm font-bold text-fg-2">
                  {initials(s.name) || "?"}
                </span>
                <strong className="truncate">{s.name}</strong>
              </span>
            </td>
            <td className="py-2.5 text-fg-2">{s.role ? roleText(s.role) : "No longer on record"}</td>
            <td className="py-2.5 text-right font-mono tabular-nums">{s.bills}</td>
            <td className="py-2.5 text-right">
              <Money amount={s.total} className="font-bold" />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

const inks = { low: "text-warning", out: "text-danger" } as const;

export function LowStock({ items }: { items: readonly StockItem[] }) {
  const { rows, more } = lowRows(items, listRows);
  if (rows.length === 0) return <Nothing>Nothing is low.</Nothing>;
  return (
    <>
      <ul>
        {rows.map(({ item, level }) => (
          <li key={item.id} className={row}>
            <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-tight bg-raised-high text-fg-muted">
              <PackageIcon size={18} />
            </span>
            <span className="min-w-0 flex-1 truncate text-base font-semibold">{item.name}</span>
            <span className={cx("shrink-0 font-mono text-sm font-semibold tabular-nums", inks[level])}>{formatQty(item.on_hand, item.unit)} left</span>
            <StatusChip status={{ kind: "stock", status: level }} />
          </li>
        ))}
      </ul>
      <More count={more} one="more item is low" many="more items are low" />
    </>
  );
}

/** The line under a list that had more than it could show. */
function More({ count, one, many }: { count: number; one: string; many: string }) {
  if (count <= 0) return null;
  return (
    <p className="pt-2.5 text-sm text-fg-muted">
      <span className="font-mono tabular-nums">{count}</span> {count === 1 ? one : many}
    </p>
  );
}
