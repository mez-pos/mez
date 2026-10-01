// The venue's stock as the Stock screen reads and writes it
// (backoffice-stock.md). Every write reads the list again: on hand is the
// server's sum of the ledger, never worked out here.

import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  MovementsResponse,
  RecipeBook,
  RecipeRequest,
  StockEditResponse,
  StockItemInput,
  StockItemsResponse,
  StockMovementInput,
  StockRecorded,
} from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";

export const stockKey = ["stock"] as const;
export const recipesKey = ["stock-recipes"] as const;
const historyKey = (id: number) => ["stock-history", id] as const;

export interface StockApi {
  items: ReturnType<typeof useQuery<StockItemsResponse>>;
  recipes: ReturnType<typeof useQuery<RecipeBook>>;
  createItem: (input: StockItemInput) => Promise<number>;
  updateItem: (id: number, input: StockItemInput) => Promise<void>;
  setActive: (id: number, active: boolean) => Promise<void>;
  record: (id: number, input: StockMovementInput) => Promise<StockRecorded>;
  setItemRecipe: (itemId: number, req: RecipeRequest) => Promise<void>;
  setModifierRecipe: (modifierId: number, req: RecipeRequest) => Promise<void>;
}

export function useStock(): StockApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const items = useQuery({
    queryKey: stockKey,
    queryFn: ({ signal }) => store.api.get<StockItemsResponse>("/api/v1/stock", { signal }),
  });
  const recipes = useQuery({
    queryKey: recipesKey,
    queryFn: ({ signal }) => store.api.get<RecipeBook>("/api/v1/stock/recipes", { signal }),
  });

  // Reads again whether the write landed or not: a refusal can mean the
  // screen's copy is behind.
  const after = async <T>(p: Promise<T>, also: readonly unknown[][] = []): Promise<T> => {
    try {
      return await p;
    } finally {
      await Promise.all([stockKey, ...also].map((queryKey) => queryClient.invalidateQueries({ queryKey })));
    }
  };

  return {
    items,
    recipes,
    createItem: (input) => after(store.api.post<StockEditResponse>("/api/v1/stock/items", input)).then((r) => r.id),
    updateItem: (id, input) => after(store.api.put<StockEditResponse>(`/api/v1/stock/items/${id}`, input)).then(() => undefined),
    setActive: (id, active) =>
      after(store.api.post<StockEditResponse>(`/api/v1/stock/items/${id}/${active ? "restore" : "retire"}`)).then(() => undefined),
    record: (id, input) => after(store.api.post<StockRecorded>(`/api/v1/stock/items/${id}/movements`, input), [[...historyKey(id)]]),
    setItemRecipe: (itemId, req) =>
      after(store.api.put<StockEditResponse>(`/api/v1/stock/recipes/items/${itemId}`, req), [[...recipesKey]]).then(() => undefined),
    setModifierRecipe: (modifierId, req) =>
      after(store.api.put<StockEditResponse>(`/api/v1/stock/recipes/modifiers/${modifierId}`, req), [[...recipesKey]]).then(() => undefined),
  };
}

/** The stock list alone, for the dashboard's low-stock card. Same key as the screen's, so they agree. */
export function useStockItems(enabled: boolean) {
  const store = useAuthStore();
  return useQuery({
    queryKey: stockKey,
    queryFn: ({ signal }) => store.api.get<StockItemsResponse>("/api/v1/stock", { signal }),
    enabled,
  });
}

/** An item's history, a page at a time, newest first. */
export function useHistory(id: number) {
  const store = useAuthStore();
  return useInfiniteQuery({
    queryKey: historyKey(id),
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      store.api.get<MovementsResponse>(`/api/v1/stock/items/${id}/movements?limit=25${pageParam ? `&before=${pageParam}` : ""}`, { signal }),
    getNextPageParam: (last) => last.next_before,
  });
}
