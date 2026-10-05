// The two controls beside the Sales reports title (D15): the range, and
// Export. The design draws the range as one pill; it opens what the old
// screen had laid out in a row: the quick ranges, a first and a last day, and
// how the range is cut. Export lists each report's CSV (ADR-119). The design's
// outlet and order type filters are not drawn: a venue is one outlet, and the
// server does not filter a report by how it was served.

import { CalendarRangeIcon, ChevronDownIcon, DownloadIcon } from "lucide-react";
import type { ReportEvery, ReportSheet } from "../../../shared/api-types";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { inputClass } from "../FormFields";
import { rangeText, usable } from "./overview-logic";
import { everyWords, quickRange, quickRanges, type QuickRange, type SalesRange } from "./sales-logic";
import type { CsvFile } from "./use-csv-download";

const everys = Object.keys(everyWords) as ReportEvery[];

const headPill =
  "inline-flex min-h-tap items-center gap-2 whitespace-nowrap rounded-pill px-5 text-base font-semibold outline-offset-2 outline-attention focus-visible:outline-3";
const panelClass = "absolute right-0 top-full z-20 mt-2 rounded-card border border-line bg-raised p-4 shadow-card";

interface RangeMenuProps {
  range: SalesRange;
  /** The quick range these days are, if any. */
  quick: QuickRange | null;
  every: ReportEvery;
  onRange: (range: SalesRange) => void;
  onEvery: (every: ReportEvery) => void;
}

export function RangeMenu({ range, quick, every, onRange, onEvery }: RangeMenuProps) {
  const { open, toggle, box, trigger, panel, panelId, leaves } = useDisclosure();
  const picked = usable(range);

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(headPill, "border border-line-strong bg-raised hover:bg-raised-high")}
      >
        <CalendarRangeIcon aria-hidden="true" size={17} />
        <span className="font-mono tabular-nums">{picked ? rangeText(range) : "Pick the days"}</span>
        <ChevronDownIcon aria-hidden="true" size={16} className={cx("text-fg-muted transition-transform", open && "rotate-180")} />
      </button>
      {open && (
        // tabIndex: a click on the panel itself keeps focus inside it, so it does not close.
        <div
          ref={panel}
          id={panelId}
          tabIndex={-1}
          role="group"
          aria-label="Range"
          className={cx(panelClass, "flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-4")}
        >
          <div role="group" aria-label="Quick ranges" className="flex flex-wrap gap-tap-gap">
            {quickRanges.map((q) => (
              <button
                key={q.id}
                type="button"
                aria-pressed={quick === q.id}
                className={pillClass(quick === q.id)}
                onClick={() => onRange(quickRange(q.id, new Date()))}
              >
                {q.label}
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1.5 text-sm font-semibold">
              From
              <input
                type="date"
                className={cx(inputClass, "font-mono")}
                value={range.from}
                max={range.to}
                onChange={(e) => onRange({ ...range, from: e.target.value })}
              />
            </label>
            <label className="flex flex-col gap-1.5 text-sm font-semibold">
              To
              <input
                type="date"
                className={cx(inputClass, "font-mono")}
                value={range.to}
                min={range.from}
                onChange={(e) => onRange({ ...range, to: e.target.value })}
              />
            </label>
          </div>
          <div role="group" aria-label="By" className="flex flex-col gap-1.5 text-sm font-semibold">
            By
            <div className="flex gap-tap-gap">
              {everys.map((e) => (
                <button key={e} type="button" aria-pressed={every === e} className={pillClass(every === e)} onClick={() => onEvery(e)}>
                  {everyWords[e]}
                </button>
              ))}
            </div>
          </div>
          <p className="text-sm text-fg-3">A sale counts on the trading day its bill was invoiced. A trading day runs 04:00 to 04:00.</p>
        </div>
      )}
    </div>
  );
}

/** Each report's file, in the order the screen shows them. */
export const sheets: readonly { id: ReportSheet; label: string }[] = [
  { id: "summary", label: "The range" },
  { id: "over_time", label: "Sales over time" },
  { id: "items", label: "Items" },
  { id: "categories", label: "Categories" },
  { id: "payments", label: "Payments" },
  { id: "channels", label: "Channels" },
  { id: "staff", label: "Staff" },
  { id: "taxes", label: "Taxes" },
  { id: "hours", label: "Hours" },
];

interface ExportMenuProps {
  file: CsvFile;
  /** The path of a report's CSV for the range on screen. */
  path: (sheet: ReportSheet) => string;
  /** Off while the figures shown are not the range's: a file must match its screen. */
  disabled: boolean;
}

export function ExportMenu({ file, path, disabled }: ExportMenuProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  const busy = file.busy !== null;

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        // Not disabled while a file downloads: a disabled button drops the focus a choice just gave back to it.
        disabled={disabled}
        aria-disabled={busy || undefined}
        onClick={busy ? undefined : toggle}
        className={cx(headPill, "bg-attention text-attention-fg hover:bg-attention-hover disabled:pointer-events-none disabled:opacity-40")}
      >
        <DownloadIcon aria-hidden="true" size={17} />
        {busy ? "Downloading" : "Export"}
      </button>
      {open && (
        <div ref={panel} id={panelId} role="group" aria-label="Download as CSV" className={cx(panelClass, "w-64 p-2")}>
          <p className="px-4 pb-1 pt-2 text-meta font-bold uppercase text-fg-faint">Download as CSV</p>
          {sheets.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={pick(() => void file.download(path(s.id)))}
              className="flex min-h-tap w-full items-center rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
            >
              {s.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
