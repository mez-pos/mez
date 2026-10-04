// The order's lines (D03): a diet mark and the name, what it comes to, its
// choices and notes, a stepper for how many and a bin. The lines scroll; the
// totals under them do not. Every figure is the server's projection, never
// arithmetic done here. A voided line stays, struck through: nothing is hidden.
//
// A line the kitchen has not seen can be stepped and removed here. One it has
// seen cannot change quantity: its bin asks a manager to void it (ADR-072).

import { Trash2Icon } from "lucide-react";
import type { Diet, LineResponse, Money as Amount, NoteResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Money, Qty, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { humanise } from "../../../shared/ui/timeline-entries";
import { lineActions } from "./cart-logic";
import { DietMark } from "./ItemGrid";
import type { PendingLine } from "./ticket-queue";

interface RailLinesProps {
  order: OrderResponse | undefined;
  pending: readonly PendingLine[];
  /** Lines whose ticket is on the pass (ADR-090). */
  ready: ReadonlySet<number>;
  dietFor: (line: LineResponse) => Diet | undefined;
  onOpenLine: (line: LineResponse) => void;
  onVoidLine: (line: LineResponse) => void;
  onRemoveLine: (line: LineResponse) => void;
  onQty: (line: LineResponse, next: number) => void;
  onRetry: (key: string) => void;
  onDrop: (key: string) => void;
  onNote: () => void;
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });
const modifierText = (mods: readonly { name: string }[] | undefined): string => (mods ?? []).map((m) => m.name).join(", ");

export function RailLines(props: RailLinesProps) {
  const { order, pending, ready, dietFor, onOpenLine, onVoidLine, onRemoveLine, onQty, onRetry, onDrop, onNote } = props;
  const empty = (order?.lines.length ?? 0) === 0 && pending.length === 0;
  const orderNotes = order?.notes.filter((n) => n.scope === "order") ?? [];
  const paid = order?.tenders.some((t) => t.status !== "voided") ?? false;

  return (
    <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
      {empty ? (
        <div className="py-12 text-center">
          <p className="font-serif text-xl text-fg-muted">No items yet</p>
          <p className="mt-1.5 text-sm text-fg-muted">Tap a dish to start the order</p>
        </div>
      ) : (
        <ol>
          {order?.lines.map((line) => (
            <OrderLine
              key={line.id}
              line={line}
              diet={dietFor(line)}
              notes={order.notes}
              ready={ready.has(line.id)}
              paid={paid}
              onOpen={() => onOpenLine(line)}
              onVoid={() => onVoidLine(line)}
              onRemove={() => onRemoveLine(line)}
              onQty={(next) => onQty(line, next)}
            />
          ))}
          {pending.map((p) => (
            <PendingRow key={p.key} line={p} onRetry={onRetry} onDrop={onDrop} />
          ))}
        </ol>
      )}
      {orderNotes.map((n) => (
        <p key={n.seq} className="mt-3 rounded border border-line bg-sunken p-3 text-sm text-fg-2">
          Note: {n.text}
        </p>
      ))}
      <button
        type="button"
        disabled={!order}
        onClick={onNote}
        className="mt-3 flex min-h-tap w-full items-center rounded bg-raised-high px-4 text-base text-fg-muted outline-offset-2 outline-line-focus hover:bg-sunken focus-visible:outline-2 disabled:opacity-40"
      >
        Add a note to this order
      </button>
    </div>
  );
}

interface OrderLineProps {
  line: LineResponse;
  diet: Diet | undefined;
  notes: readonly NoteResponse[];
  ready: boolean;
  paid: boolean;
  onOpen: () => void;
  onVoid: () => void;
  onRemove: () => void;
  onQty: (next: number) => void;
}

function OrderLine({ line, diet, notes, ready, paid, onOpen, onVoid, onRemove, onQty }: OrderLineProps) {
  const can = lineActions(line, paid);
  const voided = line.status === "voided";
  const mine = notes.filter((n) => n.scope === "line" && n.line_id === line.id);
  const bin = can.remove ? { label: `Remove ${line.name}`, act: onRemove } : can.void ? { label: `Void ${line.name}`, act: onVoid } : null;

  return (
    <li className="flex gap-3 border-b border-line-soft py-3.5">
      <button
        type="button"
        onClick={onOpen}
        aria-label={`${line.qty} ${line.name}${line.status === "active" ? "" : `, ${line.status}`}`}
        className="min-w-0 flex-1 rounded text-left outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
      >
        <span className={cx("flex items-start gap-2", voided && "line-through opacity-60")}>
          <DietMark diet={diet} />
          <span className="text-base font-bold">{line.name}</span>
        </span>
        <Money
          amount={line.gross}
          tone={line.status === "active" ? "default" : "muted"}
          className={cx("mt-0.5 block text-left font-semibold text-fg-heading", voided && "line-through")}
        />
        {ready && !voided && <StatusChip status={{ kind: "kitchen", status: "ready" }} className="mt-1.5" />}
        {line.status !== "active" && (
          <span className="mt-1.5 flex items-center gap-2">
            <StatusChip status={{ kind: "line", status: line.status }} />
            {line.reason_code && <span className="text-sm text-fg-muted">{humanise(line.reason_code)}</span>}
          </span>
        )}
        {line.modifiers && line.modifiers.length > 0 && (
          <span className={cx("mt-1 block text-sm text-fg-muted", voided && "line-through")}>{modifierText(line.modifiers)}</span>
        )}
        {mine.map((n) => (
          <span key={n.seq} className="mt-1 block text-sm text-fg-2">
            Note: {n.text}
          </span>
        ))}
        {line.discount.minor > 0 && (
          <span className="mt-1 flex gap-2 text-sm text-fg-muted">
            Discount <Money amount={negate(line.discount)} />
          </span>
        )}
      </button>
      <div className="flex shrink-0 flex-col items-end gap-1">
        {can.change ? (
          <Qty value={line.qty} onChange={onQty} label={line.name} />
        ) : (
          <span className="font-mono text-base font-semibold tabular-nums text-fg-2">×{line.qty}</span>
        )}
        {bin && (
          <button
            type="button"
            aria-label={bin.label}
            onClick={bin.act}
            className="flex size-tap items-center justify-center rounded-round text-danger outline-offset-2 outline-line-focus hover:bg-danger-tint focus-visible:outline-2"
          >
            <Trash2Icon aria-hidden="true" size={18} />
          </button>
        )}
      </div>
    </li>
  );
}

interface PendingRowProps {
  line: PendingLine;
  onRetry: (key: string) => void;
  onDrop: (key: string) => void;
}

function PendingRow({ line, onRetry, onDrop }: PendingRowProps) {
  const failed = line.status === "failed";
  const { command } = line;
  return (
    <li className="border-b border-line-soft py-3.5" aria-busy={!failed}>
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
            <Button variant="ghost" onClick={() => onDrop(line.key)}>
              Remove
            </Button>
          </div>
        </>
      )}
    </li>
  );
}
