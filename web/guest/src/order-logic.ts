// Placing an order, without React (docs/design/screens/guest-order.md,
// ADR-213): what the phone sends, the key that makes sending twice ordering
// once, what it remembers of the order it placed, and the words for each
// answer. The prices in the answer are the server's; nothing here adds up.

import { ApiError } from "../../shared/api";
import type { GuestOrderRequest, GuestRoundResponse, GuestRoundStatus } from "../../shared/api-types";
import type { PricedLine } from "./cart-logic";

/** How often a waiting order asks whether it has been answered. */
export const ASK_AGAIN_MS = 8_000;

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
}

export function placedStorageKey(code: string): string {
  return `mez.placed.${code}`;
}

const statuses: readonly GuestRoundStatus[] = ["waiting", "sent", "declined"];

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
  return { key: raw.key, request: request as unknown as GuestOrderRequest, round: round as unknown as GuestRoundResponse, at: raw.at };
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

/** Whether an order still has an answer to wait for. */
export function awaitsAnswer(placed: Placed | null): boolean {
  return placed?.round.status === "waiting";
}

// Words.

export interface StatusWords {
  readonly title: string;
  readonly line: string;
  /** One or two words for the strip over the menu. */
  readonly brief: string;
}

export function statusWords(status: GuestRoundStatus, venue: string): StatusWords {
  switch (status) {
    case "waiting":
      return {
        title: `Waiting for ${venue} to accept your order`,
        line: "A member of staff will confirm it in a moment. Nothing is being made yet.",
        brief: "Waiting to be accepted",
      };
    case "sent":
      return {
        title: "Your order is with the kitchen",
        line: "You can order more from the menu at any time. You pay when you are ready to leave.",
        brief: "Sent to the kitchen",
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
  if (error.unreachable || (error.status !== null && error.status >= 502 && error.status <= 504)) {
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
      return { text: error.message, readMenu: false };
    case "idempotency_key_reused":
      return { text: "Your cart changed while the order was being placed. Check it and place the order again.", readMenu: false };
    default:
      return { text: "Something went wrong and your order was not placed. Try again, or order at the counter.", readMenu: false };
  }
}
