// What the status bar reads from the server: the venue it is in, its clock,
// and the server's own version. They are read once; none changes while a
// till is open, and none is worth a request on every screen. The back
// office's Settings, which can change the venue's zone and day start, read
// the venue again after a save (venueKey).

import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import type { VenueNameResponse } from "./api-types";
import { useAuth, useAuthStore } from "./auth/use-auth";
import { venueClock, type VenueClock } from "./venue-clock";

/** Every device's read of the venue, for a save to ask for again. */
export const venueKey = ["venue"] as const;

export function useVenue(): VenueNameResponse | undefined {
  const store = useAuthStore();
  const auth = useAuth();
  // Keyed by the device: pair this browser to another venue without a reload and
  // the old venue's name must not be the one on the bar.
  const device = auth.kind === "signed_in" || auth.kind === "locked" ? auth.device.id : null;
  return useQuery({
    queryKey: [...venueKey, device],
    queryFn: ({ signal }) => store.api.get<VenueNameResponse>("/api/v1/venue", { signal }),
    staleTime: Infinity,
  }).data;
}

/** The venue's clock: its zone and when its trading day begins (ADR-385); the device's zone and 04:00 until it is read. */
export function useVenueClock(): VenueClock {
  const venue = useVenue();
  return useMemo(() => venueClock(venue), [venue]);
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
