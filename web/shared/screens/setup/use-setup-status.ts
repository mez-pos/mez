// Whether the venue is still to be made, and whether this browser is where it
// can be (GET /api/v1/setup, ADR-266). Asked with no token: it is the question
// a device asks before it has one.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import type { SetupStatusResponse } from "../../api-types";
import { useAuthStore } from "../../auth/use-auth";

export const setupStatusKey = ["setup"] as const;

export function useSetupStatus(enabled = true): UseQueryResult<SetupStatusResponse> {
  const store = useAuthStore();
  return useQuery({
    queryKey: setupStatusKey,
    queryFn: ({ signal }) => store.api.get<SetupStatusResponse>("/api/v1/setup", { signal }),
    enabled,
    // Asked afresh each time a screen that depends on it opens: a tablet left
    // on "not set up yet" must not go on saying so after the owner has finished.
    staleTime: 0,
    gcTime: 0,
  });
}
