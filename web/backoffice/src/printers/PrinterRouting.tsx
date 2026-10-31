// Where receipts and each station's tickets print: a printer picked for
// each, saved as it is picked.

import { useState } from "react";
import { useAway } from "../../../shared/use-realtime";
import { Link } from "wouter";
import type { PrinterResponse, PrintersResponse } from "../../../shared/api-types";
import { Panel } from "../dashboard/Panel";
import { usePermissions } from "../shell/BackOfficeShell";
import { problemOf } from "./printers-logic";
import type { PrinterSettingsApi } from "./use-printers";
import { inputClass } from "../../../shared/ui/field";

export function PrinterRouting({ data, api }: { data: PrintersResponse; api: PrinterSettingsApi }) {
  const mayEditMenu = usePermissions().includes("menu.edit");
  const active = data.printers.filter((p) => p.active);
  const receipt = data.printers.find((p) => p.is_receipt && p.active)?.id ?? null;
  return (
    <Panel title="Where things print">
      <div className="flex flex-col gap-4">
        <Assign label="Receipts" value={receipt} printers={active} onChange={(id) => api.setReceipt(id)} />
        {data.stations.length === 0 && <p className="text-base text-fg-muted">No stations yet. They are added in the menu, with the items made at each.</p>}
        {data.stations.map((s) => {
          const current = s.printer_id !== undefined && active.some((p) => p.id === s.printer_id) ? s.printer_id : null;
          return <Assign key={s.code} label={`${s.name} tickets`} value={current} printers={active} onChange={(id) => api.setStation(s.code, id)} />;
        })}
        {mayEditMenu && (
          <p className="text-sm text-fg-muted">
            Stations are added, renamed and retired in the menu:{" "}
            <Link href="/menu?view=setup" className="text-attention-text underline">
              Stations and courses
            </Link>
            .
          </p>
        )}
      </div>
    </Panel>
  );
}

interface AssignProps {
  label: string;
  value: number | null;
  printers: readonly PrinterResponse[];
  onChange: (id: number | null) => Promise<void>;
}

function Assign({ label, value, printers, onChange }: AssignProps) {
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  // Which printer prints what is set at the venue (ADR-308).
  const away = useAway();
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-base font-semibold">{label}</span>
      <select
        className={inputClass}
        value={value ?? ""}
        disabled={away}
        onChange={(e) => {
          setMessage(null);
          onChange(e.target.value === "" ? null : Number(e.target.value))
            .then(() => setMessage({ text: "Saved", error: false }))
            .catch((err: unknown) => setMessage({ text: problemOf(err).message, error: true }));
        }}
      >
        <option value="">None</option>
        {printers.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>
      {message && (
        <span role={message.error ? "alert" : "status"} className={message.error ? "text-sm text-danger" : "text-sm text-fg-muted"}>
          {message.text}
        </span>
      )}
    </label>
  );
}
