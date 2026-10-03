// The draft behind a settings section's form (backoffice-settings.md, "How a
// section saves"): what is typed, whether it differs from what the server
// holds, and Save, Discard and the words a refusal shows. Every section uses
// it, so each saves, discards and says it saved the same way.

import { useEffect, useState } from "react";
import type { Problem } from "./problem";

export interface DraftOptions<F, Field extends string> {
  /** What the server holds, as a form. */
  saved: F;
  changed: (draft: F, saved: F) => boolean;
  /** What can be refused without asking the server. */
  check: (draft: F) => Problem<Field> | null;
  /** Sends the draft, and resolves with the form the server now holds. */
  send: (draft: F) => Promise<F>;
  problemOf: (err: unknown) => Problem<Field>;
  /** Told whenever the draft starts or stops differing from what is saved. */
  onDirty: (dirty: boolean) => void;
}

export interface Draft<F, Field extends string> {
  form: F;
  update: (next: F | ((f: F) => F)) => void;
  dirty: boolean;
  busy: boolean;
  /** The last save went through and nothing has been typed since. */
  done: boolean;
  /** The words under a field, or null. */
  on: (field: Field) => string | null;
  /** A refusal about no field of the form. */
  footerError: string | null;
  save: () => void;
  discard: () => void;
}

export function useSectionDraft<F, Field extends string>(o: DraftOptions<F, Field>): Draft<F, Field> {
  const [form, setForm] = useState<F>(o.saved);
  const [problem, setProblem] = useState<Problem<Field> | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  const dirty = o.changed(form, o.saved);
  const { onDirty } = o;
  useEffect(() => {
    onDirty(dirty);
    return () => onDirty(false);
  }, [dirty, onDirty]);

  const update = (next: F | ((f: F) => F)) => {
    setForm(next);
    setProblem(null);
    setDone(false);
  };
  const save = () => {
    const refused = o.check(form);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    setProblem(null);
    o.send(form)
      .then((now) => {
        setForm(now);
        setDone(true);
      })
      .catch((err: unknown) => setProblem(o.problemOf(err)))
      .finally(() => setBusy(false));
  };
  const discard = () => {
    setForm(o.saved);
    setProblem(null);
    setDone(false);
  };

  return {
    form,
    update,
    dirty,
    busy,
    done,
    on: (field) => (problem?.field === field ? problem.message : null),
    footerError: problem && problem.field === null ? problem.message : null,
    save,
    discard,
  };
}
