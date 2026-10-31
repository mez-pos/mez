// The menu editor (D18): the categories, the items of the one chosen, and
// the item open in its editor, side by side. Every change reaches the order
// screens at once; orders already open keep what they were rung up at.
//
// The design's one screen is the Items view. The back office also has the
// groups of choices and the stations and courses (rebuild.md, rule 4): they
// are the two other views, as pills under the title.

import { PlusIcon, SearchIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { EditorGroup, MenuEditor } from "../../../shared/api-types";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { Button, Card, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { useClock } from "../../../shared/use-clock";
import { useOnMirror } from "../../../shared/use-mirror";
import { inputClass } from "../FormFields";
import { useCsvFile } from "../sales/use-csv-download";
import { itemState, showOptions, validScope, type ListFilter, type Scope, type Show } from "./browse-logic";
import { CategoryFormModal } from "./CategoryPanel";
import { CategoryRail } from "./CategoryRail";
import { ChoicesImportModal } from "./ChoicesImportModal";
import { emptyGroupForm, groupFormFrom, groupsChanged, usedBy } from "./group-logic";
import { GroupFormModal } from "./GroupFormModal";
import { GroupsView } from "./GroupsView";
import { ImportModal } from "./ImportModal";
import { PricesModal } from "./PricesModal";
import { SpreadsheetMenu } from "./SpreadsheetMenu";
import { ItemEditor } from "./ItemEditor";
import { ItemList } from "./ItemList";
import type { PhotoChange } from "./ItemPhotoField";
import { itemFormFrom, itemFromSearch, newItemForm, problemOf, viewFromSearch, viewWords, type MenuView } from "./menu-logic";
import { SetupView } from "./SetupView";
import { useMenuEditor, type MenuEditorApi } from "./use-menu-editor";
import { useRetireItem } from "./use-retire-item";
import { useLeave } from "../shell/leave-guard";

type Editing =
  { kind: "add-category" } | { kind: "rename-category"; id: number; name: string } | { kind: "add-group" } | { kind: "edit-group"; group: EditorGroup };

/** What the editor has open: a saved item, a new one in a category, or nothing. */
type Open = { kind: "item"; id: number } | { kind: "new"; categoryId: number; nth: number } | null;

/** Does what the item form asked for its photo, after the item is saved. */
function applyPhoto(api: MenuEditorApi, id: number, change: PhotoChange): Promise<void> {
  switch (change.kind) {
    case "keep":
      return Promise.resolve();
    case "new":
      return api.setItemPhoto(id, change.blob);
    case "remove":
      return api.clearItemPhoto(id);
  }
}

export function MenuScreen() {
  const api = useMenuEditor();
  const toast = useToast();
  // A mirror shows the menu as the backup has it, and changes none of it (ADR-330).
  const mirror = useOnMirror();
  const now = new Date(useClock(60_000));
  const [view, setView] = useState<MenuView>(() => viewFromSearch(window.location.search));
  const [filter, setFilter] = useState<ListFilter>({ scope: "all", search: "", show: "all", order: "menu" });
  const [open, setOpen] = useState<Open>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [filing, setFiling] = useState<"import" | "choices" | "prices" | null>(null);
  const exported = useCsvFile();
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const made = useRef(0);
  const menu = api.data;
  // Whatever replaces the open editor asks first while it holds a draft (BO-2).
  const leave = useLeave();

  // A link that names an item (`/menu?item=12`, from the tax settings) opens
  // it once, as soon as the menu is read.
  const asked = useRef(false);
  useEffect(() => {
    if (!menu || asked.current) return;
    asked.current = true;
    const item = itemFromSearch(window.location.search, menu);
    if (item) {
      setFilter((f) => ({ ...f, scope: item.category_id }));
      setOpen({ kind: "item", id: item.id });
    }
  }, [menu]);

  const scope = menu ? validScope(menu, filter.scope) : filter.scope;
  const shown: ListFilter = { ...filter, scope };
  const item = menu && open?.kind === "item" ? (menu.items.find((i) => i.id === open.id) ?? null) : null;
  const newIn = menu && open?.kind === "new" ? (menu.categories.find((c) => c.id === open.categoryId) ?? menu.categories[0] ?? null) : null;

  const pick = (patch: Partial<ListFilter>) => {
    setFilter((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  // A refusal is shown over the columns and as a toast: the list can be a
  // screen tall, and the line over it out of sight.
  const refused = (message: string) => {
    setProblem(message);
    toast(message, { tone: "warning" });
  };
  const retire = useRetireItem(api, refused);
  const act = (fn: () => Promise<void>) => {
    setBusy(true);
    setProblem(null);
    fn()
      .catch((err: unknown) => refused(problemOf(err).message))
      .finally(() => setBusy(false));
  };
  const addItem = (m: MenuEditor) => {
    // In the category shown, or the first active one from All items or Popular.
    const into = typeof scope === "number" ? scope : (m.categories.find((c) => c.active)?.id ?? m.categories[0]?.id);
    if (into === undefined) {
      setEditing({ kind: "add-category" });
      return;
    }
    made.current += 1;
    setOpen({ kind: "new", categoryId: into, nth: made.current });
    setProblem(null);
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Menu Management</h1>
          <p className="mt-1 text-base text-fg-muted">
            {mirror
              ? "The items, categories and their choices, as the venue last backed them up."
              : "Create and manage items, categories and their choices. Changes reach every order screen at once."}
          </p>
        </div>
        {view === "items" ? (
          <div className="flex flex-wrap items-center gap-2.5">
            <label className="relative block">
              <span className="sr-only">Search menu items</span>
              <SearchIcon aria-hidden="true" size={18} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-fg-muted" />
              <input
                type="search"
                className={cx(inputClass, "w-72 max-w-full rounded-pill pl-11")}
                placeholder="Search menu items"
                value={filter.search}
                onChange={(e) => pick({ search: e.target.value })}
              />
            </label>
            <div className="w-40 shrink-0">
              <select
                aria-label="Show"
                className={cx(inputClass, "rounded-pill font-semibold")}
                value={filter.show}
                onChange={(e) => pick({ show: e.target.value as Show })}
              >
                {showOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <SpreadsheetMenu
              exported={exported}
              disabled={!menu}
              readOnly={mirror}
              onImport={() => setFiling("import")}
              onChoices={() => setFiling("choices")}
              onPrices={() => setFiling("prices")}
            />
            {!mirror && (
              <Button variant="attention" disabled={!menu} onClick={() => menu && leave(() => addItem(menu))}>
                <PlusIcon aria-hidden="true" size={18} />
                Add New Item
              </Button>
            )}
          </div>
        ) : mirror ? null : view === "choices" ? (
          <Button variant="attention" disabled={!menu} onClick={() => setEditing({ kind: "add-group" })}>
            <PlusIcon aria-hidden="true" size={18} />
            Add a group of choices
          </Button>
        ) : null}
      </div>

      {mirror && <OnAMirror />}
      <div role="group" aria-label="Show" className="flex flex-wrap gap-2">
        {(["items", "choices", "setup"] as const).map((v) => (
          <button
            key={v}
            type="button"
            aria-pressed={view === v}
            className={pillClass(view === v)}
            onClick={() => {
              if (v === view) return;
              leave(() => {
                setView(v);
                setProblem(null);
              });
            }}
          >
            {viewWords[v]}
          </button>
        ))}
      </div>

      {problem && (
        <p role="alert" className="text-base text-danger">
          {problem}
        </p>
      )}
      {api.loading && <p className="text-base text-fg-muted">Loading the menu</p>}
      {!api.loading && api.error !== null && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
      )}
      {menu && view === "choices" && (
        <div className="max-w-6xl">
          <GroupsView menu={menu} api={api} onEdit={(group) => setEditing({ kind: "edit-group", group })} onProblem={setProblem} />
        </div>
      )}
      {menu && view === "setup" && (
        <div className="max-w-6xl">
          <SetupView menu={menu} api={api} onProblem={setProblem} />
        </div>
      )}
      {menu && view === "items" && (
        // The categories take the sidebar width and the list the back office's side
        // width (tokens.md), so the editor is what gives as the screen narrows.
        <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[var(--side)_minmax(0,1fr)] xl:grid-cols-[var(--side)_var(--aside)_minmax(0,1fr)]">
          <CategoryRail
            menu={menu}
            scope={scope}
            onScope={(s: Scope) => pick({ scope: s, search: "" })}
            api={api}
            onAdd={() => setEditing({ kind: "add-category" })}
            onRename={(id, name) => setEditing({ kind: "rename-category", id, name })}
            onProblem={refused}
          />
          <ItemList
            menu={menu}
            filter={shown}
            onOrder={(order) => pick({ order })}
            openId={item?.id ?? null}
            onOpen={(i) => {
              if (open?.kind === "item" && open.id === i.id) return;
              leave(() => {
                setOpen({ kind: "item", id: i.id });
                setProblem(null);
              });
            }}
            api={api}
            onProblem={refused}
          />
          <div className="min-w-0 lg:col-span-2 xl:col-span-1">
            {item ? (
              <ItemEditor
                // A different item is a different form; the same item saved again keeps what is typed.
                key={item.id}
                menu={menu}
                item={item}
                state={itemState(item, now)}
                soldOutUntil={item.sold_out_until ?? null}
                initial={itemFormFrom(item)}
                busy={busy}
                onCancel={() => setOpen(null)}
                onSoldOut={(soldOut) => act(() => api.setItemSoldOut(item.id, soldOut))}
                onActive={(active) => act(() => (active ? api.setItemActive(item.id, true) : retire(item)))}
                onSave={async (input, groups, photo) => {
                  await api.updateItem(item.id, input);
                  if (groupsChanged(item, groups)) await api.setItemGroups(item.id, groups);
                  await applyPhoto(api, item.id, photo);
                  // Moved to another category: the list follows, so the open item stays in it.
                  if (typeof scope === "number" && scope !== input.category_id) pick({ scope: input.category_id });
                  const repriced = input.price.minor !== item.price.minor;
                  toast(repriced ? "Saved. Orders already open keep their prices." : "Saved.", { tone: "success" });
                }}
              />
            ) : newIn && open?.kind === "new" ? (
              <ItemEditor
                key={`new-${open.nth}`}
                menu={menu}
                item={null}
                state={null}
                soldOutUntil={null}
                initial={newItemForm(menu, newIn.id)}
                busy={busy}
                onCancel={() => setOpen(null)}
                onSoldOut={() => undefined}
                onActive={() => undefined}
                onSave={async (input, groups, photo) => {
                  const id = await api.createItem(input);
                  // The item is saved now. Its choices and photo are written before
                  // it is opened as itself, so the editor starts from what was kept
                  // and a later Save cannot clear choices it never saw. A failure
                  // here still opens it: Save on this form again would add it twice.
                  const missing: string[] = [];
                  if (groups.length > 0) await api.setItemGroups(id, groups).catch(() => missing.push("its choices"));
                  await applyPhoto(api, id, photo).catch(() => missing.push("its photo"));
                  setOpen({ kind: "item", id });
                  pick({ scope: input.category_id, search: "" });
                  if (missing.length > 0) {
                    toast(`${input.name} is saved, but ${missing.join(" and ")} did not. Set them again and save.`, { tone: "warning" });
                    return;
                  }
                  toast("Saved.", { tone: "success" });
                }}
              />
            ) : (
              <Card padding="lg">
                {mirror ? (
                  <EmptyState icon={<InboxIcon />} line={menu.items.length === 0 ? "No items yet." : "Pick an item to read it."} />
                ) : (
                  <EmptyState
                    icon={<InboxIcon />}
                    line={menu.items.length === 0 ? "No items yet. Add the first one." : "Pick an item to edit it, or add a new one."}
                    action={{ label: "Add New Item", onClick: () => addItem(menu) }}
                  />
                )}
              </Card>
            )}
          </div>
        </div>
      )}

      {menu && filing === "import" && <ImportModal api={api} onClose={() => setFiling(null)} />}
      {menu && filing === "choices" && <ChoicesImportModal api={api} onClose={() => setFiling(null)} />}
      {menu && filing === "prices" && <PricesModal api={api} exported={exported} onClose={() => setFiling(null)} />}
      {menu && editing && (
        <EditingModal menu={menu} editing={editing} api={api} onCategory={(id) => pick({ scope: id, search: "" })} onClose={() => setEditing(null)} />
      )}
    </div>
  );
}

interface EditingModalProps {
  menu: MenuEditor;
  editing: Editing;
  api: MenuEditorApi;
  /** Shows a category just made. */
  onCategory: (id: number) => void;
  onClose: () => void;
}

function EditingModal({ menu, editing, api, onCategory, onClose }: EditingModalProps) {
  const toast = useToast();
  switch (editing.kind) {
    case "add-category":
      return (
        <CategoryFormModal
          title="Add a category"
          initial=""
          onSave={async (name) => {
            onCategory(await api.createCategory(name));
            toast("Saved.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "rename-category":
      return (
        <CategoryFormModal
          title={`Rename ${editing.name}`}
          initial={editing.name}
          onSave={async (name) => {
            await api.renameCategory(editing.id, name);
            toast("Saved.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "add-group":
      return (
        <GroupFormModal
          title="Add a group of choices"
          currency={menu.currency}
          initial={emptyGroupForm()}
          usedBy={0}
          onSave={async (input) => {
            await api.createGroup(input);
            toast("Saved. Offer it on items from each item's editor.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "edit-group":
      return (
        <GroupFormModal
          title={`Edit ${editing.group.name}`}
          currency={menu.currency}
          initial={groupFormFrom(editing.group)}
          usedBy={usedBy(menu, editing.group.id)}
          onSave={async (input) => {
            await api.updateGroup(editing.group.id, input);
            toast("Saved. Orders already open keep their choices.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
  }
}
