// Add or edit a stock item (backoffice-stock.md). On an existing item the
// form also offers Correct and Retire / Bring back.

import { useState } from "react";
import type { StockItem, StockItemInput } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Choice, Field, inputClass } from "../FormFields";
import { emptyItemForm, itemFormFrom, problemOf, toItemInput, units, type ItemForm, type Problem } from "./stock-logic";

interface StockItemModalProps {
  /** Null to add one. */
  item: StockItem | null;
  onSave: (input: StockItemInput) => Promise<void>;
  onCorrect: () => void;
  onSetActive: (active: boolean) => Promise<void>;
  onClose: () => void;
}

export function StockItemModal({ item, onSave, onCorrect, onSetActive, onClose }: StockItemModalProps) {
  const [form, setForm] = useState<ItemForm>(item ? itemFormFrom(item) : emptyItemForm());
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<ItemForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };
  const save = () => {
    const built = toItemInput(form);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    void run(() => onSave(built.input));
  };
  const on = (field: string) => (problem?.field === field ? problem.message : null);

  return (
    <Modal
      open
      onClose={onClose}
      title={item ? `Edit ${item.name}` : "Add a stock item"}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={save}>
            {busy ? "Saving" : "Save"}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label="Name" error={on("name")}>
          {(id) => <input id={id} className={inputClass} value={form.name} maxLength={60} autoFocus onChange={(e) => set({ name: e.target.value })} />}
        </Field>
        <Choice label="Counted in" value={form.unit} options={units} onChange={(unit) => set({ unit })} error={on("unit")} />
        <Field label={`Low at, in ${form.unit}`} hint="Shown as low at or under this. Empty for never." error={on("low_level")}>
          {(id) => (
            <input
              id={id}
              className={cx(inputClass, "text-right font-mono tabular-nums")}
              value={form.low}
              inputMode="decimal"
              autoComplete="off"
              onChange={(e) => set({ low: e.target.value })}
            />
          )}
        </Field>
        <label className="flex min-h-tap items-center gap-2.5 text-base">
          <input type="checkbox" className="size-5" checked={form.countAtClose} onChange={(e) => set({ countAtClose: e.target.checked })} />
          <span>
            Count at close
            <span className="block text-sm text-fg-muted">Counted, blind, on the till as each shift closes.</span>
          </span>
        </label>
        {item && (
          <div className="flex flex-wrap gap-tap-gap border-t border-line-soft pt-4">
            {item.active && (
              <Button
                disabled={busy}
                onClick={() => {
                  onClose();
                  onCorrect();
                }}
              >
                Correct on hand
              </Button>
            )}
            <Button disabled={busy} onClick={() => void run(() => onSetActive(!item.active))}>
              {item.active ? "Retire" : "Bring back"}
            </Button>
          </div>
        )}
        <button type="submit" hidden />
      </form>
    </Modal>
  );
}
