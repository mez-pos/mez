// The words and arithmetic behind the open-shift screen (D02), apart from the
// components so they are tested without rendering.

import type { CountRequest } from "../../../shared/api-types";
import { isSoldOut } from "../../../shared/menu";
import { parseAmount } from "../../../shared/money";
import { countRequest, facesFor, maxPieces, type Tally } from "./shift-logic";

/**
 * The float as counted: by note, or for a currency with no table of notes as
 * one amount typed, null while what is typed is not an amount.
 */
export interface FloatCounted {
  tally: Tally;
  typed: number | null;
}

export const emptyFloat: FloatCounted = { tally: {}, typed: 0 };

/** One amount typed for the whole float. An empty box is none; text that is not an amount is null, never zero. */
export function typedFloat(text: string, currency: string): number | null {
  if (text.trim() === "") return 0;
  return parseAmount(text, currency);
}

/** The opening float to send, or null while the amount typed is not one: the shift is not opened on a guess. */
export function floatRequest(currency: string, counted: FloatCounted): CountRequest | null {
  if (facesFor(currency).length === 0 && counted.typed === null) return null;
  return countRequest(currency, counted.tally, counted.typed ?? 0);
}

/**
 * The tally after a count typed into one note's box. Digits only: anything
 * else typed or pasted is dropped, an empty box is none of that note, and the
 * count stops at what no drawer holds (the same ceiling as the pad).
 */
export function setFaceCount(tally: Tally, face: number, text: string): Tally {
  const digits = text.replace(/\D/g, "");
  const count = digits === "" ? 0 : Math.min(Number(digits), maxPieces);
  return { ...tally, [face]: count };
}

/** The names of the items sold out at `now`, in menu order. */
export function soldOutNames(items: readonly { name: string; sold_out_until?: string }[], now: Date): string[] {
  return items.filter((item) => isSoldOut(item, now)).map((item) => item.name);
}

/** What the sold-out notice says: its headline, and a sentence naming up to three of them. */
export function soldOutNotice(names: readonly string[]): { title: string; body: string } | null {
  if (names.length === 0) return null;
  const shown = names.slice(0, 3);
  const more = names.length - shown.length;
  const list = more > 0 ? `${shown.join(", ")} and ${more} more` : joinNames(shown);
  return {
    title: names.length === 1 ? "1 item is sold out" : `${names.length} items are sold out`,
    body: `${list} ${names.length === 1 ? "is" : "are"} sold out and can't be ordered until back on sale.`,
  };
}

/** "A", "A and B", "A, B and C". */
export function joinNames(names: readonly string[]): string {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}
