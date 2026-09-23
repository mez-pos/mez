// The ticket on the right. The lines scroll; the totals block does not, so
// the total is on screen whatever the ticket's length (terminal-register.md).
// Every figure is the server's projection, never arithmetic done here.

import type { LineResponse, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Button, Meta, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import type { PendingLine } from "./ticket-queue";

interface TicketRailProps {
  order: OrderResponse | undefined;
  orderId: string | null;
  pending: readonly PendingLine[];
  currency: string;
  /** Whether a line can still have its modifiers changed. */
  canModify: (line: LineResponse) => boolean;
  onModify: (line: LineResponse) => void;
  onRetry: (key: string) => void;
  onRemove: (key: string) => void;
}

export function TicketRail({ order, orderId, pending, currency, canModify, onModify, onRetry, onRemove }: TicketRailProps) {
  const zero: Amount = { minor: 0, currency };
  const totals = order?.totals;
  const empty = (order?.lines.length ?? 0) === 0 && pending.length === 0;

  return (
    <aside aria-label="Ticket" className="flex w-104 shrink-0 flex-col border-l border-line bg-chrome">
      <header className="border-b border-line px-5 py-4">
        <Meta items={["Ticket", "Counter"]} />
        {/* Not in the trail: it uppercases, and an order id is read back as written. */}
        <p className="mt-1 font-mono text-base text-fg-2">{orderId ?? "New ticket"}</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5">
        {empty ? (
          <div className="py-16 text-center">
            <p className="font-serif text-xl italic text-fg-faint">No items yet</p>
            <p className="mt-1.5 text-sm text-fg-faint">Tap the menu to start a ticket</p>
          </div>
        ) : (
          <ol>
            {order?.lines.map((line) => (
              <OrderLine key={line.id} line={line} onModify={canModify(line) ? () => onModify(line) : undefined} />
            ))}
            {pending.map((p) => (
              <PendingRow key={p.key} line={p} onRetry={onRetry} onRemove={onRemove} />
            ))}
          </ol>
        )}
      </div>

      <footer className="border-t border-line px-5 pb-5 pt-4">
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
  /** Present while the kitchen has not seen the line and it has choices. */
  onModify?: (() => void) | undefined;
}

function OrderLine({ line, onModify }: OrderLineProps) {
  const off = line.status !== "active";
  return (
    <li className="grid grid-cols-[1fr_auto] gap-3 border-b border-line py-3.5">
      <div className={cx("min-w-0", off && "line-through opacity-60")}>
        <div className="flex items-baseline gap-2">
          <span className="font-mono text-sm tabular-nums text-fg-3">{line.qty}×</span>
          <span className="text-base font-medium">{line.name}</span>
        </div>
        {line.modifiers && line.modifiers.length > 0 && (
          <p className="mt-1 text-sm text-fg-muted">{modifierText(line.modifiers)}</p>
        )}
        {onModify && (
          <Button variant="ghost" className="-ml-4 mt-1" onClick={onModify} aria-label={`Modify ${line.name}`}>
            Modify
          </Button>
        )}
      </div>
      <Money amount={line.gross} tone={off ? "muted" : "default"} />
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
