// Add or edit a printer (backoffice-printers.md). The server checks every
// field; a refusal shows under the field it is about.

import { useState } from "react";
import type { PrinterTransport } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Choice, Field, inputClass } from "../FormFields";
import { problemOf, type PrinterForm, type PrintsFor, type Problem } from "./printers-logic";

interface PrinterFormModalProps {
  title: string;
  initial: PrinterForm;
  /** Asked when adding: what the new printer is for (BO-15). Editing leaves it to Where things print. */
  prints?: { options: readonly { value: PrintsFor; label: string }[]; initial: PrintsFor } | undefined;
  onSave: (form: PrinterForm, prints: PrintsFor) => Promise<void>;
  onClose: () => void;
}

export function PrinterFormModal({ title, initial, prints, onSave, onClose }: PrinterFormModalProps) {
  const [form, setForm] = useState(initial);
  const [printsFor, setPrintsFor] = useState<PrintsFor>(prints?.initial ?? "later");
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<PrinterForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const save = async () => {
    setBusy(true);
    try {
      await onSave(form, printsFor);
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };
  const on = (field: Problem["field"]) => (problem?.field === field ? problem.message : null);
  const usb = form.transport === "windows";

  return (
    <Modal
      open
      onClose={onClose}
      title={title}
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
      <div className="flex flex-col gap-5">
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label="Name" error={on("name")}>
          {(id, aria) => <input id={id} {...aria} className={inputClass} value={form.name} maxLength={40} onChange={(e) => set({ name: e.target.value })} />}
        </Field>
        <Choice
          label="How it connects"
          value={form.transport}
          options={[
            { value: "tcp", label: "Network (IP address)" },
            { value: "windows", label: "USB, on this computer (Windows)" },
          ]}
          onChange={(transport: PrinterTransport) => set({ transport })}
          error={on("transport")}
        />
        <Field
          label={usb ? "The printer's name in Windows" : "IP address"}
          hint={usb ? "As Windows shows it under Printers, like EPSON TM-T82." : "Like 192.168.1.50. Port 9100 unless you add another."}
          error={on("address")}
        >
          {(id, aria) => (
            <input
              id={id}
              {...aria}
              className={cx(inputClass, !usb && "font-mono")}
              value={form.address}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => set({ address: e.target.value })}
            />
          )}
        </Field>
        <Choice
          label="Paper"
          value={String(form.paperMM)}
          options={[
            { value: "80", label: "80mm" },
            { value: "58", label: "58mm" },
          ]}
          onChange={(v: string) => set({ paperMM: v === "58" ? 58 : 80 })}
          error={on("paper_mm")}
        />
        {prints && <Choice label="What it prints" value={printsFor} options={prints.options} onChange={(v: PrintsFor) => setPrintsFor(v)} error={null} />}
      </div>
    </Modal>
  );
}
