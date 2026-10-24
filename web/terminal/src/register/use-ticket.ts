// The register's tickets: the one in front, with the queue that sends its
// taps and the order as the server last described it, the others this till
// is working, and the venue's open-ticket list. The stream keeps all of it
// current (Task 5, ADR-074).

import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";
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
  TenderRequest,
} from "../../../shared/api-types";
import { browserStorage } from "../../../shared/auth/storage";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useToast } from "../../../shared/ui";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { TicketBook } from "./ticket-book";
import { newKey, ticketMemory } from "./ticket-key";
import { TicketQueue, type ModifyLine, type PendingLine } from "./ticket-queue";

/** Every cached order, whichever id: the family orderQueryKey belongs to. */
export const orderFamilyKey = ["order"] as const;
export const orderQueryKey = (id: string) => ["order", id] as const;

/**
 * After a resync, what is cached is not a base the server's answer can be
 * held against: a restored backup has orders behind what this till saw,
 * and newer() would keep the till's copy over the truth for good. Every
 * cached order is forgotten: the ones on screen are read again and taken
 * as they come, the rest are read fresh when next looked at.
 */
export function resetOrders(queryClient: QueryClient): Promise<void> {
  return queryClient.resetQueries({ queryKey: orderFamilyKey });
}

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
  /** Why the order in front could not be read, or null. */
  error: unknown;
  /** Reads the order in front again. */
  refetch: () => void;
  /** How the ticket in front opens, or opened: counter or table, covers. */
  opening: OpenOrder;
  /** Every unclosed ticket at the venue, oldest first; undefined until read. */
  open: OpenTicketsResponse | undefined;
  /** Rings a line up, with a kitchen note for it if one was written (ADR-153). */
  ringUp: (command: AddLine, note?: string) => void;
  retry: (key: string) => void;
  remove: (key: string) => void;
  startNew: (open: OpenOrder) => void;
  /** Seats a party: a ticket in front whose order opens now, at the table. */
  seat: (open: OpenOrder) => Promise<void>;
  switchTo: (orderId: string) => void;
  /** Parks the ticket in front and starts a counter ticket. */
  hold: () => Promise<void>;
  /** Brings a held ticket to the front and takes it off hold. */
  resume: (orderId: string) => Promise<void>;
  /** Rejects with the words to show when the server refuses. */
  modify: (lineId: number, change: ModifyLine) => Promise<void>;
  changeQty: (lineId: number, from: number, to: number) => Promise<void>;
  addNote: (note: AddNote) => Promise<void>;
  /** Sends what is waiting to its stations, firing the first course if none has gone (ADR-089). */
  send: () => Promise<void>;
  /** Fires a course: its lines go to their stations. */
  fire: (course: number) => Promise<void>;
  /** Rung up by mistake, nothing done yet: no manager (ADR-073). */
  removeLine: (lineId: number) => Promise<void>;
  /** The approval is a manager's; the server checks their PIN and permission. */
  voidLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  compLine: (lineId: number, reasonCode: string, approval: Override) => Promise<void>;
  discount: (request: DiscountRequest) => Promise<void>;
  /** Takes a discount off again, with a manager's approval (ADR-373). */
  removeDiscount: (seq: number, reason: string, approval: Override) => Promise<void>;
  /**
   * Takes a payment. The key is the caller's, kept across retries of the
   * same tender, so a lost answer cannot charge twice (terminal-tender.md).
   */
  tender: (payment: TenderRequest, key: string) => Promise<void>;
  /**
   * Reads the order again from the server, past the cache: after a payment
   * whose answer was lost, to see whether it was recorded. Rejects when the
   * server cannot be reached.
   */
  reread: () => Promise<OrderResponse>;
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
  const toast = useToast();
  // The book is made once; what it says when a note is lost follows the render.
  const noteLost = useRef<(name: string, why: string) => void>(() => undefined);
  useLayoutEffect(() => {
    noteLost.current = (name, why) => toast(`${name} is on the order, but its note is not. ${why}`, { tone: "warning" });
  });

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
            modifyLine: (id, lineId, change, key) => store.api.post<CommandResponse>(url(id, `/lines/${lineId}/modifiers`), change, { idempotencyKey: key }),
            addNote: (id, note, key) => store.api.post<CommandResponse>(url(id, "/notes"), note, { idempotencyKey: key }),
          },
          onNoteLost: (name, why) => noteLost.current(name, why),
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
    // Fresh until a command answers or the stream says it moved, while a
    // screen is listening; one coming back (from the lock screen, from
    // Tables) reads again, since the stream was heard by nobody meanwhile.
    staleTime: Infinity,
    refetchOnMount: "always",
  });

  const open = useQuery({
    queryKey: openTicketsKey,
    queryFn: ({ signal }) => store.api.get<OpenTicketsResponse>("/api/v1/orders", { signal }),
    staleTime: Infinity,
    refetchOnMount: "always",
  });

  // A remembered order the server no longer has (a fresh database, another
  // venue), or one closed elsewhere: start again rather than add to it.
  const gone = (order.error instanceof ApiError && order.error.status === 404) || order.data?.status === "closed";
  useEffect(() => {
    if (gone) book.startNew(counterSale);
  }, [gone, book]);

  useRealtimeNotices((notice) => {
    if (notice.kind === "menu" || notice.kind === "print" || notice.kind === "floor") return;
    // Any order's event can change the list: a new ticket on another till.
    void queryClient.invalidateQueries({ queryKey: openTicketsKey });
    if (notice.kind === "resync") {
      void resetOrders(queryClient);
      return;
    }
    if (orderId && notice.message.order_id === orderId) void queryClient.invalidateQueries({ queryKey: orderQueryKey(orderId) });
  });

  const post = (suffix: string, body: unknown, on = queue, key?: string) =>
    on.run((id, sent) => store.api.post<CommandResponse>(`/api/v1/orders/${encodeURIComponent(id)}${suffix}`, body, { idempotencyKey: sent }), key);

  return {
    order: orderId ? order.data : undefined,
    orderId,
    pending: state.pending,
    loading: orderId !== null && order.isPending,
    error: orderId !== null ? order.error : null,
    refetch: () => void order.refetch(),
    opening: queue.opening,
    open: open.data,
    ringUp: (command, note) => queue.ringUp(command, note),
    retry: (key) => queue.retry(key),
    remove: (key) => queue.remove(key),
    startNew: (opening) => book.startNew(opening),
    seat: (opening) => book.seat(opening).then(() => undefined),
    switchTo: (id) => void book.switchTo(id),
    hold: async () => {
      await post("/hold", undefined);
      book.startNew(counterSale);
    },
    resume: (id) => post("/resume", undefined, book.switchTo(id)),
    modify: (lineId, change) => queue.modify(lineId, change),
    changeQty: (lineId, from, to) => post(`/lines/${lineId}/qty`, { from, to }),
    addNote: (note) => post("/notes", note),
    send: () => post("/send", undefined),
    fire: (course) => post(`/courses/${course}/fire`, undefined),
    removeLine: (lineId) => post(`/lines/${lineId}/remove`, undefined),
    voidLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/void`, { ...approval, reason_code: reasonCode }),
    compLine: (lineId, reasonCode, approval) => post(`/lines/${lineId}/comp`, { ...approval, reason_code: reasonCode }),
    discount: (request) => post("/discounts", request),
    removeDiscount: (seq, reason, approval) => post(`/discounts/${seq}/remove`, { ...approval, reason }),
    tender: (payment, key) => post("/payments", payment, queue, key),
    reread: async () => {
      const read = await order.refetch({ throwOnError: true });
      if (!read.data) throw new Error("There is no order to read.");
      return read.data;
    },
    issueInvoice: () => post("/invoice", undefined),
    close: () => post("/close", undefined),
    reprint: (invoiceSeq, reason, approval) => post(`/invoices/${invoiceSeq}/reprint`, { ...approval, reason }),
  };
}
