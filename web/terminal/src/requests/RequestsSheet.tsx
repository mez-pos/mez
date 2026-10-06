// Guests' requests (docs/design/screens/terminal-requests.md): each order a
// phone placed at a table nobody has seated, oldest first, with what it asks
// for and how long it has waited, and Accept or Decline.

import { useEffect, useState } from "react";
import type { ChargeResolution, GuestRequestResponse, PaidNoticeResponse, UnappliedChargeResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money, Sheet } from "../../../shared/ui";
import { choicesOf, dishCount, gatewayName, methodWord, owesOtherwise, saidAgo, waited } from "./request-logic";
import type { GuestRequests } from "./use-guest-requests";

interface RequestsSheetProps {
  guests: GuestRequests;
  onClose: () => void;
  /** On to the payment screen of the ticket a notice is about. */
  onTakePayment: (orderId: string) => void;
}

export function RequestsSheet({ guests, onClose, onTakePayment }: RequestsSheetProps) {
  // "4 min" moves on while the sheet is open.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 15_000);
    return () => clearInterval(t);
  }, []);

  return (
    <Sheet open onClose={onClose} title="From the tables">
      <div className="px-6 py-4">
        {!guests.loaded && !guests.error && <p className="text-base text-fg-muted">Loading the requests</p>}
        {guests.error && (
          <p role="alert" className="text-base text-danger">
            {guests.error}
          </p>
        )}
        {guests.loaded && guests.requests.length === 0 && guests.paid.length === 0 && guests.unapplied.length === 0 && (
          <EmptyState icon={<InboxIcon />} line="No table is waiting to order from a phone, and none says it has paid." />
        )}
        {guests.unapplied.length > 0 && (
          <section aria-label="Paid online, and on no bill" className="mb-5 rounded-card border border-danger bg-danger-tint p-4">
            <h3 className="text-meta font-bold uppercase text-danger">Paid online, and on no bill</h3>
            <p className="mt-1 text-sm text-fg">
              A guest's money was taken and the bill could not take it. Refund it at the payment provider, or record it on a bill by hand. Then say which, here.
            </p>
            <ol>
              {guests.unapplied.map((charge) => (
                <UnappliedRow key={charge.id} charge={charge} now={now} guests={guests} />
              ))}
            </ol>
          </section>
        )}
        {guests.paid.length > 0 && (
          <section aria-label="Say they have paid">
            <h3 className="text-meta font-bold uppercase text-fg-faint">Say they have paid</h3>
            <p className="mt-1 text-sm text-fg-muted">Nothing is paid until you record it. Check the venue's phone for the payment first.</p>
            <ol>
              {guests.paid.map((notice) => (
                <PaidRow key={notice.id} notice={notice} now={now} guests={guests} onTakePayment={onTakePayment} />
              ))}
            </ol>
          </section>
        )}
        {guests.paid.length + guests.unapplied.length > 0 && guests.requests.length > 0 && (
          <h3 className="mt-5 text-meta font-bold uppercase text-fg-faint">Want to order</h3>
        )}
        <ol>
          {guests.requests.map((request) => (
            <RequestRow key={request.id} request={request} now={now} guests={guests} />
          ))}
        </ol>
      </div>
    </Sheet>
  );
}

function RequestRow({ request, now, guests }: { request: GuestRequestResponse; now: number; guests: GuestRequests }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Decline tells a guest no, so it is asked twice, on the button.
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(t);
  }, [armed]);

  const act = (fn: (id: string) => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    fn(request.id)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusy(false));
  };
  const dishes = dishCount(request);

  return (
    <li className="border-b border-line-soft py-4">
      <div className="flex items-baseline justify-between gap-3">
        <p className="font-serif text-xl text-fg-heading">Table {request.table_name}</p>
        <p className="text-sm text-fg-muted">
          Waiting <span className="font-mono tabular-nums">{waited(request.placed_at, now)}</span>
        </p>
      </div>
      <ul className="mt-2">
        {request.lines.map((line, i) => (
          <li key={i} className="flex items-start justify-between gap-3 py-1 text-base">
            <span className="min-w-0">
              <span className="font-mono tabular-nums">{line.qty}</span> × {line.name}
              {choicesOf(line) && <span className="block text-sm text-fg-muted">{choicesOf(line)}</span>}
            </span>
            <Money amount={line.total} className="shrink-0" />
          </li>
        ))}
      </ul>
      <div className="mt-2 flex items-baseline justify-between gap-3 text-base font-semibold">
        <span>
          {dishes} {dishes === 1 ? "dish" : "dishes"}, before tax
        </span>
        <Money amount={request.subtotal} />
      </div>
      <div className="mt-3 flex gap-tap-gap">
        <Button variant="primary" disabled={busy} onClick={() => act(guests.accept)}>
          Accept and send
        </Button>
        <Button variant="ghost" disabled={busy} onClick={() => (armed ? act(guests.decline) : setArmed(true))}>
          {armed ? "Tap again to decline" : "Decline"}
        </Button>
      </div>
      {error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {error}
        </p>
      )}
    </li>
  );
}

interface PaidRowProps {
  notice: PaidNoticeResponse;
  now: number;
  guests: GuestRequests;
  onTakePayment: (orderId: string) => void;
}

/**
 * A table whose guest says they have paid by UPI (ADR-218). It is a message:
 * Take payment leads to the ticket, where the payment is recorded as any
 * other, and No payment found puts the message away and changes nothing.
 */
function PaidRow({ notice, now, guests, onTakePayment }: PaidRowProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Putting it away tells nobody the guest was wrong, but it does stop the
  // terminal saying so: asked twice, on the button.
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(t);
  }, [armed]);

  const putAway = () => {
    setBusy(true);
    setError(null);
    guests
      .seen(notice.id)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusy(false));
  };

  return (
    <li className="border-b border-line-soft py-4">
      <div className="flex items-baseline justify-between gap-3">
        <p className="font-serif text-xl text-fg-heading">Table {notice.table_name}</p>
        <p className="font-mono text-sm tabular-nums text-fg-muted">{saidAgo(notice.told_at, now)}</p>
      </div>
      <div className="mt-2 flex items-baseline justify-between gap-3 text-base font-semibold">
        <span>Says paid by UPI</span>
        <Money amount={notice.amount} />
      </div>
      {owesOtherwise(notice) && (
        <div className="mt-1 flex items-baseline justify-between gap-3 text-base text-attention-text">
          <span>The table owes now</span>
          <Money amount={notice.due} />
        </div>
      )}
      <div className="mt-3 flex gap-tap-gap">
        <Button variant="primary" disabled={busy} onClick={() => onTakePayment(notice.order_id)}>
          Take payment
        </Button>
        <Button variant="ghost" disabled={busy} onClick={() => (armed ? putAway() : setArmed(true))}>
          {armed ? "Tap again: no payment" : "No payment found"}
        </Button>
      </div>
      {error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {error}
        </p>
      )}
    </li>
  );
}

/**
 * Money a payment gateway took that no bill could (ADR-223). Nothing here
 * moves it: the refund is made at the provider, and a payment kept is
 * recorded on a bill as any other. These two buttons only say which was
 * done, by whom, so it stops being in front of everyone.
 */
function UnappliedRow({ charge, now, guests }: { charge: UnappliedChargeResponse; now: number; guests: GuestRequests }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Either answer takes it off every terminal for good: asked twice.
  const [armed, setArmed] = useState<ChargeResolution | null>(null);
  useEffect(() => {
    if (armed === null) return;
    const t = setTimeout(() => setArmed(null), 4000);
    return () => clearTimeout(t);
  }, [armed]);

  const say = (how: ChargeResolution) => {
    if (armed !== how) {
      setArmed(how);
      return;
    }
    setBusy(true);
    setError(null);
    guests
      .resolve(charge.id, how)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusy(false));
  };

  return (
    <li className="border-b border-danger/30 py-3 last:border-b-0">
      <div className="flex items-baseline justify-between gap-3">
        <p className="font-serif text-xl text-fg-heading">Table {charge.table_name}</p>
        <p className="font-mono text-sm tabular-nums text-fg-2">{saidAgo(charge.at, now).replace(/^Said /, "")}</p>
      </div>
      <div className="mt-1 flex items-baseline justify-between gap-3 text-base font-semibold">
        <span>Paid by {methodWord(charge.method)}</span>
        <Money amount={charge.amount} />
      </div>
      <p className="mt-1 text-base text-fg">{charge.problem}</p>
      <p className="mt-1 text-sm text-fg-2">
        {gatewayName(charge.gateway)} payment <span className="font-mono break-all">{charge.reference || charge.gateway_id}</span>
      </p>
      <div className="mt-3 flex flex-wrap gap-tap-gap">
        <Button disabled={busy} onClick={() => say("refunded")}>
          {armed === "refunded" ? "Tap again: refunded" : "I refunded it"}
        </Button>
        <Button disabled={busy} onClick={() => say("recorded")}>
          {armed === "recorded" ? "Tap again: recorded" : "I recorded it on a bill"}
        </Button>
      </div>
      {error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {error}
        </p>
      )}
    </li>
  );
}
