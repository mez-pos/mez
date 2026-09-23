// React binding for the menu: one cached query, kept fresh by the realtime
// stream rather than by a timer. See menu.ts.

import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import type { MenuResponse } from "./api-types";
import { useAuthStore } from "./auth/use-auth";
import { isMenuStale, menuPath, menuQueryKey } from "./menu";
import { useRealtimeNotices } from "./use-realtime";

/**
 * The venue's menu. Every screen that calls this shares one cached copy;
 * a `menu` notice naming another version invalidates it, and React Query
 * folds the refetches of several callers into one request.
 */
export function useMenu(): UseQueryResult<MenuResponse> {
  const store = useAuthStore();
  const queryClient = useQueryClient();

  useRealtimeNotices((notice) => {
    if (isMenuStale(notice, queryClient.getQueryData<MenuResponse>(menuQueryKey))) {
      void queryClient.invalidateQueries({ queryKey: menuQueryKey });
    }
  });

  return useQuery({
    queryKey: menuQueryKey,
    queryFn: ({ signal }) => store.api.get<MenuResponse>(menuPath, { signal }),
    // Fresh until the stream says otherwise.
    staleTime: Infinity,
  });
}
