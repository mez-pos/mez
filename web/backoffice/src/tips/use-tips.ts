// A shift's tips (docs/design/screens/tips.md): the pool or its preview,
// the staff who can be added, the server's shares for a choice of people,
// and pooling.

import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PoolMember, TipPoolResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useStaffList } from "../../../shared/use-staff-list";

export const tipsKey = (shiftId: number) => ["tips", shiftId] as const;

export function useTips(shiftId: number) {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const pool = useQuery({
    queryKey: tipsKey(shiftId),
    queryFn: ({ signal }) => store.api.get<TipPoolResponse>(`/api/v1/shift/${shiftId}/tips`, { signal }),
  });
  const staff = useStaffList({ staleTime: 60_000 });
  const commit = async (members: PoolMember[]) => {
    const res = await store.api.post<TipPoolResponse>(`/api/v1/shift/${shiftId}/tips`, { members });
    queryClient.setQueryData(tipsKey(shiftId), res);
    return res;
  };
  return { pool, staff, commit };
}

/** The server's shares for these members; off while members is null. */
export function useTipPreview(shiftId: number, members: PoolMember[] | null, enabled: boolean) {
  const store = useAuthStore();
  return useQuery({
    queryKey: [...tipsKey(shiftId), "preview", members],
    queryFn: ({ signal }) => store.api.post<TipPoolResponse>(`/api/v1/shift/${shiftId}/tips/preview`, { members: members ?? [] }, { signal }),
    enabled: enabled && members !== null && members.length > 0,
    placeholderData: keepPreviousData,
    retry: false,
  });
}
