// The venue's link to the relay (docs/design/screens/backoffice-devices.md).
// Read often while it is reconnecting and seldom once it has settled.

import { useQuery } from "@tanstack/react-query";
import type { TunnelResponse } from "../../../shared/api-types";
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
}

export function useTunnel(): TunnelApi {
  const store = useAuthStore();
  const query = useQuery({
    queryKey: tunnelKey,
    queryFn: ({ signal }) => store.api.get<TunnelResponse>("/api/v1/tunnel", { signal }),
    refetchInterval: (q) => refreshMs(q.state.data?.state),
  });
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    readAt: new Date(query.dataUpdatedAt || Date.now()),
    refetch: () => void query.refetch(),
  };
}
