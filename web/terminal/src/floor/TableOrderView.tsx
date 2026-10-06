// The order at the table picked (D05): its lines and what it comes to, which
// scroll together, and under them Add Items, Send to Kitchen, More Actions and Settle Bill. The
// lines are read here and changed on the order screen, which is where Add Items
// goes. The design's "Guests" tab needs guests the order does not keep; Notes
// is drawn because notes are kept.

import { ArrowRightIcon, PlusIcon, SendIcon } from "lucide-react";
import { useState } from "react";
import type { LineResponse, LiveTableResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { HistorySheet } from "../register/HistorySheet";
import { failureOf, wordsOf } from "../register/order-messages";
import { OrderTotals } from "../register/OrderTotals";
import { amountOwed } from "../register/rail-logic";
import { readyLines, sendLabel, sendPlan } from "../register/send-logic";
import { chargeBlock, chargeLabel } from "../tender/tender-logic";
import { TableActions } from "./TableActions";
import { useTableOrder } from "./use-table-order";

export type PartyMode = "move" | "join" | "part";

interface TableOrderViewProps {
  table: LiveTableResponse;
  orderId: string;
  /** Brings the order to the front on the order screen, off hold if it was held. */
  onAddItems: (orderId: string, held: boolean) => Promise<void>;
  onSettle: (orderId: string) => void;
  onMode: (kind: PartyMode) => void;
  /** Lets go guests who ordered nothing. */
  onRelease: () => Promise<void>;
  /**
   * Items rung up on this till are still on their way to this order: a Send
   * from here would overtake them, and the kitchen would miss them.
   */
  sendWaits: boolean;
}

type Tab = "order" | "notes";

export function TableOrderView({ table, orderId, onAddItems, onSettle, onMode, onRelease, sendWaits }: TableOrderViewProps) {
  const { order, loading, error, refetch, send, fire } = useTableOrder(orderId);
  const [tab, setTab] = useState<Tab>("order");
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);

  if (!order) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 px-6 text-center">
        <p className="text-base text-fg-muted">{loading ? "Opening the order" : failureOf(error)}</p>
        {!loading && <Button onClick={refetch}>Try again</Button>}
      </div>
    );
  }

  const run = (act: () => Promise<void>) => {
    setBusy(true);
    setFailure(null);
    act()
      .catch((err: unknown) => setFailure(wordsOf(err)))
      .finally(() => setBusy(false));
  };
  const plan = sendPlan(order);
  const held = order.status === "held";
  const block = chargeBlock(order, 0);
  // "Settle Bill" while money is owed or nothing can be done; a paid order still
  // open says what the button finishes, as the order screen's Pay does.
  const finishing = block === null && chargeLabel(order) !== "Pay" ? chargeLabel(order) : null;
  const tabs: readonly { id: Tab; label: string }[] = [
    { id: "order", label: "Current Order" },
    { id: "notes", label: order.notes.length > 0 ? `Notes (${order.notes.length})` : "Notes" },
  ];

  return (
    <>
      <div role="tablist" aria-label="Order" className="flex gap-6 border-b border-line px-5">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`table-tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls="table-tabpanel"
            // One tab stop, and the arrows move between the two (the ARIA tabs pattern).
            tabIndex={tab === t.id ? 0 : -1}
            onKeyDown={(e) => {
              if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
              const other: Tab = t.id === "order" ? "notes" : "order";
              setTab(other);
              document.getElementById(`table-tab-${other}`)?.focus();
            }}
            onClick={() => setTab(t.id)}
            className={cx(
              "min-h-tap border-b-3 text-base outline-offset-2 outline-attention focus-visible:outline-3",
              tab === t.id ? "border-attention font-bold text-attention-text" : "border-transparent text-fg-muted hover:text-fg",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div id="table-tabpanel" role="tabpanel" aria-labelledby={`table-tab-${tab}`} className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
        {held && (
          <p role="status" className="mb-2 flex items-center gap-2 text-sm text-fg-2">
            <StatusChip status={{ kind: "order", status: "held" }} /> Saved. Add Items brings it back.
          </p>
        )}
        {tab === "order" ? <Lines order={order} /> : <Notes order={order} />}
        {tab === "order" && order.lines.length > 0 && (
          <div className="mt-4 pb-2">
            <OrderTotals order={order} currency={order.currency} />
          </div>
        )}
      </div>

      <footer className="border-t border-line px-4 pb-4 pt-3 2xl:px-5 2xl:pb-5 2xl:pt-4">
        {failure && (
          <p role="alert" className="mb-3 text-base text-danger">
            {failure}
          </p>
        )}
        <div className="flex gap-3">
          <Button className="flex-1 px-3" disabled={busy || order.status === "closed"} onClick={() => run(() => onAddItems(order.id, held))}>
            <PlusIcon aria-hidden="true" size={17} />
            Add Items
          </Button>
          <Button className="flex-1 px-3" disabled={busy || held || sendWaits || plan.waiting === 0} onClick={() => run(send)}>
            <SendIcon aria-hidden="true" size={17} />
            {sendLabel(plan)}
          </Button>
        </div>
        <div className="mt-3 flex gap-3">
          <TableActions
            joined={(order.joined?.length ?? 0) > 0}
            nextCourse={!held && plan.next ? plan.next.course : null}
            canRelease={table.status === "seated"}
            disabled={busy}
            onMove={() => onMode("move")}
            onJoin={() => onMode("join")}
            onPart={() => onMode("part")}
            onFire={(course) => run(() => fire(course))}
            onHistory={() => setHistoryOpen(true)}
            onRelease={() => run(onRelease)}
          />
          <Button
            variant="primary"
            size="charge"
            className="min-w-0 flex-1 px-4"
            disabled={busy || block !== null}
            aria-describedby={block ? "settle-block" : undefined}
            onClick={() => onSettle(order.id)}
          >
            {finishing ?? "Settle Bill"}
            {finishing === null && order.lines.length > 0 && <Money amount={amountOwed(order.totals, order.currency)} className="text-base 2xl:text-lg" />}
            <ArrowRightIcon aria-hidden="true" size={19} className="shrink-0" />
          </Button>
        </div>
        {block && (
          <p id="settle-block" className="mt-2 text-sm text-fg-muted">
            {block}
          </p>
        )}
      </footer>

      {historyOpen && <HistorySheet orderId={order.id} onClose={() => setHistoryOpen(false)} />}
    </>
  );
}

const modifierText = (line: LineResponse): string => (line.modifiers ?? []).map((m) => m.name).join(", ");

function Lines({ order }: { order: OrderResponse }) {
  const ready = readyLines(order);
  if (order.lines.length === 0) {
    return (
      <div className="py-12 text-center">
        <p className="font-serif text-xl text-fg-muted">No items yet</p>
        <p className="mt-1.5 text-sm text-fg-muted">Add Items to start the order</p>
      </div>
    );
  }
  return (
    <ol>
      {order.lines.map((line) => {
        const voided = line.status === "voided";
        const choices = modifierText(line);
        return (
          <li key={line.id} className="flex items-start gap-3 border-b border-line-soft py-3">
            <span className="min-w-5 font-mono text-base font-bold tabular-nums text-fg-muted">{line.qty}</span>
            <span className="min-w-0 flex-1">
              <span className={cx("block text-base font-bold", voided && "line-through opacity-60")}>{line.name}</span>
              {choices && <span className={cx("block text-sm text-fg-muted", voided && "line-through")}>{choices}</span>}
              {ready.has(line.id) && !voided && <StatusChip status={{ kind: "kitchen", status: "ready" }} className="mt-1.5" />}
              {line.status !== "active" && <StatusChip status={{ kind: "line", status: line.status }} className="mt-1.5" />}
            </span>
            <Money
              amount={line.gross}
              tone={line.status === "active" ? "default" : "muted"}
              className={cx("font-semibold", line.status === "active" && "text-fg-heading", voided && "line-through")}
            />
          </li>
        );
      })}
    </ol>
  );
}

function Notes({ order }: { order: OrderResponse }) {
  if (order.notes.length === 0) return <p className="py-12 text-center text-base text-fg-muted">No notes on this order.</p>;
  const lineName = (id: number | undefined) => order.lines.find((l) => l.id === id)?.name;
  return (
    <ul className="flex flex-col gap-2 py-1">
      {order.notes.map((n) => (
        <li key={n.seq} className="rounded border border-line bg-sunken p-3">
          <span className="block text-meta uppercase text-fg-faint">{n.scope === "line" ? (lineName(n.line_id) ?? "An item") : "The order"}</span>
          <span className="mt-1 block text-base text-fg">{n.text}</span>
        </li>
      ))}
    </ul>
  );
}
