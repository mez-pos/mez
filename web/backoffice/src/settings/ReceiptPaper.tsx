// A sample receipt as the printer would put it on paper, drawn from the lines
// the server laid out with the code that prints a real one (backoffice-settings.md,
// "Receipt"). Columns are characters: the paper is as wide as the printer's line.

import type { CSSProperties } from "react";
import type { ReceiptPreviewLine, ReceiptPreviewResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";

// A real monospace: the numeral face is not one, and a receipt's columns only
// line up in a face where every character is as wide as the next.
const PAPER_FONT: CSSProperties = { fontFamily: 'ui-monospace, "Cascadia Mono", Consolas, Menlo, monospace', fontSize: "13px", lineHeight: 1.45 };

export function ReceiptPaper({ preview }: { preview: ReceiptPreviewResponse }) {
  return (
    <div className="relative overflow-x-auto">
      <div
        role="img"
        aria-label={`A sample receipt, ${preview.columns} characters wide`}
        className="mx-auto rounded border border-line bg-raised px-4 py-5 shadow-rest"
        style={{ ...PAPER_FONT, width: `calc(${preview.columns}ch + 2rem + 2px)` }}
      >
        {preview.lines.map((l, i) => (
          <PaperLine key={i} line={l} />
        ))}
      </div>
    </div>
  );
}

function PaperLine({ line: l }: { line: ReceiptPreviewLine }) {
  if (l.barcode) {
    return (
      <div className="flex flex-col items-center py-1" aria-hidden="true">
        <div
          className="h-9 w-3/4"
          style={{ background: "repeating-linear-gradient(90deg, var(--body) 0 2px, transparent 2px 5px, var(--body) 5px 6px, transparent 6px 9px)" }}
        />
        <div className="text-center" style={{ fontSize: "11px" }}>
          {l.text}
        </div>
      </div>
    );
  }
  return (
    <div
      className={cx("whitespace-pre", l.bold && "font-bold", l.height > 1 && "py-1")}
      style={{ textAlign: l.align as CSSProperties["textAlign"], minHeight: "1.45em" }}
    >
      {l.text}
    </div>
  );
}
