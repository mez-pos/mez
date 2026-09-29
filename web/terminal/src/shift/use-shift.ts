// The venue's shift, and what the till does to it (terminal-close.md). The
// stream carries no shifts, so the shift is read on start, every 30
// seconds, and from every command's answer. Every command carries an
// Idempotency-Key kept across retries of the same body.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { CountRequest, CurrentShiftResponse, ShiftResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useAttemptKeys } from "../bills/use-bills";
import { failureOf } from "../register/order-messages";

export const shiftKey = ["shift"] as const;

export interface ShiftCommands {
  open: (float: CountRequest) => Promise<ShiftResponse>;
  count: (shiftId: number, counted: CountRequest) => Promise<ShiftResponse>;
  explain: (shiftId: number, reason: string) => Promise<ShiftResponse>;
  close: (shiftId: number) => Promise<ShiftResponse>;
}

export interface ShiftState {
  /** The open shift, null when the till is closed, undefined until known. */
  shift: ShiftResponse | null | undefined;
  error: unknown;
  retry: () => void;
  commands: ShiftCommands;
}

export function useShift(): ShiftState {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const keys = useAttemptKeys();

  const query = useQuery({
    queryKey: shiftKey,
    queryFn: ({ signal }) => store.api.get<CurrentShiftResponse>("/api/v1/shift", { signal }),
    refetchInterval: 30_000,
  });

  const post = async (path: string, body: unknown, closes = false): Promise<ShiftResponse> => {
    try {
      const res = await store.api.post<ShiftResponse>(path, body, { idempotencyKey: keys.keyFor({ path, body }) });
      keys.done();
      queryClient.setQueryData<CurrentShiftResponse>(shiftKey, { shift: closes ? null : res });
      return res;
    } catch (err) {
      // Another till may have moved the shift on: read it again either way.
      void queryClient.invalidateQueries({ queryKey: shiftKey });
      throw new Error(failureOf(err), { cause: err });
    }
  };

  return {
    shift: query.data?.shift,
    error: query.error,
    retry: () => void query.refetch(),
    commands: {
      open: (float) => post("/api/v1/shift/open", float),
      count: (id, counted) => post(`/api/v1/shift/${id}/count`, counted),
      explain: (id, reason) => post(`/api/v1/shift/${id}/reason`, { reason }),
      close: (id) => post(`/api/v1/shift/${id}/close`, {}, true),
    },
  };
}
