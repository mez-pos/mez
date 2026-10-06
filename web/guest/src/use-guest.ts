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
  GuestRoundResponse,
  GuestTableResponse,
} from "../../shared/api-types";

// A guest is nobody: no device, no session. The code in the path is all
// there is to present.
const api = createApiClient({ credentials: noCredentials });

const base = (code: string) => `/order/api/t/${encodeURIComponent(code)}`;

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

export function useTable(code: string): UseQueryResult<GuestTableResponse> {
  return useQuery({
    queryKey: ["guest", "table", code],
    queryFn: ({ signal }) => api.get<GuestTableResponse>(base(code), { signal }),
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

export function useBill(code: string): UseQueryResult<GuestBillResponse> {
  return useQuery({
    queryKey: billKey(code),
    queryFn: ({ signal }) => api.get<GuestBillResponse>(`${base(code)}/bill`, { signal }),
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

/** Starts paying what the table owes through the venue's gateway (ADR-222). */
export function startCharge(code: string): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/bill/charge`);
}

/** Starts paying for an order the venue is holding until it is paid (ADR-227). */
export function startRoundCharge(code: string, round: string): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/orders/${encodeURIComponent(round)}/charge`);
}

/** How a charge this table started stands, as the venue's server found out from the gateway. */
export function readCharge(code: string, id: string, signal: AbortSignal): Promise<GuestChargeResponse> {
  return api.get<GuestChargeResponse>(`${base(code)}/bill/charge/${encodeURIComponent(id)}`, { signal });
}
