// The venue's printers (docs/design/screens/backoffice-printers.md): each
// printer with how it is reached, what it prints and its last result; where
// receipts and each station's tickets print; add, edit, test and retire.

import { useEffect, useState } from "react";
import { Link } from "wouter";
import type { PrinterResponse, PrintersResponse } from "../../../shared/api-types";
import { Button, Card, EmptyState, InboxIcon, StatusChip } from "../../../shared/ui";
import { agoText, connectionText, emptyForm, formFrom, problemOf, stationsText, toSettings } from "./printers-logic";
import { PrinterFormModal } from "./PrinterFormModal";
import { usePrinters, type PrinterSettingsApi } from "./use-printers";

type Editing = { kind: "add" } | { kind: "edit"; printer: PrinterResponse };

export function PrintersScreen() {
  const api = usePrinters();
  const [editing, setEditing] = useState<Editing | null>(null);

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <Link href="/" className="-mb-4 inline-flex min-h-tap items-center self-start text-base text-fg-2 underline">
        Back office
      </Link>
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <h1 className="font-serif text-total">Printers</h1>
        <Button variant="primary" onClick={() => setEditing({ kind: "add" })}>
          Add a printer
        </Button>
      </div>

      {api.loading && <p className="text-base text-fg-muted">Loading the printers</p>}
      {!api.loading && api.error !== null && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
      )}
      {api.data && api.data.printers.length === 0 && (
        <EmptyState icon={<InboxIcon />} line="No printers yet. Add the receipt printer first, then one for each station that needs tickets." />
      )}
      {api.data && (
        <>
          <ul className="flex flex-col gap-4">
            {api.data.printers.map((p) => (
              <PrinterCard key={p.id} printer={p} data={api.data!} api={api} onEdit={() => setEditing({ kind: "edit", printer: p })} />
            ))}
          </ul>
          {api.data.printers.length > 0 && <Routing data={api.data} api={api} />}
        </>
      )}

      {editing && (
        <PrinterFormModal
          title={editing.kind === "add" ? "Add a printer" : `Edit ${editing.printer.name}`}
          initial={editing.kind === "add" ? emptyForm() : formFrom(editing.printer)}
          onSave={(form) =>
            editing.kind === "add" ? api.create(toSettings(form)) : api.update(editing.printer.id, toSettings(form))
          }
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

interface PrinterCardProps {
  printer: PrinterResponse;
  data: PrintersResponse;
  api: PrinterSettingsApi;
  onEdit: () => void;
}

function PrinterCard({ printer: p, data, api, onEdit }: PrinterCardProps) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  const run = (fn: () => Promise<void>, done: string) => {
    setBusy(true);
    setMessage(null);
    fn()
      .then(() => setMessage({ text: done, error: false }))
      .catch((err: unknown) => setMessage({ text: problemOf(err).message, error: true }))
      .finally(() => setBusy(false));
  };
  const now = useNow();

  return (
    <li>
      <Card padding="lg" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="text-lg font-semibold">{p.name}</h2>
          {p.is_receipt && <span className="rounded border border-line-strong px-2 py-0.5 text-sm font-semibold">Receipts</span>}
          {!p.active && <span className="rounded border border-line px-2 py-0.5 text-sm text-fg-muted line-through">Retired</span>}
        </div>
        <p className="flex flex-wrap gap-x-4 text-base text-fg-2">
          <span className="font-mono">{connectionText(p)}</span>
          <span className="font-mono">{p.paper_mm}mm</span>
          <span>{stationsText(p, data.stations)}</span>
        </p>
        <div className="flex flex-wrap items-center gap-2 text-sm text-fg-muted">
          {p.last_job ? (
            <>
              <StatusChip status={{ kind: "print", status: p.last_job.state }} />
              <span>{p.last_job.label}</span>
              <span>{agoText(p.last_job.at, now)}</span>
              {p.last_job.error && <span className="text-danger">{p.last_job.error}</span>}
            </>
          ) : (
            <span>Nothing printed yet</span>
          )}
        </div>
        <div className="flex flex-wrap gap-tap-gap">
          <Button disabled={busy || !p.active} onClick={() => run(() => api.testPage(p.id), "Test page queued. Check the printer.")}>
            Test page
          </Button>
          <Button disabled={busy} onClick={onEdit}>
            Edit
          </Button>
          {p.active ? (
            <Button variant="danger" disabled={busy} onClick={() => run(() => api.retire(p.id), "Retired.")}>
              Retire
            </Button>
          ) : (
            <Button disabled={busy} onClick={() => run(() => api.restore(p.id), "Back in service.")}>
              Bring back
            </Button>
          )}
        </div>
        {message && (
          <p role={message.error ? "alert" : "status"} className={message.error ? "text-sm text-danger" : "text-sm text-fg-2"}>
            {message.text}
          </p>
        )}
      </Card>
    </li>
  );
}

/** Where receipts and each station's tickets print. */
function Routing({ data, api }: { data: PrintersResponse; api: PrinterSettingsApi }) {
  const active = data.printers.filter((p) => p.active);
  const receipt = data.printers.find((p) => p.is_receipt && p.active)?.id ?? null;
  return (
    <Card padding="lg" className="flex flex-col gap-4">
      <h2 className="text-xl font-semibold">Where things print</h2>
      <Assign label="Receipts" value={receipt} printers={active} onChange={(id) => api.setReceipt(id)} />
      {data.stations.length === 0 && <p className="text-base text-fg-muted">No stations yet: they come with the menu.</p>}
      {data.stations.map((s) => {
        const current = s.printer_id !== undefined && active.some((p) => p.id === s.printer_id) ? s.printer_id : null;
        return (
          <Assign key={s.code} label={`${s.name} tickets`} value={current} printers={active} onChange={(id) => api.setStation(s.code, id)} />
        );
      })}
    </Card>
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
  return (
    <label className="grid grid-cols-[minmax(0,12rem)_minmax(0,1fr)] items-center gap-3 max-sm:grid-cols-1">
      <span className="text-base font-semibold">{label}</span>
      <span className="flex flex-wrap items-center gap-3">
        <select
          className="min-h-tap min-w-48 rounded border border-line-strong bg-raised px-3 text-fg outline-offset-2 outline-line-focus focus-visible:outline-2"
          value={value ?? ""}
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
      </span>
    </label>
  );
}

/** The time, moving on each minute, for "12 min ago". */
function useNow(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30_000);
    return () => clearInterval(t);
  }, []);
  return now;
}
