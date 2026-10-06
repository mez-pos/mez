// The sheet of codes to print (backoffice-floor.md, ADR-245): one card for
// each table, to cut out and stick on it. The cards are drawn once, outside
// the app's own tree, and shown only on paper (index.css): what prints is
// the sheet and nothing of the back office around it.

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import type { TableCodeSheetResponse } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { codesProblem, hostOf, sheetSummary } from "./codes-logic";

interface CodeSheetProps {
  venueName: string;
  /** Reads every table's code drawn, at this moment. */
  read: () => Promise<TableCodeSheetResponse>;
  onClose: () => void;
}

export function CodeSheet({ venueName, read, onClose }: CodeSheetProps) {
  const [sheet, setSheet] = useState<TableCodeSheetResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let gone = false;
    read()
      .then((got) => !gone && setSheet(got))
      .catch((err: unknown) => !gone && setError(codesProblem(err)));
    return () => {
      gone = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- read once, when it is opened
  }, []);
  const count = sheet?.tables.length ?? 0;

  return (
    <>
      <Modal
        open
        onClose={onClose}
        title="Print the codes"
        footer={
          <>
            <Button onClick={onClose}>Close</Button>
            <Button variant="primary" disabled={count === 0} onClick={() => window.print()}>
              Print
            </Button>
          </>
        }
      >
        {sheet === null && error === null && <p className="text-base text-fg-muted">Drawing the codes</p>}
        {error && (
          <p role="alert" className="text-base text-danger">
            {error}
          </p>
        )}
        {sheet && (
          <div className="flex flex-col gap-3 text-base">
            <p role="status" className="font-semibold">
              {sheetSummary(count)}
            </p>
            {count > 0 && (
              <>
                <p className="text-fg-2">
                  Each card has the table's name on it. Cut along the dashed lines and put each on its own table: a card on the wrong table sends that table's
                  orders to another.
                </p>
                <p className="text-fg-2">These are the codes as they are now. Renew a code after printing and its card stops working.</p>
              </>
            )}
          </div>
        )}
      </Modal>
      {sheet &&
        count > 0 &&
        createPortal(
          <div className="code-sheet">
            {sheet.tables.map((t) => (
              <section key={t.table_id} className="code-sheet-card">
                <p className="code-sheet-venue">{venueName}</p>
                <p className="code-sheet-ask">Scan to see the menu and order</p>
                <img src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(t.svg)}`} alt="" className="code-sheet-qr" />
                <p className="code-sheet-table">Table {t.table_name}</p>
                <p className="code-sheet-url">{hostOf(t.url)}</p>
              </section>
            ))}
          </div>,
          document.body,
        )}
    </>
  );
}
