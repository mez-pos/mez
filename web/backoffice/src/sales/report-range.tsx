// The range the report screens share (D15 to D17). It is kept above the
// routes, so moving from Sales to Item reports keeps the days that were
// picked. It starts at seven days.

import { createContext, useContext, useState, type ReactNode } from "react";
import type { ReportEvery } from "../../../shared/api-types";
import { everyFor, quickRange, quickRangeOf, type QuickRange, type SalesRange } from "./sales-logic";
import { usable } from "./overview-logic";

export interface ReportRange {
  range: SalesRange;
  setRange: (range: SalesRange) => void;
  /** How the range is cut: chosen by hand, or following the range until it is. */
  every: ReportEvery;
  setEvery: (every: ReportEvery) => void;
  /** The quick range these days are, if any. */
  quick: QuickRange | null;
  /** Whether these are days worth asking the server for. */
  picked: boolean;
}

const Context = createContext<ReportRange | null>(null);

export function ReportRangeProvider({ children }: { children: ReactNode }) {
  const [range, setRange] = useState<SalesRange>(() => quickRange("week", new Date()));
  // Null until chosen by hand: until then the cut follows the range.
  const [chosen, setEvery] = useState<ReportEvery | null>(null);
  // Not memoised: which quick range these days are depends on today, and
  // the provider outlives a night left open.
  const value: ReportRange = { range, setRange, every: chosen ?? everyFor(range), setEvery, quick: quickRangeOf(range, new Date()), picked: usable(range) };
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useReportRange(): ReportRange {
  const value = useContext(Context);
  if (!value) throw new Error("useReportRange needs a ReportRangeProvider above it");
  return value;
}
