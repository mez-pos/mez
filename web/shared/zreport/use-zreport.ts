// The venue's Z-reports (docs/design/screens/zreport.md). A report never
// changes once issued, so one is read once and kept.

import { useQuery } from "@tanstack/react-query";
import type { ZReportResponse, ZReportsResponse } from "../api-types";
import { ApiError } from "../api";
import { useAuthStore } from "../auth/use-auth";

export const zReportKey = (number: number) => ["z-report", number] as const;
export const zReportsKey = (before: number) => ["z-reports", before] as const;

/** The HTTP status of a failed read, when the server answered. */
export function statusOf(err: unknown): number | undefined {
  return err instanceof ApiError && err.status !== null ? err.status : undefined;
}

export function useZReport(number: number) {
  const store = useAuthStore();
  return useQuery({
    queryKey: zReportKey(number),
    queryFn: ({ signal }) => store.api.get<ZReportResponse>(`/api/v1/z-reports/${number}`, { signal }),
    staleTime: Infinity,
    retry: (count, err) => statusOf(err) === undefined && count < 2,
  });
}

/** A page of reports, newest first, below `before` (0 for the newest). */
export function useZReports(before: number) {
  const store = useAuthStore();
  return useQuery({
    queryKey: zReportsKey(before),
    queryFn: ({ signal }) =>
      store.api.get<ZReportsResponse>(before > 0 ? `/api/v1/z-reports?before=${before}` : "/api/v1/z-reports", { signal }),
    staleTime: 30_000,
  });
}

/** Queues a copy, marked COPY, on the report printer. */
export function usePrintZReport(): (number: number) => Promise<void> {
  const store = useAuthStore();
  return (number) => store.api.post<void>(`/api/v1/z-reports/${number}/print`);
}
