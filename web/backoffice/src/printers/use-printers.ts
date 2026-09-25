// The venue's printer settings (backoffice-printers.md). Every change
// answers with the settings as they now stand, which replace the cache; the
// stream's print_changed refreshes each printer's last result.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { AssignRequest, PrinterSettings, PrintersResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";

export const printersKey = ["printers"] as const;

export interface PrinterSettingsApi {
  data: PrintersResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  create: (s: PrinterSettings) => Promise<void>;
  update: (id: number, s: PrinterSettings) => Promise<void>;
  retire: (id: number) => Promise<void>;
  restore: (id: number) => Promise<void>;
  setReceipt: (id: number | null) => Promise<void>;
  setStation: (code: string, id: number | null) => Promise<void>;
  testPage: (id: number) => Promise<void>;
}

export function usePrinters(): PrinterSettingsApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: printersKey,
    queryFn: ({ signal }) => store.api.get<PrintersResponse>("/api/v1/printers", { signal }),
    staleTime: Infinity,
  });
  useRealtimeNotices((notice) => {
    if (notice.kind === "print" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: printersKey });
  });

  const keep = (next: PrintersResponse) => {
    queryClient.setQueryData(printersKey, next);
  };
  const assign = (id: number | null): AssignRequest => ({ printer_id: id });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    create: (s) => store.api.post<PrintersResponse>("/api/v1/printers", s).then(keep),
    update: (id, s) => store.api.put<PrintersResponse>(`/api/v1/printers/${id}`, s).then(keep),
    retire: (id) => store.api.post<PrintersResponse>(`/api/v1/printers/${id}/retire`).then(keep),
    restore: (id) => store.api.post<PrintersResponse>(`/api/v1/printers/${id}/restore`).then(keep),
    setReceipt: (id) => store.api.put<PrintersResponse>("/api/v1/printers/receipt", assign(id)).then(keep),
    setStation: (code, id) =>
      store.api.put<PrintersResponse>(`/api/v1/stations/${encodeURIComponent(code)}/printer`, assign(id)).then(keep),
    testPage: (id) =>
      store.api.post(`/api/v1/printers/${id}/test`).then(() => queryClient.invalidateQueries({ queryKey: printersKey })),
  };
}
