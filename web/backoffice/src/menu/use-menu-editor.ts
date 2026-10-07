// The menu as the editor reads and writes it (backoffice-menu.md). Every
// edit answers only an id, so each is followed by a fresh read; the
// stream's menu_changed does the same, so a second back office follows the
// first.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  ChoicesImport,
  MenuCategoryInput,
  MenuEditResponse,
  MenuEditor,
  MenuGroupInput,
  MenuImport,
  MenuPrices,
  MenuItemInput,
  MenuOrderRequest,
  CourseRetiredResponse,
  RetireInput,
  StationInput,
  StationRetiredResponse,
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
  createStation: (input: StationInput) => Promise<number>;
  renameStation: (id: number, name: string) => Promise<void>;
  orderStations: (ids: number[]) => Promise<void>;
  /** Retires a station, its items and printer going to `moveTo` (0 for nowhere). */
  retireStation: (id: number, moveTo: number) => Promise<StationRetiredResponse>;
  restoreStation: (id: number) => Promise<void>;
  createCourse: (name: string) => Promise<number>;
  renameCourse: (id: number, name: string) => Promise<void>;
  orderCourses: (ids: number[]) => Promise<void>;
  retireCourse: (id: number, moveTo: number) => Promise<CourseRetiredResponse>;
  restoreCourse: (id: number) => Promise<void>;
  /** What a CSV file would add to the menu. Writes nothing. */
  previewImport: (file: Blob) => Promise<MenuImport>;
  /** Adds a CSV file's items. `imported` is false when it had nothing new. */
  importMenu: (file: Blob) => Promise<MenuImport>;
  /** The groups of choices a CSV file would add. Writes nothing. */
  previewChoices: (file: Blob) => Promise<ChoicesImport>;
  /** Adds them, offered by no item. `imported` is false when the file had nothing new. */
  importChoices: (file: Blob) => Promise<ChoicesImport>;
  /** The prices a CSV file would change. Writes nothing. */
  previewPrices: (file: Blob) => Promise<MenuPrices>;
  /** Changes them, if the menu is still at the version the preview answered. */
  applyPrices: (file: Blob, version: number) => Promise<MenuPrices>;
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
  // The same, for an edit that answers with more than an id.
  const refreshing = async <T>(send: () => Promise<T>): Promise<T> => {
    try {
      return await send();
    } finally {
      await queryClient.invalidateQueries({ queryKey: menuEditorKey });
    }
  };
  const to = (moveTo: number): RetireInput => ({ move_to: moveTo });
  const name = (n: string): MenuCategoryInput => ({ name: n });
  const order = (ids: number[]): MenuOrderRequest => ({ ids });

  return {
    data: query.data,
    loading: query.isPending,
    error: query.error,
    refetch: () => void query.refetch(),
    createCategory: (n) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/categories", name(n))),
    renameCategory: (id, n) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/categories/${id}`, name(n)))),
    setCategoryActive: (id, active) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/categories/${id}/${active ? "restore" : "retire"}`))),
    orderCategories: (ids) => done(edit(() => store.api.put<MenuEditResponse>("/api/v1/menu/categories/order", order(ids)))),
    createItem: (input) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/items", input)),
    updateItem: (id, input) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${id}`, input))),
    setItemActive: (id, active) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/items/${id}/${active ? "restore" : "retire"}`))),
    orderItems: (categoryId, ids) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/categories/${categoryId}/items/order`, order(ids)))),
    setItemGroups: (itemId, ids) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${itemId}/groups`, order(ids)))),
    createGroup: (input) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/groups", input)),
    updateGroup: (id, input) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/groups/${id}`, input))),
    setGroupActive: (id, active) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/groups/${id}/${active ? "restore" : "retire"}`))),
    setItemSoldOut: (id, soldOut) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/items/${id}/${soldOut ? "sold-out" : "on-sale"}`))),
    // A photo can take longer than a small edit over a venue's wifi.
    setItemPhoto: (id, photo) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/items/${id}/photo`, photo, { timeoutMs: 30_000 }))),
    clearItemPhoto: (id) => done(edit(() => store.api.del<MenuEditResponse>(`/api/v1/menu/items/${id}/photo`))),
    createStation: (input) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/stations", input)),
    renameStation: (id, n) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/stations/${id}`, { code: "", name: n } satisfies StationInput))),
    orderStations: (ids) => done(edit(() => store.api.put<MenuEditResponse>("/api/v1/menu/stations/order", order(ids)))),
    retireStation: (id, moveTo) => refreshing(() => store.api.post<StationRetiredResponse>(`/api/v1/menu/stations/${id}/retire`, to(moveTo))),
    restoreStation: (id) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/stations/${id}/restore`))),
    createCourse: (n) => edit(() => store.api.post<MenuEditResponse>("/api/v1/menu/courses", { name: n })),
    renameCourse: (id, n) => done(edit(() => store.api.put<MenuEditResponse>(`/api/v1/menu/courses/${id}`, { name: n }))),
    orderCourses: (ids) => done(edit(() => store.api.put<MenuEditResponse>("/api/v1/menu/courses/order", order(ids)))),
    retireCourse: (id, moveTo) => refreshing(() => store.api.post<CourseRetiredResponse>(`/api/v1/menu/courses/${id}/retire`, to(moveTo))),
    restoreCourse: (id) => done(edit(() => store.api.post<MenuEditResponse>(`/api/v1/menu/courses/${id}/restore`))),
    // A whole menu can take longer than a small edit over a venue's wifi.
    previewImport: (file) => store.api.post<MenuImport>("/api/v1/menu/import/preview", file, { timeoutMs: 30_000 }),
    importMenu: (file) => refreshing(() => store.api.post<MenuImport>("/api/v1/menu/import", file, { timeoutMs: 30_000 })),
    previewChoices: (file) => store.api.post<ChoicesImport>("/api/v1/menu/choices/import/preview", file, { timeoutMs: 30_000 }),
    importChoices: (file) => refreshing(() => store.api.post<ChoicesImport>("/api/v1/menu/choices/import", file, { timeoutMs: 30_000 })),
    previewPrices: (file) => store.api.post<MenuPrices>("/api/v1/menu/prices/preview", file, { timeoutMs: 30_000 }),
    applyPrices: (file, version) => refreshing(() => store.api.post<MenuPrices>(`/api/v1/menu/prices?version=${version}`, file, { timeoutMs: 30_000 })),
  };
}
