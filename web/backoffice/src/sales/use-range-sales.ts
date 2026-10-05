// The sales report for the range the report screens share, and the same
// number of days just before it (ADR-118, ADR-185). Both report screens read
// through here, so moving between them asks the server nothing new.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { SalesReport } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { previousRange } from "./overview-logic";
import { useReportRange } from "./report-range";
import { salesPath } from "./sales-logic";

export function useRangeSales() {
  const store = useAuthStore();
  const { range, every, picked } = useReportRange();
  const before = picked ? previousRange(range) : null;

  const report = useQuery({
    queryKey: ["sales", range, every],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath(range, every), { signal }),
    enabled: picked,
    // The last range's figures stay, dimmed, while the next is read.
    placeholderData: keepPreviousData,
  });
  // Its totals alone are read, so it is cut as coarsely as the server allows.
  const prior = useQuery({
    queryKey: ["sales", before, "month"],
    queryFn: ({ signal }) => store.api.get<SalesReport>(salesPath(before ?? range, "month"), { signal }),
    enabled: before !== null,
  });

  /** The figures shown are the last range's, kept while the next is read. */
  const stale = report.isPlaceholderData;
  return {
    report,
    /** The range's report, or null while there is none to show. */
    shown: picked && report.data && !report.error ? report.data : null,
    /** The days before, once the range on screen is the one asked for. */
    prior: stale ? null : (prior.data ?? null),
    /** The days before could not be read: there is nothing to compare with. */
    priorFailed: prior.isError,
    stale,
  };
}
