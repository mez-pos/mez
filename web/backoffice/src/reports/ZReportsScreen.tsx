// The venue's Z-reports (docs/design/screens/zreport.md): newest first, 50
// a page, each opening the report. A correction is marked as one.

import { useState } from "react";
import { Link } from "wouter";
import type { ZSummary } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { Button, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { dayText } from "../../../shared/venue-clock";
import { reportProblem } from "../../../shared/zreport/zreport-logic";
import { statusOf, useZReports } from "../../../shared/zreport/use-zreport";
import { tillPath } from "../shell/nav-logic";

export function ZReportsScreen() {
  // Each page's cursor; the first page is 0.
  const [pages, setPages] = useState<number[]>([0]);

  return (
    <div className="flex max-w-4xl flex-col gap-4">
      <div>
        <h1 className="font-serif text-total">Z-reports</h1>
        <p className="mt-1 text-base text-fg-muted">One is issued each time a shift closes. Newest first.</p>
      </div>
      <ol aria-label="Z-reports" className="flex flex-col gap-tap-gap">
        {pages.map((before, i) => (
          <Page key={before} before={before} last={i === pages.length - 1} onMore={(next) => setPages((p) => [...p, next])} />
        ))}
      </ol>
    </div>
  );
}

interface PageProps {
  before: number;
  last: boolean;
  onMore: (next: number) => void;
}

function Page({ before, last, onMore }: PageProps) {
  const page = useZReports(before);
  const auth = useAuth();
  // A device that is a till can be sent to it to close the day.
  const till = auth.kind === "signed_in" ? tillPath(auth.device) : null;
  if (page.isPending) return <li className="text-base text-fg-muted">Loading the reports</li>;
  if (page.isError) {
    const status = statusOf(page.error);
    return (
      <li>
        <EmptyState
          icon={<InboxIcon />}
          line={reportProblem(status)}
          {...(status === undefined ? { action: { label: "Try again", onClick: () => void page.refetch() } } : {})}
        />
      </li>
    );
  }
  const { reports, next_before: next } = page.data;
  if (before === 0 && reports.length === 0) {
    return (
      <li>
        {/* The next step, and the way to it where this device is a till (BO-15). */}
        <EmptyState
          icon={<InboxIcon />}
          line="No Z-reports yet. Close the day on the till, from its profile menu, to issue the first."
          {...(till ? { action: { label: "Open the till", href: till } } : {})}
        />
      </li>
    );
  }
  return (
    <>
      {reports.map((z) => (
        <li key={z.number}>
          <ReportRow z={z} />
        </li>
      ))}
      {last && next !== undefined && (
        <li>
          <Button onClick={() => onMore(next)}>Older reports</Button>
        </li>
      )}
    </>
  );
}

function ReportRow({ z }: { z: ZSummary }) {
  return (
    <Link
      href={`/reports/${z.number}`}
      className="grid min-h-tap grid-cols-[6rem_1fr_auto] items-center gap-4 rounded border border-line bg-raised px-4 py-3 outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3"
    >
      <span className="font-mono text-lg tabular-nums">Z {z.number}</span>
      <span className="flex min-w-0 flex-col gap-1">
        <span className="flex items-center gap-2 text-base font-semibold">
          {dayText(z.trading_day)}
          {z.correction && <span className="rounded border border-warning px-2 py-0.5 text-meta font-semibold uppercase text-warning">Correction</span>}
        </span>
        <span className="truncate font-mono text-sm text-fg-muted">{z.shift_public_id}</span>
      </span>
      <Money amount={z.sales_total} />
    </Link>
  );
}
