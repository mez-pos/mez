// The receipt, and which bill it is the receipt of (ADR-240, ADR-241). The
// phone is told a bill's id while that bill is open, on the bill and on an
// order it placed, and keeps the latest by table code. Once the bill is
// settled that id, with the table's code, is what the receipt is asked by.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import type { GuestReceiptResponse } from "../../shared/api-types";
import { billIdOf, billStorageKey, parseKeptBill, type KeptBill } from "./receipt-logic";
import { readReceipt } from "./use-guest";

/** The bills for the life of the page, when storage will not keep one. */
const unkept = new Map<string, KeptBill>();

/** The bill this phone last saw open at this table, if it still remembers one. */
export function keptBill(code: string): KeptBill | null {
  try {
    const kept = parseKeptBill(window.localStorage.getItem(billStorageKey(code)), Date.now());
    if (kept !== null) return kept;
  } catch {
    // Storage refused: what the page itself remembers is all there is.
  }
  return unkept.get(code) ?? null;
}

/** Remembers a bill the venue just named. Anything that is not a bill's id is ignored. */
export function keepBill(code: string, id: string | undefined): void {
  const bill = billIdOf(id);
  if (bill === null) return;
  const kept: KeptBill = { id: bill, at: Date.now() };
  unkept.set(code, kept);
  try {
    window.localStorage.setItem(billStorageKey(code), JSON.stringify(kept));
  } catch {
    // Kept for as long as the page, then.
  }
}

export function useReceipt(code: string, bill: string | null): UseQueryResult<GuestReceiptResponse> {
  return useQuery({
    queryKey: ["guest", "receipt", code, bill ?? ""],
    queryFn: ({ signal }) => readReceipt(code, bill ?? "", signal),
    enabled: bill !== null,
    // A credit note or a refund can still change it: read when it is opened
    // and when the phone comes back, never shown from an earlier look.
    refetchOnWindowFocus: true,
    staleTime: 0,
    gcTime: 0,
    retry: false,
  });
}
