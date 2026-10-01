// Receive, waste, count or correct one stock item (backoffice-stock.md).
// A count says what it will record before it is saved; the server works
// out the delta from the ledger as it stands then (ADR-111).

import { useState } from "react";
import type { StockItem, StockMovementInput, StockRecorded } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Field, inputClass } from "../FormFields";
import { formatDelta, formatQty } from "./qty";
import { countDelta, emptyMovementForm, problemOf, reasons, toMovementInput, type HandKind, type MovementForm, type Problem } from "./stock-logic";

const copy: Readonly<Record<HandKind, { title: string; qty: string; save: string }>> = {
  received: { title: "Receive", qty: "How much came in", save: "Receive" },
  wasted: { title: "Waste", qty: "How much was thrown away", save: "Record waste" },
  counted: { title: "Count", qty: "How much is there now", save: "Record count" },
  adjusted: { title: "Correct", qty: "The change, like 500 or -500", save: "Correct" },
};

interface MovementModalProps {
  item: StockItem;
  kind: HandKind;
  onSave: (input: StockMovementInput) => Promise<StockRecorded>;
  onDone: (recorded: StockRecorded) => void;
  onClose: () => void;
}

export function MovementModal({ item, kind, onSave, onDone, onClose }: MovementModalProps) {
  const [form, setForm] = useState<MovementForm>(emptyMovementForm());
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<MovementForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const save = async () => {
    const built = toMovementInput(kind, form);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    setBusy(true);
    try {
      onDone(await onSave(built.input));
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };
  const on = (field: string) => (problem?.field === field ? problem.message : null);
  const delta = kind === "counted" ? countDelta(form.qty, item.on_hand) : null;
  const c = copy[kind];

  return (
    <Modal
      open
      onClose={onClose}
      title={`${c.title}: ${item.name}`}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void save()}>
            {busy ? "Saving" : c.save}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label={`${c.qty}, in ${item.unit}`} error={on("qty")}>
          {(id) => (
            <input
              id={id}
              className={cx(inputClass, "text-right font-mono tabular-nums")}
              value={form.qty}
              inputMode="decimal"
              autoComplete="off"
              autoFocus
              onChange={(e) => set({ qty: e.target.value })}
            />
          )}
        </Field>
        {kind === "counted" && (
          <p role="status" className="rounded border border-line bg-sunken px-3 py-2.5 text-sm text-fg-2">
            The ledger says <span className="font-mono">{formatQty(item.on_hand, item.unit)}</span>.
            {delta !== null && (
              <>
                {" "}
                This records <span className="font-mono">{formatDelta(delta, item.unit)}</span>.
              </>
            )}
          </p>
        )}
        {kind === "wasted" && (
          <Field label="Why" error={on("reason")}>
            {(id) => (
              <select id={id} className={inputClass} value={form.reason} onChange={(e) => set({ reason: e.target.value as MovementForm["reason"] })}>
                <option value="">Choose a reason</option>
                {reasons.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </select>
            )}
          </Field>
        )}
        <Field
          label={kind === "adjusted" || form.reason === "other" ? "Note" : "Note, if any"}
          {...(kind === "adjusted" ? { hint: "Say why it is being corrected." } : {})}
          error={on("note")}
        >
          {(id) => <input id={id} className={inputClass} value={form.note} maxLength={200} onChange={(e) => set({ note: e.target.value })} />}
        </Field>
        <button type="submit" hidden />
      </form>
    </Modal>
  );
}
