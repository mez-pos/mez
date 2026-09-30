// The menu editor (docs/design/screens/backoffice-menu.md): categories down
// the side, the selected one's items in a table, and the forms that add and
// change them. Every change reaches the tills at once; tickets already open
// keep what they were rung up at.

import { useState } from "react";
import type { EditorItem, MenuEditor, MenuItemInput } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { CategoryFormModal, CategoryPanel } from "./CategoryPanel";
import { ItemFormModal } from "./ItemFormModal";
import { ItemsTable } from "./ItemsTable";
import { itemFormFrom, newItemForm, problemOf, selectedCategory } from "./menu-logic";
import { useMenuEditor, type MenuEditorApi } from "./use-menu-editor";

type Editing =
  | { kind: "add-category" }
  | { kind: "rename-category"; id: number; name: string }
  | { kind: "add-item"; categoryId: number }
  | { kind: "edit-item"; item: EditorItem };

export function MenuScreen() {
  const api = useMenuEditor();
  const [wanted, setWanted] = useState<number | null>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const menu = api.data;
  const category = menu ? selectedCategory(menu, wanted) : null;

  return (
    <div className="flex max-w-6xl flex-col gap-6">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Menu</h1>
          <p className="mt-1 text-sm text-fg-3">
            Changes reach every till at once. Tickets already open keep the prices they were rung up at.
          </p>
        </div>
        <div className="flex flex-wrap gap-tap-gap">
          <Button disabled={!menu} onClick={() => setEditing({ kind: "add-category" })}>
            Add a category
          </Button>
          <Button variant="primary" disabled={!category} onClick={() => category && setEditing({ kind: "add-item", categoryId: category.id })}>
            Add an item
          </Button>
        </div>
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
      {menu && !category && <EmptyState icon={<InboxIcon />} line="No menu yet. Add a category, then the items in it." />}
      {menu && category && (
        <div className="flex flex-col gap-6 2xl:flex-row 2xl:items-start">
          <CategoryPanel
            menu={menu}
            selected={category}
            onSelect={(id) => {
              setWanted(id);
              setProblem(null);
            }}
            api={api}
            onRename={() => setEditing({ kind: "rename-category", id: category.id, name: category.name })}
            onProblem={setProblem}
          />
          <ItemsTable menu={menu} category={category} api={api} onEdit={(item) => setEditing({ kind: "edit-item", item })} onProblem={setProblem} />
        </div>
      )}

      {menu && editing && <EditingModal menu={menu} editing={editing} api={api} onSelect={setWanted} onClose={() => setEditing(null)} />}
    </div>
  );
}

interface EditingModalProps {
  menu: MenuEditor;
  editing: Editing;
  api: MenuEditorApi;
  onSelect: (categoryId: number) => void;
  onClose: () => void;
}

function EditingModal({ menu, editing, api, onSelect, onClose }: EditingModalProps) {
  const toast = useToast();
  switch (editing.kind) {
    case "add-category":
      return (
        <CategoryFormModal
          title="Add a category"
          initial=""
          onSave={async (name) => {
            onSelect(await api.createCategory(name));
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
    case "add-item":
      return (
        <ItemFormModal
          menu={menu}
          item={null}
          initial={newItemForm(menu, editing.categoryId)}
          onSave={async (input) => {
            await api.createItem(input);
            onSelect(input.category_id);
            toast("Saved.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "edit-item":
      return (
        <ItemFormModal
          menu={menu}
          item={editing.item}
          initial={itemFormFrom(editing.item)}
          onSave={async (input: MenuItemInput) => {
            await api.updateItem(editing.item.id, input);
            onSelect(input.category_id);
            const repriced = input.price.minor !== editing.item.price.minor;
            toast(repriced ? "Saved. Tickets already open keep their prices." : "Saved.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
  }
}
