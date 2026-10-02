// Find a settled bill again (terminal-past-bills.md): the latest closed
// bills, or those an invoice or credit note number finds. A barcode scanner
// types the receipt's number into the search and presses Enter.

import { useState } from "react";
import type { PastBillResponse } from "../../../shared/api-types";
import { EmptyState, InboxIcon, Money, SearchField, SearchIcon, Sheet } from "../../../shared/ui";
import { failureOf } from "../register/order-messages";
import { closedText, whereOf } from "./bills-logic";
import { usePastBills } from "./use-bills";

interface PastBillsSheetProps {
  onPick: (bill: PastBillResponse) => void;
  onClose: () => void;
}

export function PastBillsSheet({ onPick, onClose }: PastBillsSheetProps) {
  const [query, setQuery] = useState("");
  const bills = usePastBills(query);
  const now = new Date();
  return (
    <Sheet open onClose={onClose} title="Past bills">
      <div className="flex flex-col gap-4 px-6 py-4">
        <SearchField label="Invoice or credit note number" onSearch={setQuery} />
        {bills.isPending && <p className="text-base text-fg-muted">Looking</p>}
        {bills.isError && (
          <p role="alert" className="text-base text-danger">
            {failureOf(bills.error)}
          </p>
        )}
        {bills.data && bills.data.bills.length === 0 && (
          <EmptyState
            icon={query ? <SearchIcon /> : <InboxIcon />}
            line={query ? `No bill with the number "${query}".` : "No bills have been closed yet."}
          />
        )}
        <ol>
          {bills.data?.bills.map((b) => (
            <li key={b.id} className="border-b border-line-soft">
              <button
                type="button"
                onClick={() => onPick(b)}
                className="grid min-h-tap w-full grid-cols-[1fr_auto] items-center gap-x-4 gap-y-1 py-3 text-left outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
              >
                <span className="font-mono text-base">{b.invoice_no || b.id}</span>
                <Money amount={b.total} />
                <span className="text-sm text-fg-muted">
                  {whereOf(b)} <span className="font-mono">{closedText(b.closed_at, now)}</span>
                </span>
                {b.credited.minor > 0 ? (
                  <span className="flex items-center justify-end gap-2 text-sm text-danger">
                    Credited <Money amount={b.credited} />
                  </span>
                ) : (
                  <span />
                )}
              </button>
            </li>
          ))}
        </ol>
      </div>
    </Sheet>
  );
}
