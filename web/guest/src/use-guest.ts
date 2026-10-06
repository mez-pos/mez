// What a guest's phone reads from the venue (ADR-206): the table its code
// leads to, and the menu. Both are under the guest app's own mount, by the
// same absolute path on the venue's wifi and through the relay.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { createApiClient, noCredentials } from "../../shared/api";
import { BILL_REFRESH_MS } from "./bill-logic";
import type {
  GuestBillResponse,
  GuestChargeResponse,
  GuestMenuResponse,
  GuestOrderRequest,
  GuestPaidResponse,
  GuestPickupResponse,
  GuestReceiptResponse,
  GuestRoundResponse,
  GuestShareRequest,
  GuestTableResponse,
} from "../../shared/api-types";
import { newPhoneName, PHONE_STORAGE_KEY, phoneNameOf } from "./split-logic";

// A guest is nobody: no device, no session. The code in the path is all
// there is to present.
const api = createApiClient({ credentials: noCredentials });

// The venue's pickup code, when that is what the page was opened with
// (ADR-250). A page has one code for its life, and a pickup code is never
// the same string as a table's (ADR-247), so the code itself says which API
// it is asked under: "p" for this one, "t" for any other.
let collecting: string | null = null;

/** Says the page's code is the venue's pickup code. Called once, before anything is asked. */
export function enterCollect(code: string): void {
  collecting = code;
}

const base = (code: string) => `/order/api/${code === collecting ? "p" : "t"}/${encodeURIComponent(code)}`;

/** How often the menu is read again: what is sold out turns on the kitchen's clock. */
const MENU_REFRESH_MS = 60_000;

export function menuKey(code: string): readonly string[] {
  return ["guest", "menu", code];
}

/**
 * Places an order, or asks how one placed with this key stands: the server
 * answers the same key with the same order, as it is now (ADR-210).
 */
export function sendOrder(code: string, request: GuestOrderRequest, key: string): Promise<GuestRoundResponse> {
  return api.post<GuestRoundResponse>(`${base(code)}/orders`, request, { idempotencyKey: key });
}

/** How an order placed at this table stands, and once sent how far the kitchen has come (ADR-238). */
export function trackOrder(code: string, round: string): Promise<GuestRoundResponse> {
  return api.get<GuestRoundResponse>(`${base(code)}/orders/${encodeURIComponent(round)}`);
}

/** A settled bill's receipt: the counter's own invoice, by the table's code and the bill's id (ADR-240). */
export function readReceipt(code: string, bill: string, signal: AbortSignal): Promise<GuestReceiptResponse> {
  return api.get<GuestReceiptResponse>(`${base(code)}/receipts/${encodeURIComponent(bill)}`, { signal });
}

/**
 * Where the page's code leads: a table at a venue, or for a pickup code the
 * venue alone. With no table its name is empty, which the screens read as
 * "to collect" (tableLabel).
 */
export interface Where extends GuestTableResponse {
  readonly collect: boolean;
  /** False where the venue takes no orders to collect: guests cannot pay online there. */
  readonly ordering: boolean;
}

export function useTable(code: string): UseQueryResult<Where> {
  return useQuery({
    queryKey: ["guest", "table", code],
    queryFn: async ({ signal }): Promise<Where> => {
      if (code !== collecting) return { ...(await api.get<GuestTableResponse>(base(code), { signal })), collect: false, ordering: true };
      const venue = await api.get<GuestPickupResponse>(base(code), { signal });
      return { venue: venue.venue, table: { name: "" }, collect: true, ordering: venue.ordering };
    },
  });
}

export function useGuestMenu(code: string, enabled: boolean): UseQueryResult<GuestMenuResponse> {
  return useQuery({
    queryKey: menuKey(code),
    queryFn: ({ signal }) => api.get<GuestMenuResponse>(`${base(code)}/menu`, { signal }),
    enabled,
    refetchInterval: MENU_REFRESH_MS,
    // A phone comes back from the lock screen with a menu that may be an
    // hour old.
    refetchOnWindowFocus: true,
  });
}

/**
 * The table's bill, read while its screen is shown and again every few
 * seconds: a round accepted or a payment taken moves it.
 */
export function billKey(code: string): readonly string[] {
  return ["guest", "bill", code];
}

/** Tells staff a guest at the table says they have paid (ADR-218). It pays nothing. */
export function tellPaid(code: string): Promise<GuestPaidResponse> {
  return api.post<GuestPaidResponse>(`${base(code)}/bill/paid`);
}

/** The name for the life of the page, when storage will not keep one. */
let unkept: string | null = null;

/**
 * The name this phone gave itself (ADR-233), made once and kept: how the
 * venue tells one guest's share from another's of the same amount. Null
 * where the browser can make no random name; the phone then pays the rest
 * of a bill and no share of it.
 */
export function phoneName(): string | null {
  if (unkept !== null) return unkept;
  try {
    const kept = phoneNameOf(window.localStorage.getItem(PHONE_STORAGE_KEY));
    if (kept !== null) return kept;
  } catch {
    // Storage refused: a name is made below and lasts until the page goes.
  }
  let made: string;
  try {
    made = newPhoneName(crypto.getRandomValues(new Uint8Array(16)));
  } catch {
    return null;
  }
  try {
    window.localStorage.setItem(PHONE_STORAGE_KEY, made);
  } catch {
    unkept = made;
  }
  return made;
}

export function useBill(code: string): UseQueryResult<GuestBillResponse> {
  const phone = phoneName();
  const who = phone === null ? "" : `?phone=${encodeURIComponent(phone)}`;
  return useQuery({
    queryKey: billKey(code),
    queryFn: ({ signal }) => api.get<GuestBillResponse>(`${base(code)}/bill${who}`, { signal }),
    refetchInterval: BILL_REFRESH_MS,
    refetchOnWindowFocus: true,
    // Never shown from an earlier look without asking again.
    staleTime: 0,
    gcTime: 0,
  });
}

/**
 * Where the bill's UPI payment is drawn as a QR code (ADR-217). The amount
 * is in the address only so the picture is asked for again when it
 * changes: the server draws the balance as it stands and reads no amount
 * from a phone.
 */
export function upiQrUrl(code: string, amountMinor: number): string {
  return `${base(code)}/bill/upi.svg?am=${amountMinor}`;
}

/**
 * Starts paying through the venue's gateway (ADR-222): with no share, the
 * rest of what the table owes; with one, this phone's part of it (ADR-233).
 */
export function startCharge(code: string, share: GuestShareRequest | null = null): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/bill/charge`, share ?? undefined);
}

/** Gives up a payment page this table opened, so it holds no part of the bill (ADR-234). */
export function letGoCharge(code: string, id: string): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/bill/charge/${encodeURIComponent(id)}/let-go`);
}

/** Starts paying for an order the venue is holding until it is paid (ADR-227). */
export function startRoundCharge(code: string, round: string): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/orders/${encodeURIComponent(round)}/charge`);
}

/** How a charge this table started stands, as the venue's server found out from the gateway. */
export function readCharge(code: string, id: string, signal: AbortSignal): Promise<GuestChargeResponse> {
  // An order to collect has no bill: its payment is asked about beside it.
  return api.get<GuestChargeResponse>(`${base(code)}/${code === collecting ? "" : "bill/"}charge/${encodeURIComponent(id)}`, { signal });
}
