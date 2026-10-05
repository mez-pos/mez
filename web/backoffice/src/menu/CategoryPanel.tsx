// The form that names a category of the menu, new or renamed. The list of
// categories is CategoryRail.tsx.

import { useState } from "react";
import { Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import { problemOf } from "./menu-logic";

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
    <FormModal title={title} busy={busy} onSave={() => void save()} onClose={onClose}>
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
    </FormModal>
  );
}
