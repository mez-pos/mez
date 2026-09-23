// The register's current ticket: the queue that sends taps, the order as the
// server last described it, and the stream keeping it current.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, useSyncExternalStore } from "react";
import { ApiError } from "../../../shared/api";
import type { AddLine, AddNote, CommandResponse, DiscountRequest, OpenOrder, OrderResponse, Override } from "../../../shared/api-types";
import { browserStorage } from "../../../shared/auth/storage";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { newKey, ticketMemory } from "./ticket-key";
import { TicketQueue, type ModifyLine, type PendingLine } from "./ticket-queue";

export const orderQueryKey = (id: string) => ["order", id] as const;

const counterSale: OpenOrder = { channel: "counter", covers: 0 };

export interface Ticket {
  order: OrderResponse | undefined;
  orderId: string | null;
  pending: readonly PendingLine[];
  loading: boolean;
  ringUp: (command: AddLine) => void;
  retry: (key: string) => void;
  remove: (key: string) => void;
  /** Rejects with the words to show when the server refuses. */
  modify: (lineId: number, change: ModifyLine) => Promise<void>;
  changeQty: (lineId: number, from: number, to: number) => Promise<void>;
  addNote: (note: AddNote) => Promise<void>;
  /** The approval is a manager's; the server checks their PIN and permission. */
  /** Rung up by mistake, nothing done yet: no manager (ADR-073). */
  removeLine: (lineId: number) => Promise<void>;
  voidLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  compLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  discount: (request: DiscountRequest) => Promise<void>;
}

export function useTicket(): Ticket {
  const store = useAuthStore();
  const queryClient = useQueryClient();

  const [queue] = useState(() => {
    const memory = ticketMemory(browserStorage("session"));
    return new TicketQueue({
      commands: {
        openOrder: (key) => store.api.post<CommandResponse>("/api/v1/orders", counterSale, { idempotencyKey: key }),
        addLine: (orderId, command, key) =>
          store.api.post<CommandResponse>(`/api/v1/orders/${encodeURIComponent(orderId)}/lines`, command, {
            idempotencyKey: key,
          }),
        modifyLine: (orderId, lineId, change, key) =>
          store.api.post<CommandResponse>(
            `/api/v1/orders/${encodeURIComponent(orderId)}/lines/${lineId}/modifiers`,
            change,
            { idempotencyKey: key },
          ),
      },
      newKey: () => newKey(),
      onOrder: (order) => queryClient.setQueryData(orderQueryKey(order.id), order),
      remember: (id) => memory.write(id),
      initialOrderId: memory.read(),
    });
  });
  const state = useSyncExternalStore(queue.subscribe, queue.current);
  const orderId = state.orderId;

  const order = useQuery({
    queryKey: orderQueryKey(orderId ?? ""),
    queryFn: ({ signal }) =>
      store.api.get<OrderResponse>(`/api/v1/orders/${encodeURIComponent(orderId ?? "")}`, { signal }),
    enabled: orderId !== null,
    // Fresh until a command answers or the stream says it moved.
    staleTime: Infinity,
  });

  // A remembered order the server no longer has (a fresh database, another
  // venue): start again rather than adding to nothing.
  const gone = order.error instanceof ApiError && order.error.status === 404;
  useEffect(() => {
    if (gone) queue.startNew();
  }, [gone, queue]);

  useRealtimeNotices((notice) => {
    if (!orderId) return;
    const touched =
      notice.kind === "resync" || (notice.kind === "event" && notice.message.order_id === orderId);
    if (touched) void queryClient.invalidateQueries({ queryKey: orderQueryKey(orderId) });
  });

  const post = (suffix: string, body: unknown) =>
    queue.run((id, key) =>
      store.api.post<CommandResponse>(`/api/v1/orders/${encodeURIComponent(id)}${suffix}`, body, { idempotencyKey: key }),
    );

  return {
    order: orderId ? order.data : undefined,
    orderId,
    pending: state.pending,
    loading: orderId !== null && order.isPending,
    ringUp: (command) => queue.ringUp(command),
    retry: (key) => queue.retry(key),
    remove: (key) => queue.remove(key),
    modify: (lineId, change) => queue.modify(lineId, change),
    changeQty: (lineId, from, to) => post(`/lines/${lineId}/qty`, { from, to }),
    addNote: (note) => post("/notes", note),
    removeLine: (lineId) => post(`/lines/${lineId}/remove`, undefined),
    voidLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/void`, { ...approval, reason_code: reasonCode }),
    compLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/comp`, { ...approval, reason_code: reasonCode }),
    discount: (request) => post("/discounts", request),
  };
}
