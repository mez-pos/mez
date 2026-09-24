// The open tickets (Task 5): every ticket at the venue not yet closed, with
// where it is, how long it has run and what it comes to; and a new one,
// at the counter or at a table. Tap one to bring it to the front; a held
// one is taken off hold as it comes.

import { useEffect, useState } from "react";
import type { OpenOrder, OpenTicketsResponse, TicketSummary } from "../../../shared/api-types";
import { Button, Money, NumPad, Qty, Sheet, StatusChip, applyAmountKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { elapsed, ticketLabel } from "./ticket-time";

interface TicketsSheetProps {
  open: OpenTicketsResponse | undefined;
  /** The order in front, to mark it. */
  currentId: string | null;
  onPick: (ticket: TicketSummary) => Promise<void>;
  onNew: (opening: OpenOrder) => void;
  onClose: () => void;
}

export function TicketsSheet({ open, currentId, onPick, onNew, onClose }: TicketsSheetProps) {
  const now = useClock(15_000);
  const [error, setError] = useState<string | null>(null);
  const [table, setTable] = useState(0);
  const [covers, setCovers] = useState(2);
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
                        <span className="text-base font-semibold">{ticketLabel(t)}</span>
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

        <section aria-label="New ticket" className="flex flex-col gap-4 border-t border-line pt-6">
          <h3 className="text-lg font-semibold">New ticket</h3>
          <Button onClick={() => start({ channel: "counter", covers: 0 })}>Counter sale</Button>
          <div className="flex items-center justify-between gap-4">
            <span className="text-base text-fg-2">
              Table <span className="font-mono tabular-nums">{table || "–"}</span>
            </span>
            <Qty value={covers} onChange={setCovers} label="Covers" bounds={{ min: 1, max: 99 }} />
          </div>
          <NumPad layout="pin" label="Table number" onKey={(k) => setTable((t) => applyAmountKey(t, k, 999))} />
          <Button disabled={table === 0} onClick={() => start({ channel: "counter", table_id: table, covers })}>
            {table ? `Start table ${table}, ${covers} ${covers === 1 ? "cover" : "covers"}` : "Enter a table number"}
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
