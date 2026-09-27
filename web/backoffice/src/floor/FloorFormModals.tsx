// Add or edit a table, and add or rename an area (backoffice-floor.md).
// The server checks every field; a refusal shows under the field it is
// about. The column and row are the numeric fallback for a drag.

import { useState } from "react";
import type { AreaResponse, TableShape } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Choice, Field, inputClass } from "../FormFields";
import { problemOf, type Problem, type TableForm } from "./floor-logic";

interface SaveState {
  problem: Problem | null;
  busy: boolean;
  save: () => void;
  clear: () => void;
  on: (field: Problem["field"]) => string | null;
}

/** Saving a modal: busy while it goes, closed when it lands, the refusal kept when it does not. */
function useSave(onSave: () => Promise<void>, onClose: () => void): SaveState {
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const save = () => {
    setBusy(true);
    onSave()
      .then(onClose)
      .catch((err: unknown) => {
        setProblem(problemOf(err));
        setBusy(false);
      });
  };
  return {
    problem,
    busy,
    save,
    clear: () => setProblem(null),
    on: (field) => (problem?.field === field ? problem.message : null),
  };
}

function Footer({ state, onClose }: { state: SaveState; onClose: () => void }) {
  return (
    <>
      <Button variant="ghost" onClick={onClose}>
        Cancel
      </Button>
      <Button variant="primary" disabled={state.busy} onClick={state.save}>
        {state.busy ? "Saving" : "Save"}
      </Button>
    </>
  );
}

function Overall({ problem }: { problem: Problem | null }) {
  if (!problem || problem.field !== null) return null;
  return (
    <p role="alert" className="text-base text-danger">
      {problem.message}
    </p>
  );
}

interface TableFormModalProps {
  title: string;
  initial: TableForm;
  areas: readonly AreaResponse[];
  columns: number;
  rows: number;
  onSave: (form: TableForm) => Promise<void>;
  onClose: () => void;
}

export function TableFormModal({ title, initial, areas, columns, rows, onSave, onClose }: TableFormModalProps) {
  const [form, setForm] = useState(initial);
  const state = useSave(() => onSave(form), onClose);
  const set = (patch: Partial<TableForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    state.clear();
  };
  const choosable = areas.filter((a) => a.active || a.id === initial.areaId);
  const numberClass = cx(inputClass, "font-mono");

  return (
    <Modal open onClose={onClose} title={title} size="md" footer={<Footer state={state} onClose={onClose} />}>
      <div className="flex flex-col gap-5">
        <Overall problem={state.problem} />
        <Field label="Name" hint="What staff call it, like 4 or T4." error={state.on("name")}>
          {(id) => <input id={id} className={inputClass} value={form.name} maxLength={20} onChange={(e) => set({ name: e.target.value })} />}
        </Field>
        <Field label="Seats" error={state.on("seats")}>
          {(id) => (
            <input id={id} className={numberClass} inputMode="numeric" value={form.seats} onChange={(e) => set({ seats: e.target.value })} />
          )}
        </Field>
        <Choice
          label="Shape"
          value={form.shape}
          options={[
            { value: "square", label: "Square" },
            { value: "round", label: "Round" },
            { value: "long", label: "Long" },
          ]}
          onChange={(shape: TableShape) => set({ shape })}
          error={state.on("shape")}
        />
        <Field label="Area" error={state.on("area_id")}>
          {(id) => (
            <select id={id} className={inputClass} value={form.areaId} onChange={(e) => set({ areaId: Number(e.target.value) })}>
              {choosable.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                  {a.active ? "" : " (retired)"}
                </option>
              ))}
            </select>
          )}
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Column" hint={`1 to ${columns}, from the left`} error={state.on("x")}>
            {(id) => (
              <input id={id} className={numberClass} inputMode="numeric" value={form.column} onChange={(e) => set({ column: e.target.value })} />
            )}
          </Field>
          <Field label="Row" hint={`1 to ${rows}, from the top`} error={state.on("y")}>
            {(id) => <input id={id} className={numberClass} inputMode="numeric" value={form.row} onChange={(e) => set({ row: e.target.value })} />}
          </Field>
        </div>
      </div>
    </Modal>
  );
}

interface AreaFormModalProps {
  title: string;
  initial: string;
  onSave: (name: string) => Promise<void>;
  onClose: () => void;
}

export function AreaFormModal({ title, initial, onSave, onClose }: AreaFormModalProps) {
  const [name, setName] = useState(initial);
  const state = useSave(() => onSave(name.trim()), onClose);
  return (
    <Modal open onClose={onClose} title={title} size="md" footer={<Footer state={state} onClose={onClose} />}>
      <div className="flex flex-col gap-5">
        <Overall problem={state.problem} />
        <Field label="Name" hint="Like Inside, Terrace or Upstairs." error={state.on("name")}>
          {(id) => (
            <input
              id={id}
              className={inputClass}
              value={name}
              maxLength={30}
              onChange={(e) => {
                setName(e.target.value);
                state.clear();
              }}
            />
          )}
        </Field>
      </div>
    </Modal>
  );
}
