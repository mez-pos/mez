// What Try again does after a screen failed to draw (TECH-8, ADR-402): the
// data it was drawn from is read again before it is drawn again. Most such
// failures come from what the server sent; drawing the same answer twice
// fails twice.
//
// Data for screens no longer drawn (the failed one among them, since its
// boundary took it down) is dropped, so a screen that mounts again fetches
// it fresh instead of drawing what failed; data still drawn, a ticket held
// above the screen, is fetched again and waited for.

import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

export function useFreshRetry(): () => Promise<void> {
  const client = useQueryClient();
  return useCallback(async () => {
    client.removeQueries({ type: "inactive" });
    await client.refetchQueries({ type: "active" });
  }, [client]);
}
