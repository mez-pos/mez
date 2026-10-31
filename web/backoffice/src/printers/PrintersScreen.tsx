// The venue's printers (docs/design/screens/backoffice-printers.md): the
// printers in a table, and beside it where receipts and each station's
// tickets print; add, edit, test and retire.

import { PlusIcon } from "lucide-react";
import { useState } from "react";
import type { PrinterResponse } from "../../../shared/api-types";
import { Button, Card, EmptyState, InboxIcon } from "../../../shared/ui";
import {
  addedPrinterId,
  defaultPrints,
  emptyForm,
  formFrom,
  printersSummary,
  printsOptions,
  problemOf,
  toSettings,
  type PrinterForm,
  type PrintsFor,
} from "./printers-logic";
import { PrinterFormModal } from "./PrinterFormModal";
import { AtTheVenue } from "../../../shared/shell/AtTheVenue";
import { useAway } from "../../../shared/use-realtime";
import { PrinterRouting } from "./PrinterRouting";
import { PrinterTable } from "./PrinterTable";
import { usePrinters } from "./use-printers";

type Editing = { kind: "add" } | { kind: "edit"; printer: PrinterResponse };

export function PrintersScreen() {
  const away = useAway();
  const api = usePrinters();
  const [editing, setEditing] = useState<Editing | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const data = api.data;
  // Opening a form is a new step: what the last one was refused for is no longer the news.
  const open = (to: Editing) => {
    setProblem(null);
    setEditing(to);
  };
  // Adds the printer, then sets it to what it was said to print (BO-15). The
  // printer is made either way: a failure to set it is said here, not in the
  // form, where saving again would add a second.
  const add = async (form: PrinterForm, prints: PrintsFor) => {
    const after = await api.create(toSettings(form));
    const id = addedPrinterId(data, after);
    if (id === null || prints === "later") return;
    const station = prints.startsWith("station:") ? prints.slice("station:".length) : null;
    try {
      if (station === null) await api.setReceipt(id);
      else await api.setStation(station, id);
    } catch (err) {
      const what =
        printsOptions(after.stations)
          .find((o) => o.value === prints)
          ?.label.toLowerCase() ?? "that";
      setProblem(`The printer was added, and could not be set to print ${what}: ${problemOf(err).message} Pick it under Where things print.`);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Printers</h1>
          {data && <p className="mt-1 text-base text-fg-muted">{printersSummary(data.printers)}</p>}
        </div>
        {away ? (
          <AtTheVenue>Printers are added, tested and changed at the venue.</AtTheVenue>
        ) : (
          <Button variant="attention" onClick={() => open({ kind: "add" })}>
            <PlusIcon aria-hidden="true" size={18} />
            Add a printer
          </Button>
        )}
      </div>

      {problem && (
        <p role="alert" className="text-base text-danger">
          {problem}
        </p>
      )}
      {api.loading && <p className="text-base text-fg-muted">Loading the printers</p>}
      {!api.loading && api.error !== null && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
      )}
      {data && data.printers.length === 0 && (
        <Card>
          <EmptyState
            icon={<InboxIcon />}
            line={away ? "No printers yet." : "No printers yet. Add the receipt printer first, then one for each station that needs tickets."}
            {...(away ? {} : { action: { label: "Add a printer", onClick: () => open({ kind: "add" }) } })}
          />
        </Card>
      )}
      {data && data.printers.length > 0 && (
        <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_var(--aside)]">
          <PrinterTable
            printers={data.printers}
            stations={data.stations}
            api={api}
            onEdit={(printer) => open({ kind: "edit", printer })}
            onProblem={setProblem}
          />
          <PrinterRouting data={data} api={api} />
        </div>
      )}

      {editing && (
        <PrinterFormModal
          title={editing.kind === "add" ? "Add a printer" : `Edit ${editing.printer.name}`}
          initial={editing.kind === "add" ? emptyForm() : formFrom(editing.printer)}
          prints={editing.kind === "add" && data ? { options: printsOptions(data.stations), initial: defaultPrints(data) } : undefined}
          onSave={(form, prints) => (editing.kind === "add" ? add(form, prints) : api.update(editing.printer.id, toSettings(form)))}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}
