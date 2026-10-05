// What a guest's phone reads from the venue (ADR-206): the table its code
// leads to, and the menu. Both are under the guest app's own mount, by the
// same absolute path on the venue's wifi and through the relay.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { createApiClient, noCredentials } from "../../shared/api";
import type { GuestMenuResponse, GuestTableResponse } from "../../shared/api-types";

// A guest is nobody: no device, no session. The code in the path is all
// there is to present.
const api = createApiClient({ credentials: noCredentials });

const base = (code: string) => `/order/api/t/${encodeURIComponent(code)}`;

/** How often the menu is read again: what is sold out turns on the kitchen's clock. */
const MENU_REFRESH_MS = 60_000;

export function useTable(code: string): UseQueryResult<GuestTableResponse> {
  return useQuery({
    queryKey: ["guest", "table", code],
    queryFn: ({ signal }) => api.get<GuestTableResponse>(base(code), { signal }),
  });
}

export function useGuestMenu(code: string, enabled: boolean): UseQueryResult<GuestMenuResponse> {
  return useQuery({
    queryKey: ["guest", "menu", code],
    queryFn: ({ signal }) => api.get<GuestMenuResponse>(`${base(code)}/menu`, { signal }),
    enabled,
    refetchInterval: MENU_REFRESH_MS,
    // A phone comes back from the lock screen with a menu that may be an
    // hour old.
    refetchOnWindowFocus: true,
  });
}
