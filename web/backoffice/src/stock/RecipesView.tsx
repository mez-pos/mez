// The Recipes view of the Stock screen (backoffice-stock.md): what one of
// each dish, and each choice, takes from stock. The dishes are the till's
// menu, so a manager without menu.edit can keep recipes (ADR-112).

import { useState } from "react";
import type { MenuResponse, RecipePart, StockItem } from "../../../shared/api-types";
import { Button, Card, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { inputClass } from "../FormFields";
import { newRecipeRow, problemOf, recipeLine, recipeRowsFrom, toParts, type Problem, type RecipeRow } from "./stock-logic";
import type { StockApi } from "./use-stock";

interface Target {
  kind: "item" | "modifier";
  id: number;
  name: string;
}

interface RecipesViewProps {
  menu: MenuResponse;
  stock: readonly StockItem[];
  api: StockApi;
  onSaved: (name: string) => void;
  /** The recipes are read and none is edited: a mirror (ADR-330). */
  readOnly?: boolean;
}

export function RecipesView({ menu, stock, api, onSaved, readOnly = false }: RecipesViewProps) {
  const [editing, setEditing] = useState<Target | null>(null);
  const byId = new Map(stock.map((s) => [s.id, s]));
  const book = api.recipes.data;
  const partsOf = (t: Pick<Target, "kind" | "id">): RecipePart[] =>
    (t.kind === "item" ? book?.items : book?.modifiers)?.find((r) => r.id === t.id)?.parts ?? [];

  const row = (t: Target) => {
    const text = recipeLine(partsOf(t), byId);
    const tracked = partsOf(t).length > 0;
    return (
      <li key={`${t.kind}-${t.id}`} className="flex min-h-tap items-center justify-between gap-4 border-b border-line-soft px-4 py-1.5 last:border-b-0">
        <span className="min-w-0 text-sm">
          <span className="font-semibold">{t.name}</span>
          <span className={cx("block", tracked ? "text-fg-2" : "text-fg-muted")}>{text}</span>
        </span>
        {!readOnly && (
          <Button aria-label={`Edit the recipe of ${t.name}`} onClick={() => setEditing(t)}>
            Edit
          </Button>
        )}
      </li>
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <p className="max-w-prose text-sm text-fg-3">A dish takes its own recipe plus each chosen choice's. Put milk on the milk choices, not on the latte.</p>
      {!book && api.recipes.isPending && <p className="text-base text-fg-muted">Loading the recipes</p>}
      <section aria-labelledby="dishes">
        <h2 id="dishes" className="mb-2 text-meta font-semibold uppercase text-fg-faint">
          Dishes
        </h2>
        <Card padding="none">
          <ul>{menu.items.map((i) => row({ kind: "item", id: i.id, name: i.name }))}</ul>
        </Card>
      </section>
      {menu.modifier_groups.map((g) => (
        <section key={g.id} aria-labelledby={`group-${g.id}`}>
          <h2 id={`group-${g.id}`} className="mb-2 text-meta font-semibold uppercase text-fg-faint">
            Choices: {g.name}
          </h2>
          <Card padding="none">
            <ul>{g.modifiers.map((m) => row({ kind: "modifier", id: m.id, name: m.name }))}</ul>
          </Card>
        </section>
      ))}
      {editing && (
        <RecipeModal
          target={editing}
          parts={partsOf(editing)}
          stock={stock}
          onSave={(parts) => (editing.kind === "item" ? api.setItemRecipe(editing.id, { parts }) : api.setModifierRecipe(editing.id, { parts }))}
          onDone={() => onSaved(editing.name)}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

interface RecipeModalProps {
  target: Target;
  parts: readonly RecipePart[];
  stock: readonly StockItem[];
  onSave: (parts: RecipePart[]) => Promise<void>;
  onDone: () => void;
  onClose: () => void;
}

function RecipeModal({ target, parts, stock, onSave, onDone, onClose }: RecipeModalProps) {
  const [rows, setRows] = useState<RecipeRow[]>(() => recipeRowsFrom(parts));
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const usable = stock.filter((s) => s.active);
  const byId = new Map(stock.map((s) => [s.id, s]));
  const set = (next: RecipeRow[]) => {
    setRows(next);
    setProblem(null);
  };
  const save = async () => {
    const built = toParts(rows);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    setBusy(true);
    try {
      await onSave(built.parts);
      onDone();
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`What one ${target.name} takes`}
      size="md"
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
      <div className="flex flex-col gap-3">
        {problem && !problem.row && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        {rows.length === 0 && <p className="text-sm text-fg-muted">It takes nothing from stock.</p>}
        <ol className="flex flex-col gap-2.5">
          {rows.map((r, i) => {
            const unit = byId.get(r.stockItemId)?.unit;
            return (
              <li key={r.key} className="flex flex-col gap-1.5">
                <div className="flex flex-wrap items-center gap-tap-gap">
                  <select
                    aria-label={`Stock item ${i + 1}`}
                    className={cx(inputClass, "min-w-0 flex-1 basis-40")}
                    value={r.stockItemId}
                    onChange={(e) => set(rows.map((x, j) => (j === i ? { ...x, stockItemId: Number(e.target.value) } : x)))}
                  >
                    <option value={0}>Choose a stock item</option>
                    {stock
                      .filter((s) => s.active || s.id === r.stockItemId)
                      .map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.active ? s.name : `${s.name} (retired)`}
                        </option>
                      ))}
                  </select>
                  <input
                    aria-label={`Amount of stock item ${i + 1}${unit ? `, in ${unit}` : ""}`}
                    className={cx(inputClass, "w-28 text-right font-mono tabular-nums")}
                    value={r.qty}
                    inputMode="decimal"
                    placeholder="0"
                    onChange={(e) => set(rows.map((x, j) => (j === i ? { ...x, qty: e.target.value } : x)))}
                  />
                  <span className="w-10 text-sm text-fg-3">{unit ?? ""}</span>
                  <Button variant="ghost" onClick={() => set(rows.filter((_, j) => j !== i))}>
                    Remove
                  </Button>
                </div>
                {problem?.row?.index === i && (
                  <p role="alert" className="text-sm text-danger">
                    {problem.message}
                  </p>
                )}
              </li>
            );
          })}
        </ol>
        <Button className="self-start" disabled={usable.length === 0} onClick={() => set([...rows, newRecipeRow()])}>
          Add an ingredient
        </Button>
        {/* There is no "Items view": the way is the screen's own button (BO-9). */}
        {usable.length === 0 && <p className="text-sm text-fg-muted">Add an ingredient first, with Add ingredient at the top of Inventory.</p>}
      </div>
    </Modal>
  );
}
