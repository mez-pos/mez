// The orders still open, on close of day's first step (TBS-2, ADR-384).
// Submitting the count holds every till at the close, so before it the
// manager is told what is open, where, and given the way to them. With
// none open there is nothing to say; while the day's list is being read,
// nothing is said either, rather than "none" too soon.

import { ReceiptTextIcon, TriangleAlertIcon } from "lucide-react";
import type { DayOrderResponse } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { openOrdersTitle, openOrdersWhere } from "./open-orders-logic";

interface OpenOrdersNoteProps {
  /** The open orders; null until the day's list has been read. */
  open: readonly DayOrderResponse[] | null;
  /** The day's list could not be read. */
  failed: boolean;
  names: ReadonlyMap<number, string>;
  /** To the Orders screen, on its Open tab. */
  onSee: () => void;
}

export function OpenOrdersNote({ open, failed, names, onSee }: OpenOrdersNoteProps) {
  if (failed && open === null) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-tap-gap rounded-card border border-line py-2 pl-4 pr-2">
        <p className="text-base text-fg-2">The open orders could not be checked. Look in Orders before counting.</p>
        <Button onClick={onSee}>
          <ReceiptTextIcon aria-hidden="true" size={17} />
          Orders
        </Button>
      </div>
    );
  }
  const title = openOrdersTitle(open?.length ?? 0);
  if (!open || !title) return null;
  return (
    <div
      role="status"
      className="flex flex-wrap items-center justify-between gap-tap-gap rounded-card border border-chip-warning-border bg-chip-warning-bg py-3 pl-4 pr-2 short:py-2"
    >
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <TriangleAlertIcon aria-hidden="true" size={20} className="mt-0.5 shrink-0 text-chip-warning-fg" />
        <div className="min-w-0">
          <p className="text-base font-semibold text-chip-warning-fg">
            {title}: {openOrdersWhere(open, names)}.
          </p>
          <p className="text-sm text-fg">Once the count is in, every till waits for the close. Settle them first, or they carry into the next shift.</p>
        </div>
      </div>
      <Button onClick={onSee}>
        <ReceiptTextIcon aria-hidden="true" size={17} />
        See open orders
      </Button>
    </div>
  );
}
