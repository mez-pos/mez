// Where the venue is paid from a guest's phone (ADR-215): read once, and
// replaced by what the server answers a save with.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PaymentSettingsRequest, PaymentSettingsResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

const paymentsKey = ["settings", "payments"] as const;

export interface PaymentsApi {
  data: PaymentSettingsResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: PaymentSettingsRequest) => Promise<PaymentSettingsResponse>;
}

export function usePayments(): PaymentsApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: paymentsKey,
    queryFn: ({ signal }) => store.api.get<PaymentSettingsResponse>("/api/v1/settings/payments", { signal }),
    staleTime: Infinity,
  });
  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) =>
      store.api.put<PaymentSettingsResponse>("/api/v1/settings/payments", input).then((next) => {
        queryClient.setQueryData(paymentsKey, next);
        return next;
      }),
  };
}
