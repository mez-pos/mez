// Guests' requests (docs/design/screens/terminal-requests.md): each order a
// phone placed at a table nobody has seated, oldest first, with what it asks
// for and how long it has waited, and Accept or Decline.

import { useEffect, useState } from "react";
import type { GuestRequestResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money, Sheet } from "../../../shared/ui";
import { choicesOf, dishCount, waited } from "./request-logic";
import type { GuestRequests } from "./use-guest-requests";

interface RequestsSheetProps {
  guests: GuestRequests;
  onClose: () => void;
}

export function RequestsSheet({ guests, onClose }: RequestsSheetProps) {
  // "4 min" moves on while the sheet is open.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 15_000);
    return () => clearInterval(t);
  }, []);

  return (
    <Sheet open onClose={onClose} title="Table requests">
      <div className="px-6 py-4">
        {!guests.loaded && !guests.error && <p className="text-base text-fg-muted">Loading the requests</p>}
        {guests.error && (
          <p role="alert" className="text-base text-danger">
            {guests.error}
          </p>
        )}
        {guests.loaded && guests.requests.length === 0 && <EmptyState icon={<InboxIcon />} line="No table is waiting to order from a phone." />}
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
