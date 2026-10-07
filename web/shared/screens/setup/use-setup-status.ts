// Whether the venue is still to be made, and whether this browser is where it
// can be (GET /api/v1/setup, ADR-266). Asked with no token: it is the question
// a device asks before it has one.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import type { SetupStatusResponse } from "../../api-types";
import { useAuthStore } from "../../auth/use-auth";
import { setupHeaders } from "./setup-token";

export const setupStatusKey = ["setup"] as const;

/**
 * `token` is the setup token this tab holds, if any: sent with the question,
 * the answer's `unlocked` says whether it is the one that opens setup.
 */
export function useSetupStatus(token: string | null = null, enabled = true): UseQueryResult<SetupStatusResponse> {
  const store = useAuthStore();
  return useQuery({
    // Not keyed by the token itself: a query key is kept and shown by dev tools.
    queryKey: [...setupStatusKey, token !== null],
    queryFn: ({ signal }) => store.api.get<SetupStatusResponse>("/api/v1/setup", { signal, headers: setupHeaders(token) }),
    enabled,
    // Asked afresh each time a screen that depends on it opens: a tablet left
    // on "not set up yet" must not go on saying so after the owner has finished.
    staleTime: 0,
    gcTime: 0,
  });
}
