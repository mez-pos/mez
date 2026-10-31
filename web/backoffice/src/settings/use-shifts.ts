// The shift and tip rules and the receipt template
// (docs/design/screens/backoffice-settings.md). A save answers with its part as
// it now stands, which replaces the cache, so a form shows what the server
// holds and not what was typed.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { ReceiptPreviewRequest, ReceiptPreviewResponse, ReceiptRequest, ReceiptResponse, RulesRequest, RulesResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { venueKey } from "../../../shared/use-venue";

export const rulesKey = ["settings", "rules"] as const;
export const receiptKey = ["settings", "receipt"] as const;

export interface RulesApi {
  data: RulesResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: RulesRequest) => Promise<RulesResponse>;
}

export function useRules(): RulesApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: rulesKey,
    queryFn: ({ signal }) => store.api.get<RulesResponse>("/api/v1/settings/rules", { signal }),
    // A refetch on focus would replace a draft's baseline under the person typing.
    staleTime: Infinity,
  });
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) =>
      store.api.put<RulesResponse>("/api/v1/settings/rules", input).then((next) => {
        queryClient.setQueryData(rulesKey, next);
        // Every screen's "today" starts at the day start (ADR-385).
        void queryClient.invalidateQueries({ queryKey: venueKey });
        return next;
      }),
  };
}

export interface ReceiptApi {
  data: ReceiptResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: ReceiptRequest) => Promise<ReceiptResponse>;
  /** A sample receipt under a draft template at a paper width, saving nothing. */
  preview: (input: ReceiptPreviewRequest, signal: AbortSignal) => Promise<ReceiptPreviewResponse>;
}

export function useReceipt(): ReceiptApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: receiptKey,
    queryFn: ({ signal }) => store.api.get<ReceiptResponse>("/api/v1/settings/receipt", { signal }),
    staleTime: Infinity,
  });
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) =>
      store.api.put<ReceiptResponse>("/api/v1/settings/receipt", input).then((next) => {
        queryClient.setQueryData(receiptKey, next);
        return next;
      }),
    preview: (input, signal) => store.api.post<ReceiptPreviewResponse>("/api/v1/settings/receipt/preview", input, { signal }),
  };
}
