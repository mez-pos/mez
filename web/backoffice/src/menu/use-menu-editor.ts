// The menu as the editor reads and writes it (backoffice-menu.md). Every
// edit answers only an id, so each is followed by a fresh read; the
// stream's menu_changed does the same, so a second back office follows the
// first.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  MenuCategoryInput,
  MenuEditResponse,
  MenuEditor,
  MenuGroupInput,
  MenuItemInput,
  MenuOrderRequest,
} from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { useRealtimeNotices } from "../../../shared/use-realtime";

export const menuEditorKey = ["menu-editor"] as const;

export interface MenuEditorApi {
  data: MenuEditor | undefined;
  loading: boolean;
  error: unknown;
  refetch: () => void;
  createCategory: (name: string) => Promise<number>;
  renameCategory: (id: number, name: string) => Promise<void>;
  setCategoryActive: (id: number, active: boolean) => Promise<void>;
  orderCategories: (ids: number[]) => Promise<void>;
  createItem: (input: MenuItemInput) => Promise<number>;
  updateItem: (id: number, input: MenuItemInput) => Promise<void>;
  setItemActive: (id: number, active: boolean) => Promise<void>;
  orderItems: (categoryId: number, ids: number[]) => Promise<void>;
  setItemGroups: (itemId: number, ids: number[]) => Promise<void>;
  createGroup: (input: MenuGroupInput) => Promise<number>;
  updateGroup: (id: number, input: MenuGroupInput) => Promise<void>;
  setGroupActive: (id: number, active: boolean) => Promise<void>;
  setItemSoldOut: (id: number, soldOut: boolean) => Promise<void>;
  setItemPhoto: (id: number, photo: Blob) => Promise<void>;
  clearItemPhoto: (id: number) => Promise<void>;
}

export function useMenuEditor(): MenuEditorApi {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: menuEditorKey,
    queryFn: ({ signal }) => store.api.get<MenuEditor>("/api/v1/menu/editor", { signal }),
    staleTime: Infinity,
  });
  useRealtimeNotices((notice) => {
    if (notice.kind === "menu" || notice.kind === "resync") void queryClient.invalidateQueries({ queryKey: menuEditorKey });
  });

  // Reads the menu again whether the edit landed or not: a refusal such as
  // a stale order means the screen's copy is behind.
  const edit = async (send: () => Promise<MenuEditResponse>): Promise<number> => {
    try {
      return (await send()).id ?? 0;
    } finally {
      await queryClient.invalidateQueries({ queryKey: menuEditorKey });
    }
  };
  const done = (p: Promise<number>) => p.then(() => undefined);
  const name = (n: string): MenuCategoryInput => ({ name: n });
  const order = (ids: number[]): MenuOrderRequest => ({ ids });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    createCategory: (n) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/categories", name(n))),
    renameCategory: (id, n) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/categories/${id}`, name(n)))),
    setCategoryActive: (id, active) =>
      done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/categories/${id}/${active ? "restore" : "retire"}`))),
    orderCategories: (ids) => done(edit(() => store.api.put<MenuEditResponse>("/api/v1/menu/categories/order", order(ids)))),
    createItem: (input) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/items", input)),
    updateItem: (id, input) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${id}`, input))),
    setItemActive: (id, active) =>
      done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/items/${id}/${active ? "restore" : "retire"}`))),
    orderItems: (categoryId, ids) =>
      done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/categories/${categoryId}/items/order`, order(ids)))),
    setItemGroups: (itemId, ids) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${itemId}/groups`, order(ids)))),
    createGroup: (input) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/groups", input)),
    updateGroup: (id, input) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/groups/${id}`, input))),
    setGroupActive: (id, active) =>
      done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/groups/${id}/${active ? "restore" : "retire"}`))),
    setItemSoldOut: (id, soldOut) =>
      done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/items/${id}/${soldOut ? "sold-out" : "on-sale"}`))),
    // A photo can take longer than a small edit over a venue's wifi.
    setItemPhoto: (id, photo) =>
      done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${id}/photo`, photo, { timeoutMs: 30_000 }))),
    clearItemPhoto: (id) => done(edit(() => store.api.del<MenuEditResponse>(`/api/v1/menu/items/${id}/photo`))),
  };
}
