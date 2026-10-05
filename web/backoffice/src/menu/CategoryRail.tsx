// The categories card of the menu editor (D18): All items, Popular, then each
// category in register order with how many items it has, and "+" to add one.
// What can be done to the chosen category (rename, move, retire) sits under
// the list: the design draws no place for it, and the back office has it
// (rebuild.md, rule 4).

import { LayoutGridIcon, PlusIcon, StarIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { useState } from "react";
import type { MenuEditor } from "../../../shared/api-types";
import { Button, Card, Confirm } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { scopeRows, type Scope } from "./browse-logic";
import { moved, problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

interface CategoryRailProps {
  menu: MenuEditor;
  scope: Scope;
  onScope: (scope: Scope) => void;
  api: MenuEditorApi;
  onAdd: () => void;
  onRename: (id: number, name: string) => void;
  /** A refusal to show above the columns. */
  onProblem: (message: string) => void;
}

const icon = (scope: Scope): LucideIcon => (scope === "all" ? LayoutGridIcon : scope === "popular" ? StarIcon : UtensilsIcon);

export function CategoryRail({ menu, scope, onScope, api, onAdd, onRename, onProblem }: CategoryRailProps) {
  const [confirmRetire, setConfirmRetire] = useState(false);
  const [busy, setBusy] = useState(false);
  const rows = scopeRows(menu);
  const ids = menu.categories.map((c) => c.id);
  const chosen = typeof scope === "number" ? (menu.categories.find((c) => c.id === scope) ?? null) : null;
  const index = chosen ? ids.indexOf(chosen.id) : -1;
  const count = chosen ? (rows.find((r) => r.scope === chosen.id)?.count ?? 0) : 0;

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
    <Card className="flex min-w-0 flex-col p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-serif text-xl">Categories</h2>
        <button
          type="button"
          aria-label="Add a category"
          onClick={onAdd}
          className="flex size-12 items-center justify-center rounded-round border border-line-strong outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
        >
          <PlusIcon aria-hidden="true" size={18} />
        </button>
      </div>
      <ul aria-label="Categories" className="mt-3 grid gap-0.5">
        {rows.map((r) => {
          const Icon = icon(r.scope);
          const on = r.scope === scope;
          return (
            <li key={r.scope}>
              <button
                type="button"
                aria-current={on ? "true" : undefined}
                onClick={() => onScope(r.scope)}
                className={cx(
                  "flex min-h-tap w-full items-center gap-2.5 rounded px-3 text-left text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                  on ? "bg-attention-tint font-bold text-attention-text" : "hover:bg-raised-high",
                )}
              >
                <Icon aria-hidden="true" size={18} className="shrink-0" />
                <span className={cx("min-w-0 flex-1 truncate", r.retired && !on && "text-fg-muted line-through")}>{r.name}</span>
                {r.retired && <span className="text-meta font-semibold uppercase text-fg-faint">Retired</span>}
                <span className={cx("font-mono tabular-nums", !on && "text-fg-muted")}>{r.count}</span>
              </button>
            </li>
          );
        })}
      </ul>

      {chosen && (
        <div role="group" aria-label={`${chosen.name}: actions`} className="mt-4 flex flex-wrap gap-tap-gap border-t border-line-soft pt-4">
          <Button size="md" disabled={busy} onClick={() => onRename(chosen.id, chosen.name)}>
            Rename
          </Button>
          <Button size="md" disabled={busy || index <= 0} onClick={() => move(-1)} aria-label={`Move ${chosen.name} up`}>
            <span aria-hidden="true">↑</span>
          </Button>
          <Button size="md" disabled={busy || index >= ids.length - 1} onClick={() => move(1)} aria-label={`Move ${chosen.name} down`}>
            <span aria-hidden="true">↓</span>
          </Button>
          {chosen.active ? (
            <Button size="md" disabled={busy} onClick={() => setConfirmRetire(true)}>
              Retire
            </Button>
          ) : (
            <Button size="md" disabled={busy} onClick={() => run(() => api.setCategoryActive(chosen.id, true))}>
              Bring back
            </Button>
          )}
        </div>
      )}

      <p aria-hidden="true" className="mt-auto hidden pt-5 font-script text-xl leading-tight text-fg-2 min-[1700px]:block">
        Good Food
        <br />
        Happier People ♡
      </p>

      {chosen && (
        <Confirm
          open={confirmRetire}
          onClose={() => setConfirmRetire(false)}
          onConfirm={() => {
            setConfirmRetire(false);
            run(() => api.setCategoryActive(chosen.id, false));
          }}
          title={`Retire ${chosen.name}?`}
          confirmLabel="Retire"
        >
          <p className="text-base">{count === 1 ? "Its one item comes" : `Its ${count} items come`} off every till. You can bring it back.</p>
        </Confirm>
      )}
    </Card>
  );
}
