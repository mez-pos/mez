// A station's board (kitchen-display.md). Read again on any event at the
// station (the stream is subscribed to it alone), on a resync, and every 20
// seconds, since a void or a note on a sent line carries no station.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { KitchenBoardResponse, KitchenTicketResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { replaceQuery } from "../../../shared/query";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { useRef } from "react";
import { newKey } from "../register/ticket-key";
import { tenderFailure } from "../tender/tender-logic";

export const boardKey = (station: string) => ["kitchen", station] as const;

/** How often the board is read without being told to. */
const backstopMs = 20_000;

export interface KitchenBoardApi {
  data: KitchenBoardResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  /** A cook has picked it up: in progress (ADR-175). */
  start: (t: KitchenTicketResponse) => Promise<void>;
  bump: (t: KitchenTicketResponse) => Promise<void>;
  recall: (t: KitchenTicketResponse) => Promise<void>;
}

export function useKitchenBoard(station: string): KitchenBoardApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const key = boardKey(station);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => store.api.get<KitchenBoardResponse>(`/api/v1/kitchen/${encodeURIComponent(station)}/tickets`, { signal }),
    staleTime: Infinity,
    refetchInterval: backstopMs,
  });
  useRealtimeNotices((notice) => {
    if (notice.kind === "event" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: key });
  });

  // A move whose answer was lost goes out again under the same key on the
  // next tap, so a bump tapped twice through a wifi blip is one bump.
  const lostKeys = useRef(new Map<string, string>());
  const move = (verb: "start" | "bump" | "recall") => (t: KitchenTicketResponse) => {
    const slot = `${verb}|${t.order_id}|${t.ticket_seq}`;
    const sent = lostKeys.current.get(slot) ?? newKey();
    return store.api
      .post<KitchenBoardResponse>(`/api/v1/kitchen/orders/${encodeURIComponent(t.order_id)}/tickets/${t.ticket_seq}/${verb}`, undefined, {
        idempotencyKey: sent,
      })
      .then((next) => {
        lostKeys.current.delete(slot);
        // The answer is the board as it stands; a backstop read still in
        // flight is older, and is dropped rather than put back over it.
        return replaceQuery(queryClient, key, next);
      })
      .catch((err: unknown) => {
        if (tenderFailure(err) === "uncertain") lostKeys.current.set(slot, sent);
        else lostKeys.current.delete(slot);
        throw err;
      });
  };

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    start: move("start"),
    bump: move("bump"),
    recall: move("recall"),
  };
}
