// The order at the table picked on the tables screen (D05). It is read beside
// the order in front, not in place of it: looking at a table must not take the
// cashier's own order out of their hands. Both read the same cache, by order id,
// and the cache only moves forward (ADR-074).

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { AddNote, CommandResponse, OrderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { failureOf } from "../register/order-messages";
import { newKey } from "../register/ticket-key";
import { newer, openTicketsKey, orderQueryKey, resetOrders } from "../register/use-ticket";
import { liveFloorKey } from "./use-live-floor";

export interface TableOrder {
  order: OrderResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** Sends what is waiting to its stations (ADR-089). Rejects with the words to show. */
  send: () => Promise<void>;
  /** Fires a course held back. Rejects with the words to show. */
  fire: (course: number) => Promise<void>;
  /** Writes a note on the order. Rejects with the words to show. */
  addNote: (note: AddNote) => Promise<void>;
}

export function useTableOrder(orderId: string): TableOrder {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const url = (suffix = "") => `/api/v1/orders/${encodeURIComponent(orderId)}${suffix}`;

  const query = useQuery({
    queryKey: orderQueryKey(orderId),
    queryFn: async ({ signal }) => {
      const read = await store.api.get<OrderResponse>(url(), { signal });
      return newer(queryClient.getQueryData<OrderResponse>(orderQueryKey(read.id)), read);
    },
    // Fresh until a command answers or the stream says it moved, while this
    // table is picked. Nobody listens for it once another is, so it is read
    // again each time it is picked: another till may have added to it since.
    staleTime: Infinity,
    refetchOnMount: "always",
  });

  useRealtimeNotices((notice) => {
    if (notice.kind === "menu" || notice.kind === "print" || notice.kind === "floor") return;
    if (notice.kind === "resync") void resetOrders(queryClient);
    else if (notice.message.order_id === orderId) void queryClient.invalidateQueries({ queryKey: orderQueryKey(orderId) });
  });

  const post = async (suffix: string, body?: unknown): Promise<void> => {
    try {
      const res = await store.api.post<CommandResponse>(url(suffix), body, { idempotencyKey: newKey() });
      queryClient.setQueryData<OrderResponse>(orderQueryKey(res.order.id), (cached) => newer(cached, res.order));
      void queryClient.invalidateQueries({ queryKey: openTicketsKey });
      void queryClient.invalidateQueries({ queryKey: liveFloorKey });
    } catch (err) {
      throw new Error(failureOf(err), { cause: err });
    }
  };

  return {
    order: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    send: () => post("/send"),
    fire: (course) => post(`/courses/${course}/fire`),
    addNote: (note) => post("/notes", note),
  };
}
