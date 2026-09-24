// The order's history (Task 6): every event, oldest first, through the
// Timeline primitive: what happened, who did it and who approved it, on
// which till, and why a line was voided or comped. Read-only: the events
// are the record, and nothing here changes them.

import { useQuery } from "@tanstack/react-query";
import type { HistoryResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Sheet, Timeline } from "../../../shared/ui";
import { failureOf } from "./order-messages";

/** Under the order's own key, so an event on the order refreshes it too. */
export const historyKey = (orderId: string) => ["order", orderId, "events"] as const;

interface HistorySheetProps {
  orderId: string;
  onClose: () => void;
}

export function HistorySheet({ orderId, onClose }: HistorySheetProps) {
  const store = useAuthStore();
  const history = useQuery({
    queryKey: historyKey(orderId),
    queryFn: ({ signal }) =>
      store.api.get<HistoryResponse>(`/api/v1/orders/${encodeURIComponent(orderId)}/events`, { signal }).then(withNames),
    staleTime: Infinity,
  });

  return (
    <Sheet open onClose={onClose} title={`History · ${orderId}`}>
      {history.isPending && <p className="text-sm text-fg-muted">Loading the history</p>}
      {history.isError && (
        <div className="flex flex-col items-start gap-3">
          <p className="text-sm text-danger">{failureOf(history.error)}</p>
          <Button onClick={() => void history.refetch()}>Try again</Button>
        </div>
      )}
      {history.data && (
        <Timeline
          events={history.data.events}
          staffName={(id) => history.data.staff.get(id)}
          deviceName={(id) => history.data.devices.get(id)}
          {...(history.data.timeZone ? { timeZone: history.data.timeZone } : {})}
        />
      )}
    </Sheet>
  );
}

interface Named {
  events: HistoryResponse["events"];
  staff: ReadonlyMap<number, string>;
  devices: ReadonlyMap<number, string>;
  timeZone: string | undefined;
}

/** The names as lookups, and a time zone the browser can use, or none. */
export function withNames(h: HistoryResponse): Named {
  return {
    events: h.events,
    staff: new Map(h.staff.map((s) => [s.id, s.name])),
    devices: new Map(h.devices.map((d) => [d.id, d.name])),
    timeZone: validZone(h.time_zone),
  };
}

function validZone(zone: string): string | undefined {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: zone });
    return zone;
  } catch {
    return undefined; // the device's own zone, rather than no timeline at all
  }
}
