// Past bills and the commands on one (terminal-past-bills.md). A bill is
// read like any order, kept current by the stream; every command carries an
// Idempotency-Key kept across retries of the same command, whoever approves
// the retry, so a refund whose answer was lost is never issued twice.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";
import type {
  CommandResponse,
  CreditPlanResponse,
  CreditSelection,
  DayOrdersResponse,
  OrderResponse,
  Override,
  PastBillsResponse,
} from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { failureOf } from "../register/order-messages";
import { commandIdentity } from "./bills-logic";
import { newKey } from "../register/ticket-key";
import { newer, orderQueryKey } from "../register/use-ticket";

export const pastBillsKey = (q: string) => ["orders", "closed", q] as const;

export const dayOrdersKey = ["orders", "today"] as const;

/**
 * The trading day's orders (D09), kept current: any order's event, on this
 * till or another, asks for the list again.
 */
export function useDayOrders() {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  useRealtimeNotices((notice) => {
    if (notice.kind === "resync" || notice.kind === "event") void queryClient.invalidateQueries({ queryKey: dayOrdersKey });
  });
  return useQuery({
    queryKey: dayOrdersKey,
    queryFn: ({ signal }) => store.api.get<DayOrdersResponse>("/api/v1/orders/today", { signal }),
    staleTime: 15_000,
  });
}

/** Closed bills an invoice or credit note number finds, from any day. Asks nothing for an empty query when told not to. */
export function usePastBills(query: string, enabled = true) {
  const store = useAuthStore();
  return useQuery({
    enabled,
    queryKey: pastBillsKey(query),
    queryFn: ({ signal }) => store.api.get<PastBillsResponse>(`/api/v1/orders/closed?q=${encodeURIComponent(query)}`, { signal }),
    staleTime: 30_000,
  });
}

/** A key per request body: the same body again is a retry, and reuses it. */
export function useAttemptKeys() {
  const last = useRef<{ body: string; key: string } | null>(null);
  return {
    keyFor(body: unknown): string {
      const json = JSON.stringify(body);
      if (last.current?.body !== json) last.current = { body: json, key: newKey() };
      return last.current.key;
    },
    done() {
      last.current = null;
    },
  };
}

export interface BillCommands {
  credit: (selection: CreditSelection, paymentId: number, total: CreditPlanResponse["total"], reason: string, approval: Override) => Promise<string>;
  voidPayment: (paymentId: number, reason: string, approval: Override) => Promise<void>;
  reprint: (invoiceSeq: number, reason: string, approval: Override) => Promise<void>;
  /** Closes a settled order: for one to collect, it has been handed over (ADR-253). */
  close: () => Promise<void>;
}

/** One past bill, read and kept current, and what can be done to it. */
export function useBill(orderId: string): { order: OrderResponse | undefined; error: unknown; commands: BillCommands } {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const keys = useAttemptKeys();
  const path = `/api/v1/orders/${encodeURIComponent(orderId)}`;

  const order = useQuery({
    queryKey: orderQueryKey(orderId),
    queryFn: async ({ signal }) => {
      const read = await store.api.get<OrderResponse>(path, { signal });
      return newer(queryClient.getQueryData<OrderResponse>(orderQueryKey(read.id)), read);
    },
    staleTime: Infinity,
  });
  useRealtimeNotices((notice) => {
    const touched = notice.kind === "resync" || (notice.kind === "event" && notice.message.order_id === orderId);
    if (touched) void queryClient.invalidateQueries({ queryKey: orderQueryKey(orderId) });
  });

  const post = async (suffix: string, body: unknown): Promise<CommandResponse> => {
    try {
      const res = await store.api.post<CommandResponse>(path + suffix, body, { idempotencyKey: keys.keyFor(commandIdentity(suffix, body)) });
      keys.done();
      queryClient.setQueryData<OrderResponse>(orderQueryKey(orderId), (cached) => newer(cached, res.order));
      void queryClient.invalidateQueries({ queryKey: ["orders", "closed"] });
      void queryClient.invalidateQueries({ queryKey: dayOrdersKey });
      return res;
    } catch (err) {
      throw new Error(failureOf(err), { cause: err });
    }
  };

  return {
    order: order.data,
    error: order.error,
    commands: {
      credit: async (selection, paymentId, total, reason, approval) => {
        const res = await post("/credit-notes", {
          ...approval,
          credit: selection,
          refunds: [{ payment_id: paymentId, amount: total }],
          reason,
        });
        const issued = res.order.credits.at(-1);
        return issued?.credit_no ?? "";
      },
      voidPayment: async (paymentId, reason, approval) => {
        await post(`/payments/${paymentId}/void`, { ...approval, reason });
      },
      reprint: async (invoiceSeq, reason, approval) => {
        await post(`/invoices/${invoiceSeq}/reprint`, { ...approval, reason });
      },
      close: async () => {
        await post("/close", undefined);
      },
    },
  };
}
