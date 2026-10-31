// What the dashboard reads (D12). Each read is for whoever may make it, and
// is asked again every minute: the dashboard is left open on a laptop in the
// office, and nothing here is worth a live stream of its own.

import { useQuery } from "@tanstack/react-query";
import type { BackupResponse, LiveFloorResponse, SalesReport } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { salesPath } from "../sales/sales-logic";

const everyMinute = 60_000;

/** One trading day's sales report. */
export function useDaySales(day: string, enabled: boolean) {
  const store = useAuthStore();
  return useQuery({
    queryKey: ["dashboard", "sales", day],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath({ from: day, to: day }, "day"), { signal }),
    enabled,
    // Keyed by the day, so when the trading day turns the old day's figures go
    // and are not shown as today's while the new ones are read.
    refetchInterval: everyMinute,
  });
}

/** The floor in service: every table and where it is in its turn. */
/**
 * How the backup stands, for the dashboard's band (BO-11). Settings reads the
 * same key, so a change made there is seen here at once. Needs settings.edit.
 */
export function useBackupStatus(enabled: boolean) {
  const store = useAuthStore();
  return useQuery({
    queryKey: ["settings", "backup"],
    queryFn: ({ signal }) => store.api.get<BackupResponse>("/api/v1/settings/backup", { signal }),
    enabled,
    refetchInterval: everyMinute,
  });
}

export function useFloorNow(enabled: boolean) {
  const store = useAuthStore();
  return useQuery({
    queryKey: ["dashboard", "floor"],
    queryFn: ({ signal }) => store.api.get<LiveFloorResponse>("/api/v1/floor/live", { signal }),
    enabled,
    refetchInterval: everyMinute,
  });
}
