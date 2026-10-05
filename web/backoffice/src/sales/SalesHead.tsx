// The head of a report screen (D15, D16): the tabs that move between the
// reports, the title, and beside it the range and Export. The design draws
// the range as one pill; it opens what the old screen had laid out in a row:
// the quick ranges, a first and a last day, and how the range is cut. Export
// lists the screen's CSV files (ADR-119). The design's outlet and order type
// filters are not drawn: a venue is one outlet, and the server does not
// filter a report by how it was served.

import { DownloadIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "wouter";
import type { ReportEvery, ReportSheet } from "../../../shared/api-types";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { insightText } from "./overview-logic";
import { RangePill, headPanel, headPill } from "./RangePill";
import { useReportRange } from "./report-range";
import { everyWords } from "./sales-logic";
import type { CsvFile } from "./use-csv-download";

const everys = Object.keys(everyWords) as ReportEvery[];

// The design's four tabs are Item, Inventory, Staff and Customer. Items and
// inventory are one screen (D16 draws them as one); customers are out of v1.
const tabs = [
  { id: "sales", label: "Sales", href: "/sales" },
  { id: "items", label: "Items and inventory", href: "/sales/items" },
  { id: "staff", label: "Staff", href: "/sales/staff" },
] as const;

export type ReportTab = (typeof tabs)[number]["id"];

interface ReportHeadProps {
  tab: ReportTab;
  title: string;
  /** Whether the screen's figures are cut by day, week or month: only then is the choice offered. */
  cut?: boolean;
  /** The Export menu, or whatever else sits beside the range. */
  children?: ReactNode;
}

export function ReportHead({ tab, title, cut = false, children }: ReportHeadProps) {
  const { range } = useReportRange();
  return (
    <div className="flex flex-col gap-4">
      <nav aria-label="Reports" className="flex flex-wrap gap-2">
        {tabs.map((t) => (
          <Link key={t.id} href={t.href} aria-current={t.id === tab ? "page" : undefined} className={pillClass(t.id === tab)}>
            {t.label}
          </Link>
        ))}
      </nav>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">{title}</h1>
          <p className="mt-1 text-base text-fg-muted">{insightText(range)}</p>
        </div>
        <div className="flex flex-wrap gap-2.5">
          <RangeMenu cut={cut} />
          {children}
        </div>
      </div>
    </div>
  );
}

function RangeMenu({ cut }: { cut: boolean }) {
  const { range, setRange, every, setEvery } = useReportRange();
  return (
    <RangePill range={range} onChange={setRange} note="A sale counts on the trading day its bill was invoiced. A trading day runs 04:00 to 04:00.">
      {cut && (
        <div role="group" aria-label="By" className="flex flex-col gap-1.5 text-sm font-semibold">
          By
          <div className="flex gap-tap-gap">
            {everys.map((e) => (
              <button key={e} type="button" aria-pressed={every === e} className={pillClass(every === e)} onClick={() => setEvery(e)}>
                {everyWords[e]}
              </button>
            ))}
          </div>
        </div>
      )}
    </RangePill>
  );
}

/** Each sales report's file, in the order the Sales screen shows them. */
export const sheets: readonly { id: ReportSheet; label: string }[] = [
  { id: "summary", label: "The range" },
  { id: "over_time", label: "Sales over time" },
  { id: "payments", label: "Payments" },
  { id: "channels", label: "Channels" },
  { id: "taxes", label: "Taxes" },
  { id: "hours", label: "Hours" },
];

export interface ExportFile {
  label: string;
  /** The path its CSV is downloaded from, for the range on screen. */
  path: string;
}

interface ExportMenuProps {
  file: CsvFile;
  files: readonly ExportFile[];
  /** Off while the figures shown are not the range's: a file must match its screen. */
  disabled: boolean;
}

export function ExportMenu({ file, files, disabled }: ExportMenuProps) {
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
        <div ref={panel} id={panelId} role="group" aria-label="Download as CSV" className={cx(headPanel, "w-64 p-2")}>
          <p className="px-4 pb-1 pt-2 text-meta font-bold uppercase text-fg-faint">Download as CSV</p>
          {files.map((f) => (
            <button
              key={f.path}
              type="button"
              onClick={pick(() => void file.download(f.path))}
              className="flex min-h-tap w-full items-center rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
            >
              {f.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
