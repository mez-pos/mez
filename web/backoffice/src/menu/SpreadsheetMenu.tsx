// The Spreadsheet pill in the Menu screen's head (backoffice-menu.md, "The
// Spreadsheet menu"): the things a file does to a menu, each with a
// line saying what it touches. One pill, since the head has no room for
// four and they belong together.

import { ChevronDownIcon, DownloadIcon, FileSpreadsheetIcon, ListChecksIcon, TagIcon, UploadIcon, type LucideIcon } from "lucide-react";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { cx } from "../../../shared/ui/cx";
import { headPanel, headPill } from "../sales/RangePill";
import type { CsvFile } from "../sales/use-csv-download";
import { exportPath } from "./import-logic";

interface SpreadsheetMenuProps {
  /** The menu's export, as a download under way or not. */
  exported: CsvFile;
  /** Off until the menu is read. */
  disabled: boolean;
  onImport: () => void;
  onChoices: () => void;
  onPrices: () => void;
}

interface Choice {
  icon: LucideIcon;
  label: string;
  /** What it does, and what it leaves alone. */
  line: string;
  run: () => void;
}

export function SpreadsheetMenu({ exported, disabled, onImport, onChoices, onPrices }: SpreadsheetMenuProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  const busy = exported.busy !== null;
  const choices: readonly Choice[] = [
    { icon: DownloadIcon, label: "Export the menu", line: "Save every item as a CSV file.", run: () => void exported.download(exportPath) },
    { icon: UploadIcon, label: "Import new items", line: "Add items from a file. Nothing already on the menu changes.", run: onImport },
    { icon: ListChecksIcon, label: "Import choices", line: "Add groups like Size or Milk from a file. Do this before the items.", run: onChoices },
    { icon: TagIcon, label: "Change prices", line: "Give back an export with its prices edited.", run: onPrices },
  ];

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
        className={cx(headPill, "border border-line-strong bg-raised hover:bg-raised-high disabled:pointer-events-none disabled:opacity-40")}
      >
        <FileSpreadsheetIcon aria-hidden="true" size={17} />
        {busy ? "Exporting" : "Spreadsheet"}
        <ChevronDownIcon aria-hidden="true" size={16} className="text-fg-muted" />
      </button>
      {open && (
        <div ref={panel} id={panelId} role="group" aria-label="Spreadsheet" className={cx(headPanel, "w-80 p-2")}>
          {choices.map((c) => (
            <button
              key={c.label}
              type="button"
              onClick={pick(c.run)}
              className="flex min-h-tap w-full items-start gap-3 rounded px-3 py-2.5 text-left outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
            >
              <c.icon aria-hidden="true" size={18} className="mt-0.5 shrink-0 text-fg-muted" />
              <span className="min-w-0">
                <span className="block text-base font-semibold">{c.label}</span>
                <span className="block text-sm text-fg-muted">{c.line}</span>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
