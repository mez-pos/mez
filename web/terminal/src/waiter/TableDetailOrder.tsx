// The table's own screen with its order (T03), in three columns: the table and
// its notes, the items, and where the order stands with what to do next.
//
// Left out, as nothing feeds them before the guest channel (phase 11): Guest QR
// orders, the items' "Rohan" and "QR" tabs and chips (a line does not say who
// rang it up), and the bell. Also left out: Preferences, Split Bill and Print
// KOT (nothing does them from here), Mark as Served (ADR-090 has no such step),
// and the pencil on the table's name. Transfer and Merge open T06 (MoveTable).

import { ArrowLeftRightIcon, GitMergeIcon, InfoIcon, PlusIcon, ReceiptTextIcon, SendIcon, StickyNoteIcon, UsersIcon, UtensilsIcon } from "lucide-react";
import { useState } from "react";
import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useMenu } from "../../../shared/use-menu";
import { useVenue } from "../../../shared/use-venue";
import type { FloorAction } from "../floor/floor-logic";
import { toneClasses } from "../floor/tones";
import { useTableOrder } from "../floor/use-table-order";
import { ConfirmSend } from "../register/ConfirmSend";
import { HistorySheet } from "../register/HistorySheet";
import { NoteSheet } from "../register/NoteSheet";
import { failureOf, wordsOf } from "../register/order-messages";
import { OrderTotals } from "../register/OrderTotals";
import { courseNamer, guestsText, serviceLabels } from "../register/rail-logic";
import { sendPlan } from "../register/send-logic";
import { chargeBlock, chargeLabel } from "../tender/tender-logic";
import type { MoveMode } from "./move-logic";
import { MoveTable } from "./MoveTable";
import { OrderProgress } from "./OrderProgress";
import { startedText } from "./table-logic";
import { TableItems } from "./TableItems";
import { cardWords } from "./waiter-logic";

interface TableDetailOrderProps {
  table: LiveTableResponse;
  /** "Table T4 + T7". */
  title: string;
  orderId: string;
  sendWaits: boolean;
  /** Brings the order to the front on the order screen, off hold if it was held. */
  onAddItems: (held: boolean) => Promise<void>;
  onBill: () => void;
  /** The floor, to move the party or join tables to it (T06). */
  floor: LiveFloorResponse;
  act: (action: FloorAction) => Promise<void>;
  onMoving: (to: number | null) => void;
}

const tile =
  "flex min-h-charge items-center justify-center gap-2 rounded px-3 text-base font-semibold outline-offset-2 outline-focus focus-visible:outline-3 disabled:opacity-40";

export function TableDetailOrder({ table, title, orderId, sendWaits, onAddItems, onBill, floor, act, onMoving }: TableDetailOrderProps) {
  const { order, loading, error, refetch, send, fire, addNote } = useTableOrder(orderId);
  const menu = useMenu();
  const venue = useVenue();
  const now = useClock(30_000);
  const [sheet, setSheet] = useState<"note" | "history" | "send" | MoveMode | null>(null);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

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
  const courseName = courseNamer(menu.data?.courses ?? []);
  const group = cardWords(table);
  const tone = toneClasses(group.tone);
  const held = order.status === "held";
  const plan = sendPlan(order);
  const block = chargeBlock(order, 0);
  const billLabel = chargeLabel(order) === "Pay" ? "Bill and pay" : chargeLabel(order);
  const notes = order.notes.filter((n) => n.scope === "order");
  const started = startedText(table.since ?? order.opened_at, now, venue?.timezone ?? null);
  const guests = guestsText(order.covers);

  return (
    <div className="flex min-w-0 flex-1 gap-4 overflow-hidden px-5 py-4">
      <div className="flex w-74 shrink-0 flex-col gap-3">
        <section aria-label="Table" className="rounded-card bg-accent p-5 text-accent-fg">
          <h1 className="font-serif text-total leading-tight text-accent-fg">{title}</h1>
          <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-base">
            {guests && (
              <span className="inline-flex items-center gap-2">
                <UsersIcon aria-hidden="true" size={17} />
                {guests}
              </span>
            )}
            <span className="inline-flex items-center gap-2">
              <UtensilsIcon aria-hidden="true" size={17} />
              {serviceLabels[order.service]}
            </span>
          </p>
          {started && <p className="mt-2 font-mono text-sm tabular-nums opacity-80">{started}</p>}
        </section>

        <div className="flex gap-2.5">
          <span className={cx("flex min-h-tap flex-1 items-center justify-center gap-2 rounded-pill text-base font-bold", tone.fill, tone.ink)}>
            <span aria-hidden="true" className={cx("size-2.5 rounded-round", tone.dot)} />
            {held ? "Saved" : group.label}
          </span>
          {(order.joined?.length ?? 0) > 0 && (
            <span className="flex min-h-tap flex-1 items-center justify-center gap-2 rounded-pill border border-line-strong bg-raised text-base font-semibold">
              <GitMergeIcon aria-hidden="true" size={16} />
              Merged
            </span>
          )}
        </div>

        <section aria-label="Table notes" className="flex min-h-0 flex-1 flex-col rounded-card border border-line bg-raised p-4">
          <h2 className="border-b border-line pb-2.5 text-base font-bold text-attention-text">Table Notes ({notes.length})</h2>
          <ul className="mt-3 min-h-0 flex-1 space-y-3.5 overflow-y-auto">
            {notes.map((n) => (
              <li key={n.seq} className="flex gap-2.5">
                <span aria-hidden="true" className="flex size-8 shrink-0 items-center justify-center rounded-tight bg-sunken">
                  <StickyNoteIcon size={16} />
                </span>
                <p className="text-sm leading-snug text-fg">{n.text}</p>
              </li>
            ))}
            {notes.length === 0 && <li className="text-sm text-fg-muted">No notes for this table yet.</li>}
          </ul>
          <button
            type="button"
            onClick={() => setSheet("note")}
            className="mt-3 flex min-h-tap w-full items-center justify-center gap-2 rounded-pill border border-dashed border-line-strong text-base font-semibold text-fg-muted outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3"
          >
            <PlusIcon aria-hidden="true" size={16} />
            Add note
          </button>
          <p aria-hidden="true" className="mt-3 rounded bg-accent-tint p-3 text-center font-script text-xl leading-tight text-fg-2">
            Great Food,
            <br />
            Brighter Days ♡
          </p>
        </section>
      </div>

      <TableItems
        order={order}
        courseName={courseName}
        canFire={!held && !busy}
        onFire={(course) => run(() => fire(course))}
        addDisabled={busy || order.status === "closed"}
        onAddItem={() => run(() => onAddItems(held))}
      />

      <div className="flex w-75 shrink-0 flex-col gap-3 overflow-y-auto">
        <OrderProgress order={order} timezone={venue?.timezone ?? null} />

        <div className="grid grid-cols-2 gap-2.5">
          <button type="button" disabled={block !== null} onClick={onBill} className={cx(tile, "bg-attention-tint text-attention-text")}>
            <ReceiptTextIcon aria-hidden="true" size={17} />
            {billLabel}
          </button>
          <button type="button" onClick={() => setSheet("history")} className={cx(tile, "border border-line-strong bg-raised hover:bg-raised-high")}>
            <InfoIcon aria-hidden="true" size={17} />
            Details
          </button>
          <button type="button" onClick={() => setSheet("transfer")} className={cx(tile, "border border-line-strong bg-raised hover:bg-raised-high")}>
            <ArrowLeftRightIcon aria-hidden="true" size={17} />
            Transfer
          </button>
          <button type="button" onClick={() => setSheet("merge")} className={cx(tile, "border border-line-strong bg-raised hover:bg-raised-high")}>
            <GitMergeIcon aria-hidden="true" size={17} />
            Merge
          </button>
        </div>
        {block && order.lines.length > 0 && <p className="-mt-1 text-sm text-fg-muted">{block}</p>}

        <section aria-label="Totals" className="rounded-card border border-line bg-raised p-4">
          <OrderTotals order={order} currency={order.currency} />
        </section>

        {failure && (
          <p role="alert" className="text-base text-danger">
            {failure}
          </p>
        )}
        <Button
          variant="primary"
          size="charge"
          className="h-16 w-full shrink-0"
          disabled={busy || held || sendWaits || plan.waiting === 0}
          onClick={() => setSheet("send")}
        >
          <SendIcon aria-hidden="true" size={20} />
          {plan.waiting > 0 ? `Send to kitchen (${plan.waiting})` : "Sent"}
        </Button>
      </div>

      {sheet === "note" && <NoteSheet onSave={(text) => addNote({ scope: "order", text }).then(() => setSheet(null))} onClose={() => setSheet(null)} />}
      {sheet === "history" && <HistorySheet orderId={order.id} onClose={() => setSheet(null)} />}
      {(sheet === "transfer" || sheet === "merge") && (
        <MoveTable floor={floor} party={table} title={title} order={order} startIn={sheet} act={act} onMoving={onMoving} onClose={() => setSheet(null)} />
      )}
      {sheet === "send" && (
        <ConfirmSend
          order={order}
          title={title}
          service={serviceLabels[order.service]}
          courseName={(line) => courseName(line.course)}
          onSend={send}
          onClose={() => setSheet(null)}
        />
      )}
    </div>
  );
}
