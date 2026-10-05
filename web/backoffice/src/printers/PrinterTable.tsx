// The printers as a table: each with how it is reached, its paper, what it
// prints and how its last job went. What can be done to one (a test page,
// edit, retire or bring back) is under the row's "...".

import { PrinterIcon } from "lucide-react";
import { useEffect, useState } from "react";
import type { PrinterResponse, StationPrinterResponse } from "../../../shared/api-types";
import { Card, StatusChip, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { RowMenu } from "../RowMenu";
import { agoText, connectionText, printsText, problemOf } from "./printers-logic";
import type { PrinterSettingsApi } from "./use-printers";

interface PrinterTableProps {
  printers: readonly PrinterResponse[];
  stations: readonly StationPrinterResponse[];
  api: PrinterSettingsApi;
  onEdit: (printer: PrinterResponse) => void;
  /** Told what an action was refused for, or null as one starts. */
  onProblem: (message: string | null) => void;
}

const head = "pb-3 text-meta font-bold uppercase text-fg-muted";

export function PrinterTable({ printers, stations, api, onEdit, onProblem }: PrinterTableProps) {
  const toast = useToast();
  const now = useNow();
  // The printer an action is running on: its menu waits for the answer.
  const [busy, setBusy] = useState<number | null>(null);
  const run = (p: PrinterResponse, what: () => Promise<void>, done: string) => {
    setBusy(p.id);
    onProblem(null);
    what()
      .then(() => toast(done, { tone: "success" }))
      .catch((err: unknown) => onProblem(`${p.name}: ${problemOf(err).message}`))
      .finally(() => setBusy(null));
  };

  return (
    <Card padding="lg" className="min-w-0">
      {/* Under 1280px it scrolls sideways; kept tall enough there for a row's menu to open inside. */}
      <div className="min-h-80 overflow-x-auto xl:min-h-0 xl:overflow-visible">
        <table aria-label="Printers" className="w-full min-w-[42rem] text-base">
          <thead>
            <tr className="border-b border-line text-left">
              <th scope="col" className={head}>
                Printer
              </th>
              <th scope="col" className={cx(head, "pr-4")}>
                Paper
              </th>
              <th scope="col" className={cx(head, "pr-4")}>
                Prints
              </th>
              <th scope="col" className={head}>
                Last print
              </th>
              <th scope="col" className={head}>
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {printers.map((p, row) => (
              <tr key={p.id} className="border-b border-line-soft last:border-b-0">
                <td className="py-2 pr-4">
                  <span className="flex items-center gap-2.5">
                    <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-tight bg-raised-high text-fg-muted">
                      <PrinterIcon size={17} />
                    </span>
                    <span className="min-w-0">
                      <strong className={cx("block whitespace-nowrap", !p.active && "font-semibold text-fg-muted")}>{p.name}</strong>
                      <span className="block font-mono text-sm text-fg-muted">{connectionText(p)}</span>
                    </span>
                  </span>
                </td>
                <td className="py-2 pr-4 font-mono tabular-nums text-fg-2">{p.paper_mm}mm</td>
                <td className="whitespace-nowrap py-2 pr-4 text-fg-2">
                  {p.active ? printsText(p, stations) : <span className="text-sm text-fg-muted">Retired</span>}
                </td>
                <td className="py-2 pr-4">
                  {p.last_job ? (
                    <span className="flex flex-col items-start gap-1">
                      <StatusChip status={{ kind: "print", status: p.last_job.state }} />
                      <span className="text-sm text-fg-muted">
                        {p.last_job.label} · {agoText(p.last_job.at, now)}
                      </span>
                      {p.last_job.error && <span className="max-w-md text-sm text-danger">{p.last_job.error}</span>}
                    </span>
                  ) : (
                    <span className="text-sm text-fg-muted">Nothing printed yet</span>
                  )}
                </td>
                <td className="py-2">
                  <span className="flex justify-end">
                    <RowMenu
                      name={p.name}
                      disabled={busy === p.id}
                      up={row >= 3 && row >= printers.length - 2}
                      actions={[
                        ...(p.active ? [{ label: "Test page", run: () => run(p, () => api.testPage(p.id), "Test page queued. Check the printer.") }] : []),
                        { label: "Edit", run: () => onEdit(p) },
                        p.active
                          ? { label: "Retire", run: () => run(p, () => api.retire(p.id), `${p.name} retired.`) }
                          : { label: "Bring back", run: () => run(p, () => api.restore(p.id), `${p.name} is back in service.`) },
                      ]}
                    />
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

/** The time, moving on each half minute, for "12 min ago". */
function useNow(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30_000);
    return () => clearInterval(t);
  }, []);
  return now;
}
