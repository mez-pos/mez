// The venue's paired devices (docs/design/screens/backoffice-devices.md). A
// rename or a revoke answers with the list as it now stands, which replaces the
// cache. A device is "last seen" at most once a minute, so the list is read
// again every 30 seconds.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { DeviceListResponse, NewCodeRequest, PairingCodeResponse, RenameDeviceRequest } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

export const devicesKey = ["devices"] as const;

/** How often the list is read again. */
export const REFRESH_MS = 30_000;

export interface DevicesApi {
  data: DeviceListResponse | undefined;
  /** When the list was last read, in epoch milliseconds; 0 before it has been. */
  readAt: number;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** Makes a pairing code: the one time it is shown. */
  pair: (input: NewCodeRequest) => Promise<PairingCodeResponse>;
  rename: (id: string, name: string) => Promise<void>;
  revoke: (id: string) => Promise<void>;
}

export function useDevices(): DevicesApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: devicesKey,
    queryFn: ({ signal }) => store.api.get<DeviceListResponse>("/api/v1/devices", { signal }),
    refetchInterval: REFRESH_MS,
  });
  const keep = (next: DeviceListResponse): void => {
    queryClient.setQueryData(devicesKey, next);
  };

  return {
    data: query.data,
    readAt: query.dataUpdatedAt,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    pair: async (input) => {
      const code = await store.api.post<PairingCodeResponse>("/api/v1/devices/codes", input);
      // The device is now waiting: the list says so.
      await queryClient.invalidateQueries({ queryKey: devicesKey });
      return code;
    },
    rename: (id, name) =>
      store.api.post<DeviceListResponse>(`/api/v1/devices/${encodeURIComponent(id)}/rename`, { name } satisfies RenameDeviceRequest).then(keep),
    revoke: (id) => store.api.post<DeviceListResponse>(`/api/v1/devices/${encodeURIComponent(id)}/revoke`).then(keep),
  };
}
