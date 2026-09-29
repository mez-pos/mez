// The open tickets (Task 5): every ticket at the venue not yet closed, with
// where it is, how long it has run and what it comes to; and a new one,
// at the counter, or at a table by way of the floor (terminal-floor.md). Tap one to bring it to the front; a held
// one is taken off hold as it comes.

import { useEffect, useState } from "react";
import type { OpenOrder, OpenTicketsResponse, TicketSummary } from "../../../shared/api-types";
import { Button, Money, Sheet, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { elapsed, ticketLabel } from "./ticket-time";

interface TicketsSheetProps {
  open: OpenTicketsResponse | undefined;
  /** The order in front, to mark it. */
  currentId: string | null;
  /** Table names by id, from the floor. */
  names: ReadonlyMap<number, string>;
  onPick: (ticket: TicketSummary) => Promise<void>;
  onNew: (opening: OpenOrder) => void;
  /** Opens the settled bills, to refund, void a payment or reprint. */
  onPastBills: () => void;
  /** Opens the floor, where a party is seated at a table. */
  onFloor: () => void;
  /** Counts the drawer and ends the shift (terminal-close.md). */
  onCloseDay: () => void;
  onClose: () => void;
}

export function TicketsSheet({ open, currentId, names, onPick, onNew, onPastBills, onFloor, onCloseDay, onClose }: TicketsSheetProps) {
  const now = useClock(15_000);
  const [error, setError] = useState<string | null>(null);
  const lateAfter = open?.late_after_minutes ?? 20;

  const pick = async (t: TicketSummary) => {
    setError(null);
    try {
      await onPick(t);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };
  const start = (opening: OpenOrder) => {
    onNew(opening);
    onClose();
  };

  return (
    <Sheet open onClose={onClose} title="Tickets">
      <div className="flex flex-col gap-6">
        {error && (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        )}
        {!open && <p className="text-sm text-fg-muted">Loading tickets</p>}
        {open && open.orders.length === 0 && <p className="text-base text-fg-muted">No open tickets.</p>}
        {open && open.orders.length > 0 && (
          <ul className="flex flex-col gap-tap-gap">
            {open.orders.map((t) => {
              const time = elapsed(t.opened_at, now, lateAfter);
              const here = t.id === currentId;
              return (
                <li key={t.id}>
                  <button
                    type="button"
                    aria-current={here ? "true" : undefined}
                    onClick={() => void pick(t)}
                    className={cx(
                      "grid min-h-tap w-full grid-cols-[1fr_auto] items-center gap-3 rounded border px-4 py-3 text-left",
                      "outline-offset-2 outline-line-focus focus-visible:outline-2",
                      here ? "border-line-focus bg-raised-high" : "border-line bg-raised hover:bg-raised-high",
                    )}
                  >
                    <span className="flex min-w-0 flex-col gap-1">
                      <span className="flex items-center gap-2">
                        <span className="text-base font-semibold">{ticketLabel(t, names)}</span>
                        {t.status !== "open" && <StatusChip status={{ kind: "order", status: t.status }} />}
                        {here && <span className="text-meta uppercase text-fg-faint">On screen</span>}
                      </span>
                      <span className="flex items-center gap-2 text-sm text-fg-muted">
                        <span className="font-mono">{t.id}</span>
                        <span>·</span>
                        <span>
                          {t.lines} {t.lines === 1 ? "item" : "items"}
                        </span>
                      </span>
                    </span>
                    <span className="flex flex-col items-end gap-1">
                      <Money amount={t.total} />
                      <span className="flex items-center gap-2">
                        {time.late && <StatusChip status={{ kind: "kitchen", status: "late" }} />}
                        <span className={cx("font-mono text-sm tabular-nums", time.late ? "text-accent-text" : "text-fg-muted")}>
                          {time.text}
                        </span>
                      </span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}

        <section aria-label="Past bills" className="flex flex-col gap-2 border-t border-line pt-6">
          <Button
            onClick={() => {
              onPastBills();
              onClose();
            }}
          >
            Past bills
          </Button>
          <p className="text-sm text-fg-muted">Refund, void a payment or reprint a settled bill.</p>
        </section>

        <section aria-label="Close the day" className="flex flex-col gap-2 border-t border-line pt-6">
          <Button
            onClick={() => {
              onCloseDay();
              onClose();
            }}
          >
            Close the day
          </Button>
          <p className="text-sm text-fg-muted">Count the drawer and end the shift.</p>
        </section>

        <section aria-label="New ticket" className="flex flex-col gap-4 border-t border-line pt-6">
          <h3 className="text-lg font-semibold">New ticket</h3>
          <Button onClick={() => start({ channel: "counter", covers: 0 })}>Counter sale</Button>
          <Button
            onClick={() => {
              onFloor();
              onClose();
            }}
          >
            Seat a table
          </Button>
        </section>
      </div>
    </Sheet>
  );
}

/** The time, every `every` ms, for a list whose ages must move. */
function useClock(every: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), every);
    return () => clearInterval(id);
  }, [every]);
  return now;
}
