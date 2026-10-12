// The venue's link to the relay (docs/design/screens/backoffice-devices.md).
// Read often while it is reconnecting and seldom once it has settled.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { CloudPairRequest, CloudPairResponse, TunnelResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { refreshMs } from "./tunnel-logic";

export const tunnelKey = ["tunnel"] as const;

export interface TunnelApi {
  data: TunnelResponse | undefined;
  loading: boolean;
  error: unknown;
  /** When data was read, which is the "now" its countdown is measured from. */
  readAt: Date;
  refetch: () => void;
  /** Pairs the venue to a mez Cloud account with a code from it (ADR-320). */
  pairToCloud: (code: string) => Promise<CloudPairResponse>;
}

export function useTunnel(): TunnelApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: tunnelKey,
    queryFn: ({ signal }) => store.api.get<TunnelResponse>("/api/v1/tunnel", { signal }),
    refetchInterval: (q) => refreshMs(q.state.data?.state),
  });
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    // Read only with data, which is when it has been read.
    readAt: new Date(query.dataUpdatedAt),
    refetch: () => void query.refetch(),
    pairToCloud: async (code) => {
      const paired = await store.api.post<CloudPairResponse>("/api/v1/cloud/pair", { code } satisfies CloudPairRequest);
      // The link to the outside is another one now, and so, it may be, is
      // where the backup goes: both are read again.
      await queryClient.invalidateQueries({ queryKey: tunnelKey });
      void queryClient.invalidateQueries({ queryKey: ["settings", "backup"] });
      return paired;
    },
  };
}
