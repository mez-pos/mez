// Sorting and cursor paging for DataTable, apart from the component.
//
// With cursor pagination the server sorts, so the table only says what was
// asked for. A cursor only points forward, so going back needs the cursors
// already visited: a stack, whose top is the page on screen.

export type SortDirection = "asc" | "desc";

export interface Sort {
  readonly column: string;
  readonly direction: SortDirection;
}

/** A header tapped: a new column sorts ascending; the same one flips. */
export function nextSort(current: Sort | null, column: string): Sort {
  if (current?.column !== column) return { column, direction: "asc" };
  return { column, direction: current.direction === "asc" ? "desc" : "asc" };
}

export interface CursorPages {
  /** The cursor that fetched each page seen; null is the first page. */
  readonly stack: readonly (string | null)[];
}

export const firstPage: CursorPages = { stack: [null] };

export function currentCursor(pages: CursorPages): string | null {
  return pages.stack[pages.stack.length - 1] ?? null;
}

export function pageNumber(pages: CursorPages): number {
  return pages.stack.length;
}

export function nextPage(pages: CursorPages, cursor: string): CursorPages {
  return { stack: [...pages.stack, cursor] };
}

export function previousPage(pages: CursorPages): CursorPages {
  return pages.stack.length > 1 ? { stack: pages.stack.slice(0, -1) } : pages;
}
