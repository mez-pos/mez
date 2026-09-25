// The venue's print queue on this till (ADR-081): read once, and again
// whenever the stream says the jobs moved or the till reconnected.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PrintJobsResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { failureOf } from "../register/order-messages";

export const printJobsKey = ["print-jobs"] as const;

export interface PrintJobs {
  jobs: PrintJobsResponse["jobs"];
  loading: boolean;
  error: string | null;
  /** Rejects with the till's words when the server refuses. */
  retry: (id: number) => Promise<void>;
  cancel: (id: number) => Promise<void>;
}

export function usePrintJobs(): PrintJobs {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: printJobsKey,
    queryFn: ({ signal }) => store.api.get<PrintJobsResponse>("/api/v1/print-jobs", { signal }),
    staleTime: Infinity,
  });

  useRealtimeNotices((notice) => {
    if (notice.kind === "print" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: printJobsKey });
  });

  const act = (id: number, what: "retry" | "cancel") =>
    store.api
      .post(`/api/v1/print-jobs/${id}/${what}`)
      .then(() => queryClient.invalidateQueries({ queryKey: printJobsKey }))
      .catch((err: unknown) => {
        throw new Error(failureOf(err), { cause: err });
      });

  return {
    jobs: query.data?.jobs ?? [],
    loading: query.isPending,
    error: query.isError ? failureOf(query.error) : null,
    retry: (id) => act(id, "retry"),
    cancel: (id) => act(id, "cancel"),
  };
}
