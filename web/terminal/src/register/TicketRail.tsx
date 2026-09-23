// The ticket on the right. The lines scroll; the totals block does not, so
// the total is on screen whatever the ticket's length (terminal-register.md).
// Every figure is the server's projection, never arithmetic done here. A
// voided line stays on the ticket, struck through; nothing is ever hidden.

import type { DiscountResponse, LineResponse, Money as Amount, NoteResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Meta, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { humanise } from "../../../shared/ui/timeline-entries";
import type { PendingLine } from "./ticket-queue";

interface TicketRailProps {
  order: OrderResponse | undefined;
  orderId: string | null;
  pending: readonly PendingLine[];
  currency: string;
  onOpenLine: (line: LineResponse) => void;
  onOrderDiscount: () => void;
  onOrderNote: () => void;
  onRetry: (key: string) => void;
  onRemove: (key: string) => void;
}

export function TicketRail(props: TicketRailProps) {
  const { order, orderId, pending, currency, onOpenLine, onOrderDiscount, onOrderNote, onRetry, onRemove } = props;
  const zero: Amount = { minor: 0, currency };
  const totals = order?.totals;
  const empty = (order?.lines.length ?? 0) === 0 && pending.length === 0;
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const orderNotes = order?.notes.filter((n) => n.scope === "order") ?? [];
  const orderDiscounts = order?.discounts.filter((d) => d.scope === "order") ?? [];

  return (
    <aside aria-label="Ticket" className="flex w-104 shrink-0 flex-col border-l border-line bg-chrome">
      <header className="border-b border-line px-5 py-4">
        <Meta items={["Ticket", "Counter"]} />
        {/* Not in the trail: it uppercases, and an order id is read back as written. */}
        <p className="mt-1 font-mono text-base text-fg-2">{orderId ?? "New ticket"}</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5">
        {orderNotes.map((n) => (
          <p key={n.seq} className="mt-3 rounded border border-line bg-raised p-3 text-sm text-fg-2">
            Note: {n.text}
          </p>
        ))}
        {empty ? (
          <div className="py-16 text-center">
            <p className="font-serif text-xl italic text-fg-faint">No items yet</p>
            <p className="mt-1.5 text-sm text-fg-faint">Tap the menu to start a ticket</p>
          </div>
        ) : (
          <ol>
            {order?.lines.map((line) => (
              <OrderLine key={line.id} line={line} notes={order.notes} onOpen={() => onOpenLine(line)} />
            ))}
            {pending.map((p) => (
              <PendingRow key={p.key} line={p} onRetry={onRetry} onRemove={onRemove} />
            ))}
          </ol>
        )}
      </div>

      <footer className="border-t border-line px-5 pb-5 pt-4">
        <div className="mb-3 grid grid-cols-2 gap-tap-gap">
          <Button disabled={!live} onClick={onOrderDiscount}>
            Discount
          </Button>
          <Button disabled={!order} onClick={onOrderNote}>
            Note
          </Button>
        </div>
        {totals && (totals.line_discounts.minor > 0 || totals.order_discounts.minor > 0) && (
          <TotalRow label="Items" amount={totals.gross} />
        )}
        {totals && totals.line_discounts.minor > 0 && <TotalRow label="Line discounts" amount={negate(totals.line_discounts)} />}
        {orderDiscounts.map((d) => (
          <TotalRow key={d.seq} label={`Discount · ${discountLabel(d)}`} amount={negate(d.value)} />
        ))}
        <TotalRow label="Subtotal" amount={totals?.subtotal ?? zero} />
        <TotalRow label="Tax" amount={totals?.tax ?? zero} />
        <div className="mt-3 flex items-baseline justify-between border-t border-line-strong pt-3">
          <span className="font-serif text-xl">Total</span>
          <Money amount={totals?.total ?? zero} variant="total" tone="accent" />
        </div>
      </footer>
    </aside>
  );
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });

function discountLabel(d: DiscountResponse): string {
  return humanise(d.reason) ?? d.reason;
}

function TotalRow({ label, amount }: { label: string; amount: Amount }) {
  return (
    <div className="flex justify-between py-0.5 text-sm text-fg-muted">
      <span>{label}</span>
      <Money amount={amount} />
    </div>
  );
}

function modifierText(mods: readonly { name: string }[] | undefined): string {
  return (mods ?? []).map((m) => m.name).join(", ");
}

interface OrderLineProps {
  line: LineResponse;
  notes: readonly NoteResponse[];
  onOpen: () => void;
}

function OrderLine({ line, notes, onOpen }: OrderLineProps) {
  const voided = line.status === "voided";
  const mine = notes.filter((n) => n.scope === "line" && n.line_id === line.id);
  return (
    <li className="border-b border-line">
      <button
        type="button"
        onClick={onOpen}
        aria-label={`${line.qty} ${line.name}${line.status === "active" ? "" : `, ${line.status}`}`}
        className="grid min-h-tap w-full grid-cols-[1fr_auto] gap-3 py-3.5 text-left outline-offset-2 outline-line-focus hover:bg-raised focus-visible:outline-2"
      >
        <div className="min-w-0">
          <div className={cx("flex items-baseline gap-2", voided && "line-through opacity-60")}>
            <span className="font-mono text-sm tabular-nums text-fg-3">{line.qty}×</span>
            <span className="text-base font-medium">{line.name}</span>
          </div>
          {line.status !== "active" && (
            <div className="mt-1.5 flex items-center gap-2">
              <StatusChip status={{ kind: "line", status: line.status }} />
              {line.reason_code && <span className="text-sm text-fg-muted">{humanise(line.reason_code)}</span>}
            </div>
          )}
          {line.modifiers && line.modifiers.length > 0 && (
            <p className={cx("mt-1 text-sm text-fg-muted", voided && "line-through")}>{modifierText(line.modifiers)}</p>
          )}
          {mine.map((n) => (
            <p key={n.seq} className="mt-1 text-sm text-fg-2">
              Note: {n.text}
            </p>
          ))}
          {line.discount.minor > 0 && (
            <p className="mt-1 flex gap-2 text-sm text-fg-muted">
              Discount <Money amount={negate(line.discount)} />
            </p>
          )}
        </div>
        <Money amount={line.gross} tone={line.status === "active" ? "default" : "muted"} className={cx(voided && "line-through")} />
      </button>
    </li>
  );
}

interface PendingRowProps {
  line: PendingLine;
  onRetry: (key: string) => void;
  onRemove: (key: string) => void;
}

function PendingRow({ line, onRetry, onRemove }: PendingRowProps) {
  const failed = line.status === "failed";
  const { command } = line;
  return (
    <li className="border-b border-line py-3.5" aria-busy={!failed}>
      <div className={cx("flex items-baseline gap-2", !failed && "text-fg-faint")}>
        <span className="font-mono text-sm tabular-nums">{command.qty}×</span>
        <span className="text-base font-medium">{command.name}</span>
        <span className="ml-auto text-meta uppercase">{failed ? "Not added" : "Adding"}</span>
      </div>
      {command.modifiers.length > 0 && <p className="mt-1 text-sm text-fg-faint">{modifierText(command.modifiers)}</p>}
      {failed && (
        <>
          <p className="mt-2 text-sm text-danger">{line.error}</p>
          <div className="mt-2 flex gap-tap-gap">
            <Button onClick={() => onRetry(line.key)}>Retry</Button>
            <Button variant="ghost" onClick={() => onRemove(line.key)}>
              Remove
            </Button>
          </div>
        </>
      )}
    </li>
  );
}
