// The venue's settings (docs/design/screens/backoffice-settings.md). A save
// answers with the settings as they now stand, which replace the cache, so the
// form shows what the server holds and not what was typed.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { SettingsResponse, SettingsVenueResponse, VenueDetailsRequest } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { venueKey } from "../../../shared/use-venue";

export const settingsKey = ["settings"] as const;

export interface SettingsApi {
  data: SettingsResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** Saves the venue's details and resolves with the venue as the server now holds it. */
  saveVenue: (input: VenueDetailsRequest) => Promise<SettingsVenueResponse>;
}

export function useSettings(): SettingsApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: settingsKey,
    queryFn: ({ signal }) => store.api.get<SettingsResponse>("/api/v1/settings", { signal }),
    // Nothing else changes these behind the screen's back, and a refetch on
    // focus would replace a draft's baseline under the person typing.
    staleTime: Infinity,
  });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    saveVenue: async (input) => {
      const next = await store.api.put<SettingsResponse>("/api/v1/settings/venue", input);
      queryClient.setQueryData(settingsKey, next);
      // The bar's clock reads the venue's zone.
      void queryClient.invalidateQueries({ queryKey: venueKey });
      return next.venue;
    },
  };
}
