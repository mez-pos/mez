// The Audit screen (docs/design/screens/backoffice-audit.md): every
// correction to a bill, from the record, with who did it, who let it
// through, why and what it was worth (ADR-117).

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { AuditEntry, AuditPage, StaffListResponse } from "../../../shared/api-types";
import { ApiError } from "../../../shared/api";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { inputClass } from "../FormFields";
import { auditQuery, authorisedText, dayText, filterFromSearch, itemText, kinds, kindWords, reasonWords, type AuditFilter } from "./audit-logic";

export function AuditScreen() {
  const store = useAuthStore();
  // The filter in the address, when the screen was opened on one (a link from the Sales reports).
  const [filter, setFilter] = useState<AuditFilter>(() => filterFromSearch(window.location.search, new Date()));
  const staff = useQuery({
    queryKey: ["staff"],
    queryFn: () => store.api.get<StaffListResponse>("/api/v1/staff").then((r) => r.staff),
  });
  const trail = useInfiniteQuery({
    queryKey: ["audit", filter],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) => store.api.get<AuditPage>(auditQuery(filter, pageParam), { signal }),
    getNextPageParam: (last) => last.next_before,
    enabled: filter.from !== "" && filter.to !== "",
  });
  const entries = trail.data?.pages.flatMap((p) => p.entries) ?? [];
  const counts = trail.data?.pages[0]?.counts;
  const set = (patch: Partial<AuditFilter>) => setFilter((f) => ({ ...f, ...patch }));

  return (
    <div className="flex max-w-6xl flex-col gap-6">
      <div>
        <h1 className="font-serif text-total">Audit</h1>
        <p className="mt-1 text-sm text-fg-3">Every correction to a bill, from the record. Amounts are what the bill itself used.</p>
      </div>

      <div role="group" aria-label="Filter" className="flex flex-wrap items-end gap-4">
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          From
          <input type="date" className={cx(inputClass, "font-mono")} value={filter.from} max={filter.to} onChange={(e) => set({ from: e.target.value })} />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          To
          <input type="date" className={cx(inputClass, "font-mono")} value={filter.to} min={filter.from} onChange={(e) => set({ to: e.target.value })} />
        </label>
        <label className="flex min-w-56 flex-col gap-1.5 text-sm font-semibold">
          Person
          <select className={inputClass} value={filter.staff} onChange={(e) => set({ staff: e.target.value })}>
            <option value="">Anyone</option>
            {/* Opened on someone not on the list (deactivated since): say the trail is filtered. */}
            {filter.staff !== "" && staff.data && !staff.data.some((s) => s.id === filter.staff) && <option value={filter.staff}>Someone no longer active</option>}
            {staff.data?.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div role="group" aria-label="Kinds" className="flex flex-wrap gap-tap-gap">
        {kinds.map((k) => {
          const on = filter.kind === k;
          return (
            <button
              key={k}
              type="button"
              aria-pressed={on}
              onClick={() => set({ kind: on ? null : k })}
              className={cx(
                "flex min-h-tap items-center gap-2 rounded border px-3.5 text-sm outline-offset-2 outline-line-focus focus-visible:outline-2",
                on ? "border-line-strong bg-raised font-semibold text-fg" : "border-line text-fg-2 hover:bg-raised",
              )}
            >
              {kindWords[k].many}
              <span className="font-mono tabular-nums">{counts ? counts[k] : "·"}</span>
            </button>
          );
        })}
      </div>

      {trail.isPending && <p className="text-base text-fg-muted">Reading the record</p>}
      {trail.error && (
        <EmptyState
          icon={<InboxIcon />}
          line={trail.error instanceof ApiError && !trail.error.unreachable ? trail.error.message : "Can't reach the Nofiltr server. Check this computer's connection."}
          action={{ label: "Try again", onClick: () => void trail.refetch() }}
        />
      )}
      {trail.data && (
        <Trail
          entries={entries}
          empty={`No ${filter.kind ? kindWords[filter.kind].many.toLowerCase() : "corrections"} between ${dayText(filter.from)} and ${dayText(filter.to)}.`}
        />
      )}
      {trail.hasNextPage && (
        <Button className="self-start" disabled={trail.isFetchingNextPage} onClick={() => void trail.fetchNextPage()}>
          {trail.isFetchingNextPage ? "Loading" : "Show older"}
        </Button>
      )}
    </div>
  );
}

function Trail({ entries, empty }: { entries: readonly AuditEntry[]; empty: string }) {
  const columns: Column<AuditEntry>[] = [
    {
      id: "when",
      header: "When",
      cell: (e) => <span className="whitespace-nowrap font-mono tabular-nums">{new Date(e.at).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</span>,
    },
    {
      id: "what",
      header: "What",
      cell: (e) => (
        <span className="flex flex-col">
          <span className="font-semibold">{kindWords[e.kind].one}</span>
          {itemText(e) && <span className="text-fg-3">{itemText(e)}</span>}
        </span>
      ),
    },
    { id: "amount", header: "Amount", align: "right", cell: (e) => (e.amount ? <Money amount={e.amount} /> : null) },
    { id: "by", header: "By", cell: (e) => e.by?.name ?? <span className="text-fg-muted">A guest</span> },
    {
      id: "authorised",
      header: "Authorised by",
      cell: (e) => {
        const a = authorisedText(e);
        return <span className={cx(a.muted && "text-fg-muted")}>{a.text}</span>;
      },
    },
    { id: "why", header: "Why", cell: (e) => reasonWords(e.reason) },
    { id: "bill", header: "Bill", cell: (e) => <span className="font-mono">{e.order_id}</span> },
  ];
  return (
    <DataTable label="Corrections" columns={columns} rows={entries} rowKey={(e) => String(e.id)} empty={<EmptyState icon={<InboxIcon />} line={empty} />} />
  );
}
