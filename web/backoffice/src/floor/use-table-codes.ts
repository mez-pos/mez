// The codes on the tables (backoffice-floor.md, ADR-243). Every change
// answers with the whole list as it now stands, which replaces the cache.
// The list follows the floor: a table added or retired changes it.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { TableCodeQRResponse, TableCodeSheetResponse, TableCodesResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";

export const tableCodesKey = ["table-codes"] as const;

export interface TableCodesApi {
  data: TableCodesResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** Gives each table without a code one, and keeps the rest. */
  makeMissing: () => Promise<void>;
  renew: (tableId: number) => Promise<void>;
  revoke: (tableId: number) => Promise<void>;
  /** A table's address drawn as a QR code. Never kept: a renew changes it. */
  qr: (tableId: number) => Promise<TableCodeQRResponse>;
  /** Every table's code drawn, to print. Never kept, for the same reason. */
  sheet: () => Promise<TableCodeSheetResponse>;
}

export function useTableCodes(): TableCodesApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: tableCodesKey,
    queryFn: ({ signal }) => store.api.get<TableCodesResponse>("/api/v1/table-codes", { signal }),
    // Credentials: read when the screen opens, never shown from an earlier visit.
    staleTime: 0,
    gcTime: 0,
  });
  useRealtimeNotices((notice) => {
    if (notice.kind === "floor" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: tableCodesKey });
  });

  const keep = (next: TableCodesResponse) => {
    queryClient.setQueryData(tableCodesKey, next);
  };
  const post = (path: string) =>
    store.api.post<TableCodesResponse>(path).then(keep, (err: unknown) => {
      // Refused because the floor moved under it: the list says how it stands.
      void queryClient.invalidateQueries({ queryKey: tableCodesKey });
      throw err;
    });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    makeMissing: () => post("/api/v1/table-codes"),
    renew: (id) => post(`/api/v1/tables/${id}/code/renew`),
    revoke: (id) => post(`/api/v1/tables/${id}/code/revoke`),
    qr: (id) => store.api.get<TableCodeQRResponse>(`/api/v1/tables/${id}/code/qr`),
    sheet: () => store.api.get<TableCodeSheetResponse>("/api/v1/table-codes/sheet"),
  };
}
