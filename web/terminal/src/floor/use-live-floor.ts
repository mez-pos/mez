// The floor in service (terminal-floor.md): every table and where it is in
// its turn. Read again when an order at a table moves, when the stream says
// the floor changed (a table cleared, the plan edited), and on a resync.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { CommandResponse, LiveFloorResponse, OrderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { newKey } from "../register/ticket-key";
import { newer, openTicketsKey, orderQueryKey } from "../register/use-ticket";
import type { FloorAction } from "./floor-logic";

export const liveFloorKey = ["floor", "live"] as const;

export interface LiveFloorApi {
  data: LiveFloorResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** Marks a dirty table cleared. */
  clear: (tableId: number) => Promise<void>;
  /** Lets guests go who ordered nothing: their empty ticket closes, the table is free. */
  release: (tableId: number) => Promise<void>;
  /** Moves a party, or joins or parts a table, as a floor mode decided (ADR-088). */
  act: (action: FloorAction) => Promise<void>;
}

export function useLiveFloor(): LiveFloorApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: liveFloorKey,
    queryFn: ({ signal }) => store.api.get<LiveFloorResponse>("/api/v1/floor/live", { signal }),
    staleTime: Infinity,
  });
  useRealtimeNotices((notice) => {
    const moved = notice.kind === "floor" || notice.kind === "resync" || (notice.kind === "event" && (notice.message.table_id ?? 0) > 0);
    if (moved) void queryClient.invalidateQueries({ queryKey: liveFloorKey });
  });

  const keep = (next: LiveFloorResponse) => {
    queryClient.setQueryData(liveFloorKey, next);
  };
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    clear: (id) => store.api.post<LiveFloorResponse>(`/api/v1/floor/tables/${id}/clear`).then(keep),
    release: (id) => store.api.post<LiveFloorResponse>(`/api/v1/floor/tables/${id}/release`).then(keep),
    act: async (action) => {
      const url = (suffix: string) => `/api/v1/orders/${encodeURIComponent(action.orderId)}${suffix}`;
      const key = { idempotencyKey: newKey() };
      const res =
        action.kind === "move"
          ? await store.api.post<CommandResponse>(url("/table"), { from: action.from, to: action.to }, key)
          : await store.api.post<CommandResponse>(url(action.kind === "join" ? "/tables/join" : "/tables/part"), { tables: [action.table] }, key);
      queryClient.setQueryData<OrderResponse>(orderQueryKey(res.order.id), (cached) => newer(cached, res.order));
      void queryClient.invalidateQueries({ queryKey: openTicketsKey });
      await queryClient.invalidateQueries({ queryKey: liveFloorKey });
    },
  };
}
