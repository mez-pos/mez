// The venue's backup: where it goes and how it is doing
// (backoffice-settings.md, "Backup"; ADR-276). Unlike the other settings it
// is read again while the section is open: how it is doing changes with
// nobody typing.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { BackupRequest, BackupResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

const backupKey = ["settings", "backup"] as const;

/** How often the status is read again while the section is open. */
const STATUS_EVERY_MS = 15_000;

// The server tries a destination before it keeps it (15 seconds for one
// that does not answer) and then stops the old backup (5 more); "back up
// now" is given 30. The client's usual ten would give up on both.
const SAVE_TIMEOUT_MS = 40_000;
const NOW_TIMEOUT_MS = 45_000;

export interface BackupApi {
  data: BackupResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: BackupRequest) => Promise<BackupResponse>;
  backUpNow: () => Promise<BackupResponse>;
}

export function useBackup(): BackupApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: backupKey,
    queryFn: ({ signal }) => store.api.get<BackupResponse>("/api/v1/settings/backup", { signal }),
    refetchInterval: STATUS_EVERY_MS,
  });
  const keep = (next: BackupResponse) => {
    // A read that set out before this was answered would land after it
    // with the backup as it was, and show a save that went through as not
    // saved.
    void queryClient.cancelQueries({ queryKey: backupKey });
    queryClient.setQueryData(backupKey, next);
    return next;
  };
  // It did not go, or was not heard to: how the backup stands has likely
  // changed either way, and is read again now and not in fifteen seconds.
  const lookAgain = (err: unknown): never => {
    void query.refetch();
    throw err;
  };
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) => store.api.put<BackupResponse>("/api/v1/settings/backup", input, { timeoutMs: SAVE_TIMEOUT_MS }).then(keep, lookAgain),
    backUpNow: () => store.api.post<BackupResponse>("/api/v1/settings/backup/now", undefined, { timeoutMs: NOW_TIMEOUT_MS }).then(keep, lookAgain),
  };
}
