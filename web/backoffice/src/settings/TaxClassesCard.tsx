// The tax classes items share: "Food 5%", "Alcohol exempt" (backoffice-settings.md).
// Each change saves at once, as the menu editor's do; there is no draft.

import { useState } from "react";
import type { TaxClassResponse } from "../../../shared/api-types";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { Button, Card, EmptyState, InboxIcon } from "../../../shared/ui";
import { useOnMirror } from "../../../shared/use-mirror";
import { Choice, Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import { classFormFrom, classProblem, classProblemOf, classRateText, emptyClassForm, toClassInput, type ClassField, type ClassForm } from "./tax-logic";
import { problemWith } from "./problem";
import type { TaxApi } from "./use-tax";

type Editing = { kind: "add" } | { kind: "edit"; cls: TaxClassResponse };

export function TaxClassesCard({ classes, api }: { classes: TaxClassResponse[]; api: TaxApi }) {
  const [editing, setEditing] = useState<Editing | null>(null);
  const [busy, setBusy] = useState<number | null>(null);
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  // A mirror shows the classes and changes none (ADR-330).
  const mirror = useOnMirror();

  const run = (id: number, fn: () => Promise<void>, done: string) => {
    setBusy(id);
    setMessage(null);
    fn()
      .then(() => setMessage({ text: done, error: false }))
      .catch((err: unknown) => setMessage({ text: problemWith(err, []).message, error: true }))
      .finally(() => setBusy(null));
  };

  return (
    <Card padding="lg" className="min-w-0 md:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="font-serif text-xl">Tax classes</h2>
          <p className="mt-1 text-base text-fg-2">A rate that items share. A new rate is for lines rung up from then on: orders already open keep theirs.</p>
        </div>
        {!mirror && <Button onClick={() => setEditing({ kind: "add" })}>Add a tax class</Button>}
      </div>

      {classes.length === 0 ? (
        <div className="mt-6">
          <EmptyState icon={<InboxIcon />} line={mirror ? "No tax classes yet." : "No tax classes yet. Items need one, so add the first."} />
        </div>
      ) : (
        <ul className="mt-6 flex max-w-xl flex-col divide-y divide-line-soft border-y border-line-soft">
          {classes.map((c) => (
            <li key={c.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
              <div className="min-w-0 flex-1">
                <p className={c.active ? "text-base font-semibold" : "text-base text-fg-muted line-through"}>{c.name}</p>
                {!c.active && <p className="text-sm text-fg-muted">Retired</p>}
              </div>
              <p className="font-mono text-base tabular-nums">{classRateText(c)}</p>
              <div hidden={mirror} className="flex gap-2">
                <Button size="md" disabled={busy !== null} onClick={() => setEditing({ kind: "edit", cls: c })}>
                  Edit
                </Button>
                {c.active ? (
                  <Button size="md" variant="danger" disabled={busy !== null} onClick={() => run(c.id, () => api.retireClass(c.id), `${c.name} retired.`)}>
                    Retire
                  </Button>
                ) : (
                  <Button size="md" disabled={busy !== null} onClick={() => run(c.id, () => api.restoreClass(c.id), `${c.name} is back.`)}>
                    Bring back
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {mirror && <OnAMirror className="mt-6" />}
      {message && (
        <p role={message.error ? "alert" : "status"} className={message.error ? "mt-4 text-base text-danger" : "mt-4 text-base text-fg-2"}>
          {message.text}
        </p>
      )}

      {editing && (
        <ClassModal
          title={editing.kind === "add" ? "Add a tax class" : `Edit ${editing.cls.name}`}
          initial={editing.kind === "add" ? emptyClassForm() : classFormFrom(editing.cls)}
          onSave={(form) => (editing.kind === "add" ? api.createClass(toClassInput(form)) : api.updateClass(editing.cls.id, toClassInput(form)))}
          onClose={() => setEditing(null)}
        />
      )}
    </Card>
  );
}

function ClassModal({
  title,
  initial,
  onSave,
  onClose,
}: {
  title: string;
  initial: ClassForm;
  onSave: (form: ClassForm) => Promise<void>;
  onClose: () => void;
}) {
  const [form, setForm] = useState(initial);
  const [problem, setProblem] = useState<{ field: ClassField | null; message: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<ClassForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const save = async () => {
    const refused = classProblem(form);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      await onSave(form);
      onClose();
    } catch (err) {
      setProblem(classProblemOf(err));
      setBusy(false);
    }
  };
  const on = (field: ClassField) => (problem?.field === field ? problem.message : null);

  return (
    <FormModal
      title={title}
      size="md"
      busy={busy}
      onSave={() => void save()}
      onClose={onClose}
      refusal={problem && problem.field === null ? problem.message : null}
    >
      <Field label="Name" hint="Like Food, Drinks or Alcohol." error={on("name")}>
        {(id, aria) => (
          <input id={id} {...aria} className={inputClass} value={form.name} maxLength={40} autoComplete="off" onChange={(e) => set({ name: e.target.value })} />
        )}
      </Field>
      <Choice
        label="Tax"
        value={form.exempt ? "exempt" : "rate"}
        options={[
          { value: "rate", label: "Charged at a rate" },
          { value: "exempt", label: "Exempt" },
        ]}
        onChange={(v: string) => set({ exempt: v === "exempt" })}
        error={null}
      />
      {!form.exempt && (
        <Field label="Rate" hint="The whole rate, like 5 or 12. Under GST it is split in half into CGST and SGST on the invoice." error={on("rate")}>
          {(id, aria) => (
            <div className="flex items-center gap-2">
              <input
                id={id}
                {...aria}
                className={`${inputClass} max-w-32 font-mono`}
                value={form.rate}
                inputMode="decimal"
                autoComplete="off"
                onChange={(e) => set({ rate: e.target.value })}
              />
              <span aria-hidden="true">%</span>
            </div>
          )}
        </Field>
      )}
    </FormModal>
  );
}
