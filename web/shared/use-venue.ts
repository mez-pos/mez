// What the status bar reads from the server: the venue it is in, and the
// server's own version. Both are read once; neither changes while a till is
// open, and neither is worth a request on every screen.

import { useQuery } from "@tanstack/react-query";
import type { VenueNameResponse } from "./api-types";
import { useAuth, useAuthStore } from "./auth/use-auth";

export function useVenue(): VenueNameResponse | undefined {
  const store = useAuthStore();
  const auth = useAuth();
  // Keyed by the device: pair this browser to another venue without a reload and
  // the old venue's name must not be the one on the bar.
  const device = auth.kind === "signed_in" || auth.kind === "locked" ? auth.device.id : null;
  return useQuery({
    queryKey: ["venue", device],
    queryFn: ({ signal }) => store.api.get<VenueNameResponse>("/api/v1/venue", { signal }),
    staleTime: Infinity,
  }).data;
}

/** The server's version string, or null until it has answered. */
export function useServerVersion(): string | null {
  const store = useAuthStore();
  const query = useQuery({
    queryKey: ["server-version"],
    queryFn: ({ signal }) => store.api.get<{ version: string }>("/api/v1/health", { signal }),
    staleTime: Infinity,
  });
  return query.data?.version ?? null;
}
