// Guests' requests on this terminal (ADR-211, ADR-212): read once, and again
// whenever the stream says the floor changed, which a request arriving or
// being answered is, or the terminal reconnected.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { GuestDecisionResponse, GuestRequestResponse, GuestRequestsResponse, PaidNoticeResponse } from "../../../shared/api-types";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";
import { liveFloorKey } from "../floor/use-live-floor";
import { failureOf } from "../register/order-messages";
import { openTicketsKey } from "../register/use-ticket";

export const guestRequestsKey = ["guest-requests"] as const;

export interface GuestRequests {
  requests: readonly GuestRequestResponse[];
  /** Tables whose guest says they have paid, and nobody has looked (ADR-218). */
  paid: readonly PaidNoticeResponse[];
  /** Undefined until the list has been read once. */
  loaded: boolean;
  error: string | null;
  /** Rejects with the terminal's words when the server refuses. */
  accept: (id: string) => Promise<GuestDecisionResponse>;
  decline: (id: string) => Promise<GuestDecisionResponse>;
  /** Puts a paid notice away: staff looked and no payment had come. */
  seen: (id: string) => Promise<void>;
}

const none: readonly GuestRequestResponse[] = [];
const nonePaid: readonly PaidNoticeResponse[] = [];

export function useGuestRequests(): GuestRequests {
  const store = useAuthStore();
  const auth = useAuth();
  const queryClient = useQueryClient();
  // Answering one is opening a ticket, so it is asked of the same people.
  const may = auth.kind === "signed_in" && auth.permissions.includes("order.open");
  const query = useQuery({
    queryKey: guestRequestsKey,
    queryFn: ({ signal }) => store.api.get<GuestRequestsResponse>("/api/v1/guest-requests", { signal }),
    staleTime: Infinity,
    enabled: may,
  });

  useRealtimeNotices((notice) => {
    if (notice.kind === "floor" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: guestRequestsKey });
  });

  // Whatever the answer, the list is read again: a refusal usually means
  // another terminal answered first, and the list should show that.
  const again = () => {
    void queryClient.invalidateQueries({ queryKey: guestRequestsKey });
    void queryClient.invalidateQueries({ queryKey: liveFloorKey });
    void queryClient.invalidateQueries({ queryKey: openTicketsKey });
  };
  const answer = (id: string, what: "accept" | "decline") =>
    store.api
      .post<GuestDecisionResponse>(`/api/v1/guest-requests/${encodeURIComponent(id)}/${what}`)
      .catch((err: unknown) => {
        throw new Error(failureOf(err), { cause: err });
      })
      .finally(again);

  return {
    requests: query.data?.requests ?? none,
    paid: query.data?.paid ?? nonePaid,
    loaded: query.data !== undefined,
    error: query.isError ? failureOf(query.error) : null,
    accept: (id) => answer(id, "accept"),
    decline: (id) => answer(id, "decline"),
    seen: (id) =>
      store.api
        .post<void>(`/api/v1/paid-notices/${encodeURIComponent(id)}/seen`)
        .catch((err: unknown) => {
          throw new Error(failureOf(err), { cause: err });
        })
        .finally(again),
  };
}
