// Guests' requests on this terminal (ADR-211, ADR-212): read once, and again
// whenever the stream says the floor changed, which a request arriving or
// being answered is, or the terminal reconnected.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  ChargeResolution,
  GuestDecisionResponse,
  GuestRequestResponse,
  GuestRequestsResponse,
  PaidNoticeResponse,
  UnappliedChargeResponse,
} from "../../../shared/api-types";
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
  /** Money a gateway took that no bill could, and nobody has dealt with (ADR-223). */
  unapplied: readonly UnappliedChargeResponse[];
  /** The venue's bills add tax to its prices, so a request's subtotal is before tax (ADR-386). */
  taxAdded: boolean;
  /** Undefined until the list has been read once. */
  loaded: boolean;
  error: string | null;
  /** Rejects with the terminal's words when the server refuses. */
  accept: (id: string) => Promise<GuestDecisionResponse>;
  decline: (id: string) => Promise<GuestDecisionResponse>;
  /** Puts a paid notice away: staff looked and no payment had come. */
  seen: (id: string) => Promise<void>;
  /** Says what was done about an unapplied charge: refunded, or recorded on a bill. */
  resolve: (id: string, how: ChargeResolution) => Promise<void>;
}

const none: readonly GuestRequestResponse[] = [];
const nonePaid: readonly PaidNoticeResponse[] = [];
const noneUnapplied: readonly UnappliedChargeResponse[] = [];

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
    unapplied: query.data?.unapplied ?? noneUnapplied,
    taxAdded: query.data?.tax_added ?? true,
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
    resolve: (id, how) =>
      store.api
        .post<void>(`/api/v1/charges/${encodeURIComponent(id)}/resolve`, { how })
        .catch((err: unknown) => {
          throw new Error(failureOf(err), { cause: err });
        })
        .finally(again),
  };
}
