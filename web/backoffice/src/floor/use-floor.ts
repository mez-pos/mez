// The venue's floor plan (backoffice-floor.md). Every change answers with
// the whole plan as it now stands, which replaces the cache; the stream's
// floor_changed refreshes it when another device changed it.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { AreaSettings, FloorResponse, TableSettings } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";

export const floorKey = ["floor"] as const;

export interface FloorApi {
  data: FloorResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  createArea: (s: AreaSettings) => Promise<void>;
  renameArea: (id: number, s: AreaSettings) => Promise<void>;
  retireArea: (id: number) => Promise<void>;
  restoreArea: (id: number) => Promise<void>;
  createTable: (s: TableSettings) => Promise<void>;
  updateTable: (id: number, s: TableSettings) => Promise<void>;
  retireTable: (id: number) => Promise<void>;
  restoreTable: (id: number) => Promise<void>;
}

export function useFloor(): FloorApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: floorKey,
    queryFn: ({ signal }) => store.api.get<FloorResponse>("/api/v1/floor", { signal }),
    staleTime: Infinity,
  });
  useRealtimeNotices((notice) => {
    if (notice.kind === "floor" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: floorKey });
  });

  const keep = (next: FloorResponse) => {
    queryClient.setQueryData(floorKey, next);
  };
  const post = (path: string, body?: unknown) => store.api.post<FloorResponse>(path, body).then(keep);
  const put = (path: string, body: unknown) => store.api.put<FloorResponse>(path, body).then(keep);

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    createArea: (s) => post("/api/v1/floor/areas", s),
    renameArea: (id, s) => put(`/api/v1/floor/areas/${id}`, s),
    retireArea: (id) => post(`/api/v1/floor/areas/${id}/retire`),
    restoreArea: (id) => post(`/api/v1/floor/areas/${id}/restore`),
    createTable: (s) => post("/api/v1/floor/tables", s),
    updateTable: (id, s) => put(`/api/v1/floor/tables/${id}`, s),
    retireTable: (id) => post(`/api/v1/floor/tables/${id}/retire`),
    restoreTable: (id) => post(`/api/v1/floor/tables/${id}/restore`),
  };
}
