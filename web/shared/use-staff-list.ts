// The venue's staff, as the lock screen, an approval and the back office
// list them. One key and one shape: the list of people, never the response
// around it, so every screen reading the cache reads the same thing.

import { useQuery } from "@tanstack/react-query";
import type { StaffListResponse, StaffResponse } from "./api-types";
import { useAuthStore } from "./auth/use-auth";

export const staffListKey = ["staff"] as const;

export interface StaffListOptions {
  /** "always" reads it afresh each time the screen shows, cached or not. */
  refetchOnMount?: boolean | "always";
  staleTime?: number;
}

export function useStaffList(options: StaffListOptions = {}) {
  const store = useAuthStore();
  return useQuery<StaffResponse[]>({
    queryKey: staffListKey,
    queryFn: ({ signal }) => store.api.get<StaffListResponse>("/api/v1/staff", { signal }).then((r) => r.staff),
    ...options,
  });
}
