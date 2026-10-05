// Confirm and send (T04): what one tap of Send will put in front of the kitchen,
// read before it goes. A banner says how many items go, how many the kitchen has
// already and how many wait for their course; under it the ticket as the kitchen
// will read it. Back changes nothing; "Send N items" sends.
//
// Left out: the choice of kitchen printer or screen (each item's station decides
// that), "Notify me when items are ready" and Undo (nothing does either), and the
// ticket's number, which the server gives it when it is sent.

import { ChefHatIcon, SendIcon } from "lucide-react";
import { useState } from "react";
import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { Button, Modal, useToast } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { useVenue } from "../../../shared/use-venue";
import { clockText } from "../../../shared/venue-text";
import { wordsOf } from "./order-messages";
import { sendPreview } from "./send-logic";

interface ConfirmSendProps {
  order: OrderResponse;
  /** "Table T4", or how the order is served when it has no table. */
  title: string;
  /** "Dine-in", "Takeaway", "Delivery". */
  service: string;
  courseName: (line: LineResponse) => string | null;
  /** Sends; rejects with the server's refusal. */
  onSend: () => Promise<void>;
  onClose: () => void;
}

const items = (n: number) => `${n} ${n === 1 ? "item" : "items"}`;

export function ConfirmSend({ order, title, service, courseName, onSend, onClose }: ConfirmSendProps) {
  const auth = useAuth();
  const venue = useVenue();
  const toast = useToast();
  const now = useClock(30_000);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const preview = sendPreview(order);
  const count = preview.going.length;
  const orderNotes = order.notes.filter((n) => n.scope === "order");
  const [date = "", time = ""] = clockText(new Date(now), venue?.timezone ?? null).split(" · ");

  const send = () => {
    setBusy(true);
    setError(null);
    onSend()
      .then(() => {
        toast(`Order sent to kitchen · ${title}`);
        onClose();
      })
      .catch((err: unknown) => {
        setError(wordsOf(err));
        setBusy(false);
      });
  };

  return (
    <Modal
      open
      onClose={onClose}
      title="Confirm and send"
      size="lg"
      footer={
        <div className="flex w-full gap-3">
          <Button size="charge" className="w-40 shrink-0" onClick={onClose}>
            Back
          </Button>
          <Button variant="primary" size="charge" className="flex-1" disabled={busy || count === 0} onClick={send}>
            <SendIcon aria-hidden="true" size={20} />
            Send {items(count)} to kitchen
          </Button>
        </div>
      }
    >
      <div className="flex gap-3 rounded-card border border-chip-success-border bg-accent-tint p-4">
        <ChefHatIcon aria-hidden="true" size={22} className="shrink-0 text-chip-success-fg" />
        <div>
          <p className="text-base font-bold text-chip-success-fg">{count === 1 ? "1 new item" : `${count} new items`} will be sent to the kitchen</p>
          {preview.gone > 0 && (
            <p className="mt-1 text-sm text-fg">
              {items(preview.gone)} {preview.gone === 1 ? "is" : "are"} already with the kitchen and won't be sent again.
            </p>
          )}
          {preview.held > 0 && (
            <p className="mt-1 text-sm text-fg">
              {items(preview.held)} of a later course {preview.held === 1 ? "waits" : "wait"} until that course is fired.
            </p>
          )}
        </div>
      </div>

      <h3 className="mb-3 mt-5 font-serif text-xl">Kitchen ticket preview</h3>
      <div className="rounded-card border border-line bg-raised p-5 font-mono text-fg shadow-rest">
        <div className="border-b border-dashed border-line-strong pb-3.5 text-center">
          <p className="text-meta font-bold uppercase">Kitchen order ticket</p>
          {venue && <p className="mt-1.5 text-sm text-fg-muted">{venue.name}</p>}
        </div>
        <div className="flex justify-between gap-4 border-b border-dashed border-line-strong py-3.5 text-sm">
          <p>
            {title}
            {title === service ? "" : ` · ${service}`}
          </p>
          <p className="text-right tabular-nums">
            {date}
            <br />
            {time}
          </p>
        </div>
        <ul aria-label="Items to send" className="grid max-h-64 gap-3 overflow-y-auto border-b border-dashed border-line-strong py-3.5">
          {preview.going.map((line) => {
            const mods = (line.modifiers ?? []).map((m) => m.name).join(" · ");
            const notes = order.notes.filter((n) => n.scope === "line" && n.line_id === line.id);
            const course = courseName(line);
            return (
              <li key={line.id} className="flex gap-3.5 text-base">
                <span className="font-bold tabular-nums">{line.qty}×</span>
                <div className="min-w-0">
                  <p className="font-bold uppercase">{line.name}</p>
                  {mods && <p className="mt-0.5 text-sm text-fg-muted">{mods}</p>}
                  {notes.map((n) => (
                    <p key={n.seq} className="mt-0.5 text-sm text-fg-2">
                      {n.text}
                    </p>
                  ))}
                  {course && <p className="mt-0.5 text-sm font-semibold uppercase text-attention-text">{course}</p>}
                </div>
              </li>
            );
          })}
        </ul>
        <div className="pt-3.5 text-sm">
          {auth.kind === "signed_in" && <p className="uppercase">Waiter: {auth.staff.name}</p>}
          {order.covers > 0 && <p className="mt-1 uppercase">Guests: {order.covers}</p>}
          {orderNotes.map((n) => (
            <p key={n.seq} className="mt-2.5 rounded-tight bg-chip-warning-bg p-2.5 text-chip-warning-fg">
              NOTE: {n.text}
            </p>
          ))}
        </div>
      </div>

      {error && (
        <p role="alert" className="mt-4 text-base text-danger">
          {error}
        </p>
      )}
    </Modal>
  );
}
