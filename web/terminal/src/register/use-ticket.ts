// The register's tickets: the one in front, with the queue that sends its
// taps and the order as the server last described it, the others this till
// is working, and the venue's open-ticket list. The stream keeps all of it
// current (Task 5, ADR-074).

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, useSyncExternalStore } from "react";
import { ApiError } from "../../../shared/api";
import type {
  AddLine,
  AddNote,
  CommandResponse,
  DiscountRequest,
  OpenOrder,
  OpenTicketsResponse,
  OrderResponse,
  Override,
  TenderPayment,
} from "../../../shared/api-types";
import { browserStorage } from "../../../shared/auth/storage";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { TicketBook } from "./ticket-book";
import { newKey, ticketMemory } from "./ticket-key";
import { TicketQueue, type ModifyLine, type PendingLine } from "./ticket-queue";

export const orderQueryKey = (id: string) => ["order", id] as const;

/**
 * The newer of two copies of an order, by last_seq. The cache only moves
 * forward: a read that left before a command and lands after it must not
 * put the older order back (ADR-074).
 */
export function newer(cached: OrderResponse | undefined, incoming: OrderResponse): OrderResponse {
  return cached && cached.id === incoming.id && cached.last_seq > incoming.last_seq ? cached : incoming;
}

export const openTicketsKey = ["orders", "open"] as const;

export interface Ticket {
  order: OrderResponse | undefined;
  orderId: string | null;
  pending: readonly PendingLine[];
  loading: boolean;
  /** How the ticket in front opens, or opened: counter or table, covers. */
  opening: OpenOrder;
  /** Every unclosed ticket at the venue, oldest first; undefined until read. */
  open: OpenTicketsResponse | undefined;
  ringUp: (command: AddLine) => void;
  retry: (key: string) => void;
  remove: (key: string) => void;
  startNew: (open: OpenOrder) => void;
  switchTo: (orderId: string) => void;
  /** Parks the ticket in front and starts a counter ticket. */
  hold: () => Promise<void>;
  /** Brings a held ticket to the front and takes it off hold. */
  resume: (orderId: string) => Promise<void>;
  /** Rejects with the words to show when the server refuses. */
  modify: (lineId: number, change: ModifyLine) => Promise<void>;
  changeQty: (lineId: number, from: number, to: number) => Promise<void>;
  addNote: (note: AddNote) => Promise<void>;
  /** Rung up by mistake, nothing done yet: no manager (ADR-073). */
  removeLine: (lineId: number) => Promise<void>;
  /** The approval is a manager's; the server checks their PIN and permission. */
  voidLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  compLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  discount: (request: DiscountRequest) => Promise<void>;
  /**
   * Takes a payment. The key is the caller's, kept across retries of the
   * same tender, so a lost answer cannot charge twice (terminal-tender.md).
   */
  tender: (payment: TenderPayment, key: string) => Promise<void>;
  /** Issues the invoice for a bill paid before invoices came with the payment. */
  issueInvoice: () => Promise<void>;
  /** Closes the ticket; the register then starts a counter ticket. */
  close: () => Promise<void>;
  /** Logs a copy of an invoice, with a manager's approval; no new number. */
  reprint: (invoiceSeq: number, reason: string, approval: Override) => Promise<void>;
}

const counterSale: OpenOrder = { channel: "counter", covers: 0 };

export function useTicket(): Ticket {
  const store = useAuthStore();
  const queryClient = useQueryClient();

  const [book] = useState(() => {
    const memory = ticketMemory(browserStorage("session"));
    const url = (id: string, suffix = "") => `/api/v1/orders/${encodeURIComponent(id)}${suffix}`;
    return new TicketBook({
      makeQueue: (ticket) =>
        new TicketQueue({
          ...ticket,
          commands: {
            openOrder: (open, key) => store.api.post<CommandResponse>("/api/v1/orders", open, { idempotencyKey: key }),
            addLine: (id, command, key) => store.api.post<CommandResponse>(url(id, "/lines"), command, { idempotencyKey: key }),
            modifyLine: (id, lineId, change, key) =>
              store.api.post<CommandResponse>(url(id, `/lines/${lineId}/modifiers`), change, { idempotencyKey: key }),
          },
          newKey: () => newKey(),
          onOrder: (order) => {
            queryClient.setQueryData<OrderResponse>(orderQueryKey(order.id), (cached) => newer(cached, order));
            void queryClient.invalidateQueries({ queryKey: openTicketsKey });
          },
        }),
      remember: (id) => memory.write(id),
      initialOrderId: memory.read(),
    });
  });
  const { current: queue } = useSyncExternalStore(book.subscribe, book.current);
  const state = useSyncExternalStore(queue.subscribe, queue.current);
  const orderId = state.orderId;

  const order = useQuery({
    queryKey: orderQueryKey(orderId ?? ""),
    queryFn: async ({ signal }) => {
      const read = await store.api.get<OrderResponse>(`/api/v1/orders/${encodeURIComponent(orderId ?? "")}`, { signal });
      return newer(queryClient.getQueryData<OrderResponse>(orderQueryKey(read.id)), read);
    },
    enabled: orderId !== null,
    // Fresh until a command answers or the stream says it moved.
    staleTime: Infinity,
  });

  const open = useQuery({
    queryKey: openTicketsKey,
    queryFn: ({ signal }) => store.api.get<OpenTicketsResponse>("/api/v1/orders", { signal }),
    staleTime: Infinity,
  });

  // A remembered order the server no longer has (a fresh database, another
  // venue), or one closed elsewhere: start again rather than add to it.
  const gone = (order.error instanceof ApiError && order.error.status === 404) || order.data?.status === "closed";
  useEffect(() => {
    if (gone) book.startNew(counterSale);
  }, [gone, book]);

  useRealtimeNotices((notice) => {
    if (notice.kind === "menu") return;
    // Any order's event can change the list: a new ticket on another till.
    void queryClient.invalidateQueries({ queryKey: openTicketsKey });
    if (!orderId) return;
    const touched = notice.kind === "resync" || notice.message.order_id === orderId;
    if (touched) void queryClient.invalidateQueries({ queryKey: orderQueryKey(orderId) });
  });

  const post = (suffix: string, body: unknown, on = queue, key?: string) =>
    on.run(
      (id, sent) =>
        store.api.post<CommandResponse>(`/api/v1/orders/${encodeURIComponent(id)}${suffix}`, body, { idempotencyKey: sent }),
      key,
    );

  return {
    order: orderId ? order.data : undefined,
    orderId,
    pending: state.pending,
    loading: orderId !== null && order.isPending,
    opening: queue.opening,
    open: open.data,
    ringUp: (command) => queue.ringUp(command),
    retry: (key) => queue.retry(key),
    remove: (key) => queue.remove(key),
    startNew: (opening) => book.startNew(opening),
    switchTo: (id) => void book.switchTo(id),
    hold: async () => {
      await post("/hold", undefined);
      book.startNew(counterSale);
    },
    resume: (id) => post("/resume", undefined, book.switchTo(id)),
    modify: (lineId, change) => queue.modify(lineId, change),
    changeQty: (lineId, from, to) => post(`/lines/${lineId}/qty`, { from, to }),
    addNote: (note) => post("/notes", note),
    removeLine: (lineId) => post(`/lines/${lineId}/remove`, undefined),
    voidLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/void`, { ...approval, reason_code: reasonCode }),
    compLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/comp`, { ...approval, reason_code: reasonCode }),
    discount: (request) => post("/discounts", request),
    tender: (payment, key) => post("/payments", payment, queue, key),
    issueInvoice: () => post("/invoice", undefined),
    close: () => post("/close", undefined),
    reprint: (invoiceSeq, reason, approval) => post(`/invoices/${invoiceSeq}/reprint`, { ...approval, reason }),
  };
}
