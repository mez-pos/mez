// Add or edit a group of choices (backoffice-menu.md, "Choices"). The group
// is saved whole: its name, how many to pick, and its choices in order. A
// choice removed here is retired on the server; a line that chose it keeps
// what it copied.

import { useState } from "react";
import type { MenuGroupInput } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import { groupProblemOf, newChoice, toGroupInput, type ChoiceForm, type GroupForm, type GroupProblem } from "./group-logic";
import { moved } from "./menu-logic";

interface GroupFormModalProps {
  title: string;
  currency: string;
  initial: GroupForm;
  /** How many items offer it, for the note on an edit. */
  usedBy: number;
  onSave: (input: MenuGroupInput) => Promise<void>;
  onClose: () => void;
}

export function GroupFormModal({ title, currency, initial, usedBy, onSave, onClose }: GroupFormModalProps) {
  const [form, setForm] = useState(initial);
  const [problem, setProblem] = useState<GroupProblem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<GroupForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const setChoice = (index: number, patch: Partial<ChoiceForm>) => set({ choices: form.choices.map((c, i) => (i === index ? { ...c, ...patch } : c)) });
  const moveChoice = (index: number, delta: -1 | 1) => {
    const order = moved(
      form.choices.map((_, i) => i),
      index,
      delta,
    );
    if (order) set({ choices: order.map((i) => form.choices[i]!) });
  };

  const save = async () => {
    const built = toGroupInput(form, currency);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    setBusy(true);
    try {
      await onSave(built.input);
      onClose();
    } catch (err) {
      setProblem(groupProblemOf(err));
      setBusy(false);
    }
  };
  const on = (field: GroupProblem["field"]) => (problem && !problem.row && problem.field === field ? problem.message : null);
  const onRow = (index: number) => (problem?.row?.index === index ? problem.message : null);

  return (
    <FormModal
      title={title}
      size="md"
      busy={busy}
      onSave={() => void save()}
      onClose={onClose}
      refusal={problem && !problem.row && problem.field === null ? problem.message : null}
    >
      {usedBy > 0 && (
        <p className="text-sm text-fg-3">
          {usedBy === 1 ? "One item offers" : `${usedBy} items offer`} these choices. Tickets already open keep the choices and prices they were rung up with.
        </p>
      )}
      <Field label="Name" hint="As staff see it on the till: Milk, Extras, Doneness." error={on("name")}>
        {(id) => <input id={id} className={inputClass} value={form.name} maxLength={60} autoFocus onChange={(e) => set({ name: e.target.value })} />}
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Fewest to pick" hint="0 makes it optional." error={on("min_select")}>
          {(id) => (
            <input
              id={id}
              className={cx(inputClass, "text-right font-mono")}
              value={form.min}
              inputMode="numeric"
              onChange={(e) => set({ min: e.target.value })}
            />
          )}
        </Field>
        <Field label="Most to pick" error={on("max_select")}>
          {(id) => (
            <input
              id={id}
              className={cx(inputClass, "text-right font-mono")}
              value={form.max}
              inputMode="numeric"
              onChange={(e) => set({ max: e.target.value })}
            />
          )}
        </Field>
      </div>

      <fieldset className="flex flex-col gap-3">
        <legend className="mb-1.5 text-base font-semibold">Choices</legend>
        <p className="text-sm text-fg-muted">Price is what it adds to the item, in {currency}: 40, or -20 for less. Empty adds nothing.</p>
        {on("modifiers") && (
          <p role="alert" className="text-sm text-danger">
            {on("modifiers")}
          </p>
        )}
        <ol className="flex flex-col gap-3">
          {form.choices.map((c, i) => (
            <li key={c.key} className="flex flex-col gap-1.5 rounded border border-line-soft p-2.5">
              <div className="flex flex-wrap items-center gap-tap-gap">
                <input
                  aria-label={`Choice ${i + 1} name`}
                  className={cx(inputClass, "min-w-0 flex-1 basis-40")}
                  value={c.name}
                  maxLength={60}
                  placeholder="Oat milk"
                  onChange={(e) => setChoice(i, { name: e.target.value })}
                />
                <input
                  aria-label={`Choice ${i + 1} price`}
                  className={cx(inputClass, "w-28 text-right font-mono")}
                  value={c.price}
                  inputMode="decimal"
                  placeholder="0"
                  onChange={(e) => setChoice(i, { price: e.target.value })}
                />
              </div>
              <div className="flex flex-wrap items-center gap-tap-gap">
                <label className="flex min-h-tap flex-1 items-center gap-2 text-sm">
                  <input type="checkbox" className="size-5" checked={c.isDefault} onChange={(e) => setChoice(i, { isDefault: e.target.checked })} />
                  Picked to start
                </label>
                <Button aria-label={`Move choice ${i + 1} up`} disabled={i === 0} onClick={() => moveChoice(i, -1)}>
                  <span aria-hidden="true">↑</span>
                </Button>
                <Button aria-label={`Move choice ${i + 1} down`} disabled={i === form.choices.length - 1} onClick={() => moveChoice(i, 1)}>
                  <span aria-hidden="true">↓</span>
                </Button>
                <Button variant="ghost" onClick={() => set({ choices: form.choices.filter((_, j) => j !== i) })}>
                  Remove
                </Button>
              </div>
              {onRow(i) && (
                <p role="alert" className="text-sm text-danger">
                  {onRow(i)}
                </p>
              )}
            </li>
          ))}
        </ol>
        <Button className="self-start" onClick={() => set({ choices: [...form.choices, newChoice()] })}>
          Add a choice
        </Button>
      </fieldset>
      <button type="submit" hidden />
    </FormModal>
  );
}
