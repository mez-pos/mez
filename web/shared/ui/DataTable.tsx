// A table for the back office: sortable, cursor-paginated
// (docs/design/components.md). Back office only, so it is not exported from
// ui/index.ts; import it from "ui/DataTable" and nothing guest loads can pull
// it in by accident.
//
// It holds no data and no sort: the owner fetches a page for (sort, cursor)
// and passes it in. table-state.ts has the sort and page arithmetic.

import type { ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";
import { SortIcon } from "./icons";
import { nextSort, type Sort } from "./table-state";

export interface Column<T> {
  id: string;
  header: string;
  cell: (row: T) => ReactNode;
  /** Numbers, money and times are right-aligned (tokens.md). */
  align?: "left" | "right";
  sortable?: boolean;
}

export interface Pagination {
  page: number;
  hasPrevious: boolean;
  hasNext: boolean;
  onPrevious: () => void;
  onNext: () => void;
}

export interface DataTableProps<T> {
  columns: readonly Column<T>[];
  rows: readonly T[];
  rowKey: (row: T) => string;
  /** Names the table for screen readers: "Today's orders". */
  label: string;
  sort?: Sort | null;
  onSortChange?: (sort: Sort) => void;
  pagination?: Pagination;
  loading?: boolean;
  /** Shown instead of rows when there are none: an EmptyState. */
  empty?: ReactNode;
}

export function DataTable<T>({ columns, rows, rowKey, label, sort = null, onSortChange, pagination, loading = false, empty }: DataTableProps<T>) {
  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-x-auto rounded-card border border-line bg-raised shadow-rest">
        <table aria-label={label} aria-busy={loading} className="w-full border-collapse text-base">
          <thead>
            <tr className="border-b border-line">
              {columns.map((column) => (
                <HeaderCell key={column.id} column={column} sort={sort} onSortChange={onSortChange} />
              ))}
            </tr>
          </thead>
          <tbody className={cx(loading && "opacity-40")}>
            {rows.map((row) => (
              <tr key={rowKey(row)} className="border-b border-line-soft last:border-b-0 hover:bg-raised-high">
                {columns.map((column) => (
                  <td key={column.id} className={cx("px-4 py-3.5 text-fg", column.align === "right" && "text-right tabular-nums")}>
                    {column.cell(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && !loading && empty}
      </div>
      {pagination && <Pager {...pagination} disabled={loading} />}
    </div>
  );
}

function HeaderCell<T>({ column, sort, onSortChange }: { column: Column<T>; sort: Sort | null; onSortChange: ((sort: Sort) => void) | undefined }) {
  const right = column.align === "right";
  const direction = sort?.column === column.id ? sort.direction : null;
  const ariaSort = direction === "asc" ? "ascending" : direction === "desc" ? "descending" : undefined;
  const base = cx("whitespace-nowrap px-4 py-3 text-meta font-bold uppercase text-fg-muted", right ? "text-right" : "text-left");

  if (!column.sortable || !onSortChange) {
    return (
      <th scope="col" className={base}>
        {column.header}
      </th>
    );
  }
  // Column headers are uppercase metadata, the one text allowed at 11px
  // (tokens.md); a sortable one keeps a full-height target around it.
  return (
    <th scope="col" aria-sort={ariaSort} className={cx(base, "p-0")}>
      <button
        type="button"
        onClick={() => onSortChange(nextSort(sort, column.id))}
        className={cx(
          "inline-flex h-10 w-full items-center gap-1.5 px-4 text-meta font-bold uppercase hover:text-fg",
          "outline-offset-2 outline-line-focus focus-visible:outline-2",
          right && "flex-row-reverse",
          direction && "text-fg",
        )}
      >
        {column.header}
        <SortIcon state={direction} />
      </button>
    </th>
  );
}

function Pager({ page, hasPrevious, hasNext, onPrevious, onNext, disabled }: Pagination & { disabled: boolean }) {
  return (
    <nav aria-label="Pages" className="flex items-center justify-end gap-tap-gap">
      <span className="mr-2 font-mono text-sm tabular-nums text-fg-muted">Page {page}</span>
      <Button size="md" onClick={onPrevious} disabled={disabled || !hasPrevious}>
        Previous
      </Button>
      <Button size="md" onClick={onNext} disabled={disabled || !hasNext}>
        Next
      </Button>
    </nav>
  );
}
