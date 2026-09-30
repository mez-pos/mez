// The categories column of the menu editor (backoffice-menu.md): the list in
// register order, and what can be done to the selected one.

import { useState } from "react";
import type { EditorCategory, MenuEditor } from "../../../shared/api-types";
import { Button, Card, Confirm, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Field, inputClass } from "../FormFields";
import { itemsOf, moved, problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

interface CategoryPanelProps {
  menu: MenuEditor;
  selected: EditorCategory;
  onSelect: (id: number) => void;
  api: MenuEditorApi;
  onRename: () => void;
  /** A refusal to show above the columns. */
  onProblem: (message: string) => void;
}

export function CategoryPanel({ menu, selected, onSelect, api, onRename, onProblem }: CategoryPanelProps) {
  const [confirmRetire, setConfirmRetire] = useState(false);
  const [busy, setBusy] = useState(false);
  const ids = menu.categories.map((c) => c.id);
  const index = ids.indexOf(selected.id);
  const count = itemsOf(menu, selected.id).length;

  const run = (fn: () => Promise<void>) => {
    setBusy(true);
    fn()
      .catch((err: unknown) => onProblem(problemOf(err).message))
      .finally(() => setBusy(false));
  };
  const move = (delta: -1 | 1) => {
    const next = moved(ids, index, delta);
    if (next) run(() => api.orderCategories(next));
  };

  return (
    <div className="flex flex-col gap-3 2xl:w-70 2xl:shrink-0">
      <Card padding="none">
        <ul aria-label="Categories">
          {menu.categories.map((c) => {
            const active = c.id === selected.id;
            return (
              <li key={c.id} className="border-b border-line-soft last:border-b-0">
                <button
                  type="button"
                  aria-current={active ? "true" : undefined}
                  onClick={() => onSelect(c.id)}
                  className={cx(
                    "flex min-h-tap w-full items-center justify-between gap-3 border-l-2 px-3 text-left text-sm",
                    active ? "border-line-strong bg-sunken font-semibold" : "border-transparent hover:bg-sunken",
                  )}
                >
                  <span className={cx(!c.active && "text-fg-muted line-through")}>{c.name}</span>
                  <span className="flex items-center gap-2">
                    {!c.active && <span className="text-meta uppercase text-fg-faint">Retired</span>}
                    <span className="font-mono tabular-nums text-fg-3">{itemsOf(menu, c.id).length}</span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </Card>

      <div className="flex flex-wrap gap-tap-gap" aria-label={`${selected.name}: actions`} role="group">
        <Button disabled={busy} onClick={onRename}>
          Rename
        </Button>
        <Button disabled={busy || index <= 0} onClick={() => move(-1)} aria-label={`Move ${selected.name} up`}>
          Move up
        </Button>
        <Button disabled={busy || index >= ids.length - 1} onClick={() => move(1)} aria-label={`Move ${selected.name} down`}>
          Move down
        </Button>
        {selected.active ? (
          <Button disabled={busy} onClick={() => setConfirmRetire(true)}>
            Retire
          </Button>
        ) : (
          <Button disabled={busy} onClick={() => run(() => api.setCategoryActive(selected.id, true))}>
            Bring back
          </Button>
        )}
      </div>

      <Confirm
        open={confirmRetire}
        onClose={() => setConfirmRetire(false)}
        onConfirm={() => {
          setConfirmRetire(false);
          run(() => api.setCategoryActive(selected.id, false));
        }}
        title={`Retire ${selected.name}?`}
        confirmLabel="Retire"
      >
        <p className="text-base">
          {count === 1 ? "Its one item comes" : `Its ${count} items come`} off every till. You can bring it back.
        </p>
      </Confirm>
    </div>
  );
}

interface CategoryFormModalProps {
  title: string;
  initial: string;
  onSave: (name: string) => Promise<void>;
  onClose: () => void;
}

export function CategoryFormModal({ title, initial, onSave, onClose }: CategoryFormModalProps) {
  const [name, setName] = useState(initial);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    try {
      await onSave(name);
      onClose();
    } catch (err) {
      setProblem(problemOf(err).message);
      setBusy(false);
    }
  };
  return (
    <Modal
      open
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void save()}>
            {busy ? "Saving" : "Save"}
          </Button>
        </>
      }
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <Field label="Name" error={problem}>
          {(id) => (
            <input
              id={id}
              className={inputClass}
              value={name}
              maxLength={60}
              autoFocus
              onChange={(e) => {
                setName(e.target.value);
                setProblem(null);
              }}
            />
          )}
        </Field>
      </form>
    </Modal>
  );
}
