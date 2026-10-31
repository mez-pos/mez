// The panel of the tables screen (D05, the `rail` width): the table picked, and what can be
// done with it. A free table seats a party, one to clear is cleared, one with
// an order shows it (TableOrderView), and one with several asks which. With no
// table picked it says so: nothing here acts on a table nobody chose.
//
// Below xl the floor needs the whole width (ADR-381): the panel is drawn only
// once a table is picked, over the floor's right edge, with a way to close it,
// and it steps aside while a move or a join wants taps on the tiles.

import { ArmchairIcon, ArrowRightIcon, ChevronLeftIcon, TimerIcon, UsersIcon, XIcon } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { LiveTableResponse, TicketSummary } from "../../../shared/api-types";
import { Button, EmptyState, Money, Qty, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { tablePaid } from "../../../shared/ui/status";
import { wordsOf } from "../register/order-messages";
import { guestsText } from "../register/rail-logic";
import { elapsed } from "../register/ticket-time";
import { panelFor, startingCovers } from "./floor-logic";
import { TableOrderView, type PartyMode } from "./TableOrderView";

export interface TablePanelProps {
  table: LiveTableResponse | null;
  /** "Table T4 + T7": the party's tables, named. */
  title: string;
  now: number;
  /** Minutes after which an open order is late; 0 for never. */
  lateAfter: number;
  /** The venue's open orders, to describe a table's when it has several. */
  tickets: readonly TicketSummary[];
  /** Seats a party. With `thenOrder` it goes on to the menu, the order in front. */
  onSeat: (table: LiveTableResponse, covers: number, thenOrder: boolean) => Promise<void>;
  onClear: (table: LiveTableResponse) => Promise<void>;
  onRelease: (table: LiveTableResponse) => Promise<void>;
  onAddItems: (orderId: string, held: boolean) => Promise<void>;
  onSettle: (orderId: string) => void;
  onMode: (kind: PartyMode) => void;
  /** Whether the order still has items on their way to the server from this till. */
  sendWaits: (orderId: string) => boolean;
  /** Lets go of the table picked: below xl, that uncovers the floor. */
  onClose: () => void;
  /** A move or a join is on: below xl the panel is not drawn, so every tile can be tapped. */
  stepAside: boolean;
}

export function TablePanel(props: TablePanelProps) {
  return (
    <aside
      aria-label="Table"
      className={cx(
        "flex w-rail shrink-0 flex-col border-l border-line bg-chrome",
        "max-xl:absolute max-xl:inset-y-0 max-xl:right-0 max-xl:z-20 max-xl:shadow-modal",
        (!props.table || props.stepAside) && "max-xl:hidden",
      )}
    >
      {props.table ? (
        // Keyed by the table: the guests counted and the order picked are its own.
        <Picked key={props.table.id} {...props} table={props.table} />
      ) : (
        <EmptyState icon={<ArmchairIcon />} line="Pick a table to see its order." className="m-auto" />
      )}
    </aside>
  );
}

function Picked({
  table: t,
  title,
  now,
  lateAfter,
  tickets,
  onSeat,
  onClear,
  onRelease,
  onAddItems,
  onSettle,
  onMode,
  sendWaits,
  onClose,
}: TablePanelProps & { table: LiveTableResponse }) {
  const [pickedOrder, setPickedOrder] = useState<string | null>(null);
  const [covers, setCovers] = useState(() => startingCovers(t.seats));
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const view = panelFor(t, pickedOrder);
  const time = t.since ? elapsed(t.since, now, t.status === "dirty" ? 0 : lateAfter) : null;
  const guests = guestsText(t.covers) ?? `${t.seats} ${t.seats === 1 ? "seat" : "seats"}`;

  const run = (act: () => Promise<void>) => {
    setBusy(true);
    setFailure(null);
    act()
      .catch((err: unknown) => setFailure(wordsOf(err)))
      .finally(() => setBusy(false));
  };

  return (
    <>
      <header className="border-b border-line px-5 py-5 short:py-4">
        <div className="flex items-start gap-3">
          <h2 className="min-w-0 flex-1 truncate font-serif text-total leading-tight">{title}</h2>
          {/* Over the floor, the panel hides tables: this gives them back. Beside it, there is nothing to uncover. */}
          <button
            type="button"
            onClick={onClose}
            // Not "Close the table": that would sound like settling it.
            aria-label="Show the whole floor"
            className="flex size-tap shrink-0 items-center justify-center rounded-round border border-line-strong bg-raised outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3 xl:hidden"
          >
            <XIcon aria-hidden="true" size={20} />
          </button>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2 short:mt-2">
          <StatusChip status={{ kind: "table", status: t.status, paid: tablePaid(t) }} />
          {time && (
            <span
              className={cx(
                "inline-flex min-h-8 items-center gap-1.5 rounded-pill px-3 font-mono text-sm font-semibold tabular-nums",
                time.late ? "bg-attention-tint text-attention-text" : "bg-sunken text-fg",
              )}
            >
              <TimerIcon aria-hidden="true" size={14} />
              {time.text}
            </span>
          )}
        </div>
        <p className="mt-3.5 flex items-center gap-2 text-base text-fg short:mt-2">
          <UsersIcon aria-hidden="true" size={17} />
          {guests}
        </p>
        {view.kind === "order" && t.orders.length > 1 && (
          <button
            type="button"
            onClick={() => setPickedOrder(null)}
            className="mt-2 inline-flex min-h-tap items-center gap-1.5 rounded-pill text-base font-semibold text-attention-text outline-offset-2 outline-attention focus-visible:outline-3"
          >
            <ChevronLeftIcon aria-hidden="true" size={16} />
            All {t.orders.length} orders at this table
          </button>
        )}
      </header>

      {view.kind === "order" && (
        <TableOrderView
          key={view.orderId}
          table={t}
          orderId={view.orderId}
          onAddItems={onAddItems}
          onSettle={onSettle}
          onMode={onMode}
          onRelease={() => onRelease(t)}
          sendWaits={sendWaits(view.orderId)}
        />
      )}

      {view.kind === "seat" && (
        <Body
          failure={failure}
          footer={
            // Seat and order is the primary, as on the waiter's tablet; a host
            // who seats and leaves the order for later has the other (TBS-19).
            <div className="flex flex-col gap-3">
              <Button variant="primary" size="charge" className="w-full" disabled={busy} onClick={() => run(() => onSeat(t, covers, true))}>
                {busy ? "Seating" : "Seat and order"}
                <ArrowRightIcon aria-hidden="true" size={17} />
              </Button>
              <Button className="w-full" disabled={busy} onClick={() => run(() => onSeat(t, covers, false))}>
                {`Seat ${covers} ${covers === 1 ? "guest" : "guests"}`}
              </Button>
            </div>
          }
        >
          <p className="text-base text-fg-2">This table is free.</p>
          <div className="mt-5 flex items-center justify-between gap-4">
            <span className="text-base font-semibold">Guests</span>
            <Qty value={covers} onChange={setCovers} label="Guests" bounds={{ min: 1, max: 99 }} />
          </div>
        </Body>
      )}

      {view.kind === "clear" && (
        <Body
          failure={failure}
          footer={
            <Button variant="primary" size="charge" className="w-full" disabled={busy} onClick={() => run(() => onClear(t))}>
              {busy ? "Clearing" : "Clear table"}
            </Button>
          }
        >
          <p className="text-base text-fg-2">The guests have paid and gone. Clear it once it is laid again, and it is free.</p>
        </Body>
      )}

      {view.kind === "choose" && (
        <Body failure={failure}>
          <p className="text-base text-fg-2">{view.orders.length} orders are open at this table. Pick one.</p>
          <ul className="mt-4 flex flex-col gap-tap-gap">
            {view.orders.map((id) => {
              const o = tickets.find((s) => s.id === id);
              const opened = o ? elapsed(o.opened_at, now, lateAfter) : null;
              return (
                <li key={id}>
                  <button
                    type="button"
                    onClick={() => setPickedOrder(id)}
                    className="grid min-h-tap w-full grid-cols-[1fr_auto] items-center gap-3 rounded border border-line-strong bg-raised px-4 py-3 text-left outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3"
                  >
                    <span className="flex min-w-0 flex-col gap-1">
                      <span className="truncate font-mono text-base font-semibold">{id}</span>
                      {o && (
                        <span className="text-sm text-fg-muted">
                          {o.lines === 0 ? "Nothing ordered yet" : `${o.lines} ${o.lines === 1 ? "item" : "items"}`}
                        </span>
                      )}
                    </span>
                    {o && opened && (
                      <span className="flex flex-col items-end gap-1">
                        <Money amount={o.total} />
                        <span className={cx("font-mono text-sm tabular-nums", opened.late ? "text-attention-text" : "text-fg-muted")}>{opened.text}</span>
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        </Body>
      )}

      {view.kind === "nothing" && (
        <Body failure={failure}>
          <p className="text-base text-fg-2">Nothing is open at this table.</p>
        </Body>
      )}
    </>
  );
}

function Body({ failure, footer, children }: { failure: string | null; footer?: ReactNode; children: ReactNode }) {
  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5 short:py-4">
        {children}
        {failure && (
          <p role="alert" className="mt-4 text-base text-danger">
            {failure}
          </p>
        )}
      </div>
      {footer && <footer className="border-t border-line px-5 pb-5 pt-4 short:pb-4 short:pt-3">{footer}</footer>}
    </>
  );
}
