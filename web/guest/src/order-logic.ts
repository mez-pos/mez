// Placing an order, without React (docs/design/screens/guest-order.md,
// ADR-213): what the phone sends, the key that makes sending twice ordering
// once, what it remembers of the order it placed, and the words for each
// answer. The prices in the answer are the server's; nothing here adds up.

import { ApiError, isUnreachable } from "../../shared/api";
import type { GuestOrderRequest, GuestRoundProgress, GuestRoundResponse, GuestRoundStatus, Money } from "../../shared/api-types";
import type { PricedLine } from "./cart-logic";

/** How often a waiting order asks whether it has been answered. */
export const ASK_AGAIN_MS = 8_000;

/** How often an order with the kitchen asks how far it has come. */
export const TRACK_MS = 10_000;

/** How often a ready order asks: only a recall moves it, and that is rare. */
export const TRACK_READY_MS = 30_000;

/** How long the phone remembers an order it placed: a meal, not a week. */
export const REMEMBER_MS = 3 * 60 * 60 * 1000;

/** What the server is sent: ids and counts, never a name or a price. */
export function toRequest(lines: readonly Pick<PricedLine, "item" | "modifiers" | "qty">[]): GuestOrderRequest {
  return { lines: lines.map((l) => ({ item_id: l.item.id, modifier_ids: l.modifiers.map((m) => m.modifier_id), qty: l.qty })) };
}

// The key. One is drawn for a cart the first time it is sent, and kept with
// what was sent. Sent again unchanged, a retry after a dropped connection,
// it goes with the same key and is the same order. Changed, it is another
// order and gets another key: the server refuses a key that comes back with
// something else in it.

export interface KeyRecord {
  readonly key: string;
  /** What the key was drawn for. */
  readonly body: string;
}

export function keyStorageKey(code: string): string {
  return `mez.order-key.${code}`;
}

export function keyFor(stored: KeyRecord | null, request: GuestOrderRequest, draw: () => string): KeyRecord {
  const body = JSON.stringify(request);
  return stored !== null && stored.body === body ? stored : { key: draw(), body };
}

export function parseKey(text: string | null): KeyRecord | null {
  const raw = json(text);
  if (!isRecord(raw) || typeof raw.key !== "string" || raw.key === "" || typeof raw.body !== "string") return null;
  return { key: raw.key, body: raw.body };
}

/** Thirty-two hex characters from the phone's own randomness. */
export function drawKey(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, "0")).join("");
}

// The order the phone placed. It is kept so a reload, or the phone coming
// back from a locked screen, still says what was ordered and how it stands,
// and so a waiting order can ask again with the key it was sent with.

export interface Placed {
  readonly key: string;
  readonly request: GuestOrderRequest;
  readonly round: GuestRoundResponse;
  /** When it was placed, by the phone's clock. */
  readonly at: number;
  /**
   * The venue held it until it was paid for (ADR-225). Kept once seen, so
   * an order that was paid first is not told it pays on leaving.
   */
  readonly paidFirst: boolean;
  /**
   * The order has been asked about at its own address since it was sent
   * (ADR-238), so what it says of the kitchen, or that it says nothing, is
   * the venue's answer and not a question still to ask.
   */
  readonly tracked: boolean;
}

type Before = Omit<Placed, "round" | "paidFirst" | "tracked"> & { paidFirst?: boolean; tracked?: boolean };

/**
 * The order with the venue's newer answer in it. asked is true when the
 * answer came from the order's own address, which is the only one that
 * says how far the kitchen has come.
 */
export function withRound(placed: Before, round: GuestRoundResponse, asked = false): Placed {
  return {
    key: placed.key,
    request: placed.request,
    at: placed.at,
    round,
    paidFirst: placed.paidFirst === true || round.status === "unpaid",
    tracked: round.status === "sent" && (asked || placed.tracked === true),
  };
}

/**
 * How long until an order is asked about again, or null when nothing more
 * is coming: it was declined or cancelled, or it was sent before orders
 * were tracked and the venue has said all it can.
 */
export function askAfterMs(placed: Placed | null): number | null {
  if (placed === null) return null;
  const { status, progress } = placed.round;
  if (status === "waiting" || status === "unpaid") return ASK_AGAIN_MS;
  if (status !== "sent") return null;
  // Not asked yet since it was sent: ask soon, to learn where it is.
  if (!placed.tracked) return ASK_AGAIN_MS;
  switch (progress) {
    case undefined:
    case "cancelled":
      return null;
    case "ready":
      return TRACK_READY_MS;
    default:
      return TRACK_MS;
  }
}

/** What an unpaid order asks the guest to pay, as the server worked it out, or null. */
export function dueOf(round: GuestRoundResponse): Money | null {
  return round.status === "unpaid" && round.due !== undefined && round.due.minor > 0 ? round.due : null;
}

/** A pickup code as the venue gives one: three or four digits and capitals (ADR-249). */
const PICKUP_CODE = /^[0-9A-Z]{3,4}$/;

/**
 * The code an order to collect is called out by, once it is paid for and
 * sent, or null. Anything that is not the shape of one is not shown.
 */
export function pickupCodeOf(round: Pick<GuestRoundResponse, "pickup_code">): string | null {
  return round.pickup_code !== undefined && PICKUP_CODE.test(round.pickup_code) ? round.pickup_code : null;
}

/**
 * Whether asking about an order was answered "not found": the venue no
 * longer finds it under this code, and asking again will say the same. A
 * failure to reach the venue, or being told to wait, is not that.
 */
export function orderGone(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export function placedStorageKey(code: string): string {
  return `mez.placed.${code}`;
}

const statuses: readonly GuestRoundStatus[] = ["waiting", "sent", "declined", "unpaid"];
const steps: readonly GuestRoundProgress[] = ["accepted", "sent", "preparing", "ready", "cancelled"];

/**
 * Reads a stored order. What is in storage is not trusted, and an order
 * from another day is not this sitting's: anything that is not one, or is
 * older than REMEMBER_MS, is nothing.
 */
export function parsePlaced(text: string | null, now: number): Placed | null {
  const raw = json(text);
  if (!isRecord(raw) || typeof raw.key !== "string" || typeof raw.at !== "number") return null;
  if (!Number.isFinite(raw.at) || now - raw.at > REMEMBER_MS || raw.at - now > REMEMBER_MS) return null;
  const { request, round } = raw;
  if (!isRecord(request) || !Array.isArray(request.lines)) return null;
  if (!isRecord(round) || typeof round.id !== "string" || !statuses.includes(round.status as GuestRoundStatus)) return null;
  if (!Array.isArray(round.lines) || !round.lines.every(isLine) || !isMoney(round.subtotal)) return null;
  if (round.due !== undefined && !isMoney(round.due)) return null;
  if (round.progress !== undefined && !steps.includes(round.progress as GuestRoundProgress)) return null;
  if (round.pickup_code !== undefined && (typeof round.pickup_code !== "string" || !PICKUP_CODE.test(round.pickup_code))) return null;
  return withRound(
    { key: raw.key, request: request as unknown as GuestOrderRequest, at: raw.at, paidFirst: raw.paidFirst === true, tracked: raw.tracked === true },
    round as unknown as GuestRoundResponse,
  );
}

function isLine(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.name === "string" &&
    typeof value.qty === "number" &&
    Array.isArray(value.choices) &&
    value.choices.every((c) => typeof c === "string") &&
    isMoney(value.total)
  );
}

function isMoney(value: unknown): boolean {
  return isRecord(value) && Number.isSafeInteger(value.minor) && typeof value.currency === "string";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function json(text: string | null): unknown {
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

// Words.

export interface StatusWords {
  readonly title: string;
  readonly line: string;
  /** One or two words for the strip over the menu. */
  readonly brief: string;
}

/**
 * What a sent order says of the kitchen (ADR-236). Each is something a
 * person in the kitchen did, read from the venue: never a guess at how long.
 * An order to collect is at no table: it is paid for, and fetched from the
 * counter.
 */
export function progressWords(progress: GuestRoundProgress, collect = false): StatusWords {
  switch (progress) {
    case "accepted":
      return { title: "Your order is accepted", line: collect ? "It is paid for." : "It is on your table's bill.", brief: "Accepted" };
    case "sent":
      return { title: "Your order is with the kitchen", line: "Nobody has started on it yet.", brief: "Sent to the kitchen" };
    case "preparing":
      return { title: "Your order is being made", line: "The kitchen has started on it.", brief: "Being made" };
    case "ready":
      return {
        title: "Your order is ready",
        line: collect ? "Collect it at the counter with your code." : "A member of staff will bring it to your table.",
        brief: "Ready",
      };
    case "cancelled":
      return {
        title: "This order was cancelled",
        line: collect ? "Nothing in it is being made. Please ask at the counter." : "Nothing in it is being made. Please ask a member of staff.",
        brief: "Cancelled",
      };
  }
}

/** The steps a sent order passes through, in order, for the row under its words. */
export const kitchenSteps: readonly { step: GuestRoundProgress; label: string }[] = [
  { step: "sent", label: "With the kitchen" },
  { step: "preparing", label: "Being made" },
  { step: "ready", label: "Ready" },
];

/** How far along the row an order is: the index of its step, or -1 when it is on none of them. */
export function stepIndex(progress: GuestRoundProgress | undefined): number {
  return kitchenSteps.findIndex((s) => s.step === progress);
}

/** What a sent order is told under how it stands: ordering more, and when it pays. To collect, what its code is for. */
export function sentNote(paidFirst: boolean, collect = false): string {
  if (collect) return "Your code is called at the counter when your order is ready.";
  return paidFirst ? "You can order more from the menu at any time." : "You can order more from the menu at any time. You pay when you are ready to leave.";
}

/** The words for an order as it stands: its status, and once sent and tracked, the kitchen's step. */
export function orderWords(placed: Pick<Placed, "round" | "paidFirst">, venue: string, collect = false): StatusWords {
  const { status, progress } = placed.round;
  if (status !== "sent") return statusWords(status, venue, placed.paidFirst);
  // Sent, and nothing known of the kitchen: the note under the card says
  // the rest, so the card has no second line to repeat it.
  if (progress === undefined) return { ...statusWords(status, venue, placed.paidFirst), line: "" };
  const words = progressWords(progress, collect);
  return placed.paidFirst && progress === "sent" ? { ...words, title: "Paid, and with the kitchen" } : words;
}

export function statusWords(status: GuestRoundStatus, venue: string, paidFirst = false): StatusWords {
  switch (status) {
    case "waiting":
      return {
        title: `Waiting for ${venue} to accept your order`,
        line: "A member of staff will confirm it in a moment. Nothing is being made yet.",
        brief: "Waiting to be accepted",
      };
    case "sent":
      return {
        title: paidFirst ? "Paid, and with the kitchen" : "Your order is with the kitchen",
        line: paidFirst
          ? "You can order more from the menu at any time."
          : "You can order more from the menu at any time. You pay when you are ready to leave.",
        brief: "Sent to the kitchen",
      };
    case "unpaid":
      return {
        title: "Pay to send your order",
        line: "Nothing is sent to the kitchen until this order is paid for.",
        brief: "Waiting for payment",
      };
    case "declined":
      return {
        title: "This order was not accepted",
        line: "Nothing was sent to the kitchen. Please ask a member of staff.",
        brief: "Not accepted",
      };
  }
}

export interface Refusal {
  /** What to tell the guest. */
  readonly text: string;
  /** The menu has moved: read it again, and the cart will follow it. */
  readonly readMenu: boolean;
}

/**
 * Why an order was not placed, in words a guest can act on. The server's
 * own message is used where it wrote one for a guest; a failure to reach it
 * is said here.
 */
export function refusalOf(error: unknown): Refusal {
  if (!(error instanceof ApiError)) return { text: "Something went wrong and your order was not placed. Try again, or order at the counter.", readMenu: false };
  if (isUnreachable(error)) {
    return { text: "Can't reach the kitchen, so your order was not placed. Check your connection and try again.", readMenu: false };
  }
  if (error.status === 429) return { text: "Too many orders at once. Wait a moment and try again.", readMenu: false };
  switch (error.code) {
    case "dish_unavailable":
    case "choices_invalid":
      return { text: error.message, readMenu: true };
    case "no_open_shift":
    case "table_not_ordering":
    case "requests_waiting":
    case "table_code_unknown":
    case "pickup_code_unknown":
    case "pickup_off":
    case "guest_session_required":
      return { text: error.message, readMenu: false };
    case "idempotency_key_reused":
      return { text: "Your cart changed while the order was being placed. Check it and place the order again.", readMenu: false };
    default:
      return { text: "Something went wrong and your order was not placed. Try again, or order at the counter.", readMenu: false };
  }
}
