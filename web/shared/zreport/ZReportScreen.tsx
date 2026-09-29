// One Z-report, read by its number, with Print a copy and the way back
// (docs/design/screens/zreport.md). Both apps show a report through this.

import { useState, type ReactNode } from "react";
import type { ZReportResponse } from "../api-types";
import { Button, EmptyState, InboxIcon, useToast } from "../ui";
import { reportProblem } from "./zreport-logic";
import { statusOf, usePrintZReport, useZReport } from "./use-zreport";
import { ZReportView } from "./ZReportView";

interface ZReportScreenProps {
  number: number;
  backLabel: string;
  onBack: () => void;
  /** More beside Print a copy: the back office's Tips. */
  extraActions?: (z: ZReportResponse) => ReactNode;
  /** Under the report: the back office's exports. */
  footer?: (z: ZReportResponse) => ReactNode;
}

export function ZReportScreen({ number, backLabel, onBack, extraActions, footer }: ZReportScreenProps) {
  const report = useZReport(number);
  const print = usePrintZReport();
  const [printing, setPrinting] = useState(false);
  const toast = useToast();

  const back = (
    <Button variant="ghost" onClick={onBack}>
      {backLabel}
    </Button>
  );
  if (report.isPending) {
    return (
      <div className="m-auto flex flex-col items-center gap-4">
        <p className="text-base text-fg-muted">Loading the report</p>
        {back}
      </div>
    );
  }
  if (report.isError) {
    const status = statusOf(report.error);
    return (
      <div className="m-auto flex flex-col items-center gap-4">
        <EmptyState
          icon={<InboxIcon />}
          line={reportProblem(status, number)}
          {...(status === undefined ? { action: { label: "Try again", onClick: () => void report.refetch() } } : {})}
        />
        {back}
      </div>
    );
  }

  const copy = async () => {
    setPrinting(true);
    try {
      await print(number);
      toast(`Printing a copy of Z-report ${number}.`, { tone: "success" });
    } catch (err) {
      toast(`Couldn't print: ${err instanceof Error ? err.message : String(err)}`, { tone: "warning" });
    } finally {
      setPrinting(false);
    }
  };

  return (
    <div className="min-w-0 flex-1 overflow-y-auto px-6 py-6">
      <ZReportView
        z={report.data}
        actions={
          <>
            {extraActions?.(report.data)}
            <Button disabled={printing} onClick={() => void copy()}>
              Print a copy
            </Button>
            {back}
          </>
        }
      />
      {footer?.(report.data)}
    </div>
  );
}
