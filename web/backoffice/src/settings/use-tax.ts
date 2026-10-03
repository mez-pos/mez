// The venue's tax profile, tax classes and invoice numbering
// (docs/design/screens/backoffice-settings.md). Every change answers with its
// part as it now stands, which replaces the cache, so a form shows what the
// server holds and not what was typed. A tax class change also moves the menu
// version: the stream tells the menu editor, which reads again.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { NumberingRequest, NumberingResponse, TaxClassRequest, TaxSettingsRequest, TaxSettingsResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

export const taxKey = ["settings", "tax"] as const;
export const numberingKey = ["settings", "numbering"] as const;

export interface TaxApi {
  data: TaxSettingsResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: TaxSettingsRequest) => Promise<TaxSettingsResponse>;
  createClass: (input: TaxClassRequest) => Promise<void>;
  updateClass: (id: number, input: TaxClassRequest) => Promise<void>;
  retireClass: (id: number) => Promise<void>;
  restoreClass: (id: number) => Promise<void>;
}

export function useTax(): TaxApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: taxKey,
    queryFn: ({ signal }) => store.api.get<TaxSettingsResponse>("/api/v1/settings/tax", { signal }),
    // A refetch on focus would replace a draft's baseline under the person typing.
    staleTime: Infinity,
  });
  const keep = (next: TaxSettingsResponse): TaxSettingsResponse => {
    queryClient.setQueryData(taxKey, next);
    return next;
  };

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) => store.api.put<TaxSettingsResponse>("/api/v1/settings/tax", input).then(keep),
    createClass: (input) =>
      store.api
        .post<TaxSettingsResponse>("/api/v1/settings/tax-classes", input)
        .then(keep)
        .then(() => undefined),
    updateClass: (id, input) =>
      store.api
        .put<TaxSettingsResponse>(`/api/v1/settings/tax-classes/${id}`, input)
        .then(keep)
        .then(() => undefined),
    retireClass: (id) =>
      store.api
        .post<TaxSettingsResponse>(`/api/v1/settings/tax-classes/${id}/retire`)
        .then(keep)
        .then(() => undefined),
    restoreClass: (id) =>
      store.api
        .post<TaxSettingsResponse>(`/api/v1/settings/tax-classes/${id}/restore`)
        .then(keep)
        .then(() => undefined),
  };
}

export interface NumberingApi {
  data: NumberingResponse | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  save: (input: NumberingRequest) => Promise<NumberingResponse>;
  /** The next numbers a draft would write, saving nothing. */
  preview: (input: NumberingRequest, signal: AbortSignal) => Promise<NumberingResponse>;
}

export function useNumbering(): NumberingApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: numberingKey,
    queryFn: ({ signal }) => store.api.get<NumberingResponse>("/api/v1/settings/numbering", { signal }),
    staleTime: Infinity,
  });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    save: (input) =>
      store.api.put<NumberingResponse>("/api/v1/settings/numbering", input).then((next) => {
        queryClient.setQueryData(numberingKey, next);
        return next;
      }),
    preview: (input, signal) => store.api.post<NumberingResponse>("/api/v1/settings/numbering/preview", input, { signal }),
  };
}
