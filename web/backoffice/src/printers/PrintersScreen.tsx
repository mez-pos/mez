// The venue's printers (docs/design/screens/backoffice-printers.md): the
// printers in a table, and beside it where receipts and each station's
// tickets print; add, edit, test and retire.

import { PlusIcon } from "lucide-react";
import { useState } from "react";
import type { PrinterResponse } from "../../../shared/api-types";
import { Button, Card, EmptyState, InboxIcon } from "../../../shared/ui";
import { emptyForm, formFrom, printersSummary, problemOf, toSettings } from "./printers-logic";
import { PrinterFormModal } from "./PrinterFormModal";
import { PrinterRouting } from "./PrinterRouting";
import { PrinterTable } from "./PrinterTable";
import { usePrinters } from "./use-printers";

type Editing = { kind: "add" } | { kind: "edit"; printer: PrinterResponse };

export function PrintersScreen() {
  const api = usePrinters();
  const [editing, setEditing] = useState<Editing | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const data = api.data;
  // Opening a form is a new step: what the last one was refused for is no longer the news.
  const open = (to: Editing) => {
    setProblem(null);
    setEditing(to);
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Printers</h1>
          {data && <p className="mt-1 text-base text-fg-muted">{printersSummary(data.printers)}</p>}
        </div>
        <Button variant="attention" onClick={() => open({ kind: "add" })}>
          <PlusIcon aria-hidden="true" size={18} />
          Add a printer
        </Button>
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
          <EmptyState icon={<InboxIcon />} line="No printers yet. Add the receipt printer first, then one for each station that needs tickets." />
        </Card>
      )}
      {data && data.printers.length > 0 && (
        <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
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
          onSave={(form) => (editing.kind === "add" ? api.create(toSettings(form)) : api.update(editing.printer.id, toSettings(form)))}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}
