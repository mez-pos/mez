// Add or edit a printer (backoffice-printers.md). The server checks every
// field; a refusal shows under the field it is about.

import { useId, useState, type ReactNode } from "react";
import type { PrinterTransport } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { problemOf, type PrinterForm, type Problem } from "./printers-logic";

interface PrinterFormModalProps {
  title: string;
  initial: PrinterForm;
  onSave: (form: PrinterForm) => Promise<void>;
  onClose: () => void;
}

const inputClass =
  "min-h-tap w-full rounded border border-line-strong bg-raised px-3 text-fg outline-offset-2 outline-line-focus focus-visible:outline-2";

export function PrinterFormModal({ title, initial, onSave, onClose }: PrinterFormModalProps) {
  const [form, setForm] = useState(initial);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<PrinterForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const save = async () => {
    setBusy(true);
    try {
      await onSave(form);
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
          {(id) => (
            <input id={id} className={inputClass} value={form.name} maxLength={40} onChange={(e) => set({ name: e.target.value })} />
          )}
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
          {(id) => (
            <input
              id={id}
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
      </div>
    </Modal>
  );
}

function Field({ label, hint, error, children }: { label: string; hint?: string; error: string | null; children: (id: string) => ReactNode }) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-base font-semibold">
        {label}
      </label>
      {children(id)}
      {hint && <p className="text-sm text-fg-muted">{hint}</p>}
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

interface ChoiceProps<T extends string> {
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (v: T) => void;
  error: string | null;
}

function Choice<T extends string>({ label, value, options, onChange, error }: ChoiceProps<T>) {
  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-base font-semibold">{label}</p>
      <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-tap-gap">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={value === o.value}
            onClick={() => onChange(o.value)}
            className={cx(
              "min-h-tap rounded border px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
              value === o.value ? "border-line-focus bg-raised font-semibold text-fg" : "border-line text-fg-2",
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
