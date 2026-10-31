// The corrections section of the Sales reports screen (backoffice-reports.md,
// phase 10 Task 5e2): the audit trail's voids, comps, discounts, refunds,
// payment voids and reopens over the range, added up by the server
// (ADR-122), each row a way down to the trail itself.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Link } from "wouter";
import type { AuditAnalysis, AuditKindTally, AuditPersonTally, AuditReasonTally } from "../../../shared/api-types";
import { wordsOf } from "../../../shared/api";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Card, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { dayText } from "../../../shared/venue-clock";
import { auditLink, kindWords, kindsText, reasonWords } from "../audit/audit-logic";
import { Section, count } from "./SalesSections";
import { analysisCsvPath, analysisPath, type SalesRange } from "./sales-logic";
import { useCsvDownload } from "./use-csv-download";

const linkClass =
  "font-semibold text-fg underline decoration-line-strong underline-offset-2 outline-offset-2 outline-focus hover:decoration-fg focus-visible:outline-3";

export function Corrections({ range }: { range: SalesRange }) {
  const store = useAuthStore();
  const csv = useCsvDownload();
  const analysis = useQuery({
    queryKey: ["corrections", range],
    queryFn: ({ signal }) => store.api.get<AuditAnalysis>(analysisPath(range), { signal }),
    placeholderData: keepPreviousData,
  });
  const stale = analysis.isPlaceholderData;
  const data = analysis.data;

  return (
    <section aria-label="Corrections" aria-busy={stale} className={cx("flex flex-col gap-6", stale && "opacity-40")}>
      <div>
        <h2 className="font-serif text-xl">Corrections</h2>
        {data && (
          <p className="mt-1 text-base">
            {count(data.total.count)} {data.total.count === 1 ? "correction" : "corrections"} worth <Money amount={data.total.amount} />
          </p>
        )}
        <p className="mt-1 text-sm text-fg-3">What each was worth to its bill, not money out of the drawer. A reopen counts and is worth nothing.</p>
      </div>

      {analysis.isPending && <p className="text-base text-fg-muted">Adding up the corrections</p>}
      {analysis.error && (
        <EmptyState
          icon={<InboxIcon />}
          line={wordsOf(analysis.error, "Can't reach the mez server. Check this computer's connection.")}
          action={{ label: "Try again", onClick: () => void analysis.refetch() }}
        />
      )}
      {data && !analysis.error && data.total.count === 0 && (
        <Card>
          <EmptyState icon={<InboxIcon />} line={`No corrections between ${dayText(data.from)} and ${dayText(data.to)}.`} />
        </Card>
      )}
      {data && !analysis.error && data.total.count > 0 && (
        <>
          <div className="grid gap-6 xl:grid-cols-2">
            <Section title="By kind" csv={null}>
              <Kinds analysis={data} />
            </Section>
            <Section title="By reason" csv={csv(analysisCsvPath(range, "reason"), stale)}>
              <Reasons rows={data.by_reason} />
            </Section>
          </div>
          <div className="grid gap-6 xl:grid-cols-2">
            <Section title="Who did them" csv={csv(analysisCsvPath(range, "by"), stale)}>
              <People label="Corrections by who did them" heading="By" nobody="A guest" rows={data.by} analysis={data} />
            </Section>
            <Section title="Who let them through" csv={csv(analysisCsvPath(range, "authorised_by"), stale)}>
              <People label="Corrections by who authorised them" heading="Authorised by" nobody="Not recorded" rows={data.authorised_by} analysis={data} />
            </Section>
          </div>
          <p className="text-sm text-fg-3">
            A name opens the audit trail on that person. The trail finds what they did and what they authorised, so it may list more than the count beside the
            name.
          </p>
        </>
      )}
    </section>
  );
}

const amountColumn = <T extends { amount: AuditAnalysis["total"]["amount"] }>(): Column<T> => ({
  id: "amount",
  header: "Amount",
  align: "right",
  cell: (r) => <Money amount={r.amount} />,
});

function Kinds({ analysis }: { analysis: AuditAnalysis }) {
  const columns: Column<AuditKindTally>[] = [
    {
      id: "kind",
      header: "Kind",
      // A kind with none has nothing to open.
      cell: (k) =>
        k.count > 0 ? (
          <Link href={auditLink({ from: analysis.from, to: analysis.to, kind: k.kind })} className={linkClass}>
            {kindWords[k.kind].many}
          </Link>
        ) : (
          <span className="text-fg-muted">{kindWords[k.kind].many}</span>
        ),
    },
    { id: "count", header: "Count", align: "right", cell: (k) => count(k.count) },
    amountColumn(),
  ];
  return <DataTable label="Corrections by kind" columns={columns} rows={analysis.by_kind} rowKey={(k) => k.kind} />;
}

function Reasons({ rows }: { rows: readonly AuditReasonTally[] }) {
  const columns: Column<AuditReasonTally>[] = [
    { id: "kind", header: "Kind", cell: (r) => kindWords[r.kind].one },
    { id: "reason", header: "Reason", cell: (r) => reasonWords(r.reason) || <span className="text-fg-muted">No reason given</span> },
    { id: "count", header: "Count", align: "right", cell: (r) => count(r.count) },
    amountColumn(),
  ];
  return (
    <div className="max-h-96 overflow-y-auto">
      <DataTable label="Corrections by reason" columns={columns} rows={rows} rowKey={(r) => `${r.kind} ${r.reason}`} />
    </div>
  );
}

interface PeopleProps {
  label: string;
  heading: string;
  /** What nobody on record is called here: "A guest", "Not recorded". */
  nobody: string;
  rows: readonly AuditPersonTally[];
  analysis: AuditAnalysis;
}

function People({ label, heading, nobody, rows, analysis }: PeopleProps) {
  const who = (p: AuditPersonTally): ReactNode => {
    if (!p.person) return <span className="text-fg-muted">{nobody}</span>;
    // Someone no longer on record has no id to find them by.
    if (p.person.id === "") return <span className="text-fg-muted">{p.person.name}</span>;
    return (
      <Link href={auditLink({ from: analysis.from, to: analysis.to, staff: p.person.id })} className={linkClass}>
        {p.person.name}
      </Link>
    );
  };
  const columns: Column<AuditPersonTally>[] = [
    {
      id: "person",
      header: heading,
      cell: (p) => (
        <span className="flex flex-col">
          {who(p)}
          <span className="text-fg-3">{kindsText(p.kinds)}</span>
        </span>
      ),
    },
    { id: "count", header: "Count", align: "right", cell: (p) => count(p.count) },
    amountColumn(),
  ];
  return (
    <div className="max-h-96 overflow-y-auto">
      <DataTable label={label} columns={columns} rows={rows} rowKey={(p) => (p.person ? `${p.person.id} ${p.person.name}` : "nobody")} />
    </div>
  );
}
