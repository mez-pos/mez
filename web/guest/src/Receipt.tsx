// The receipt (docs/design/screens/guest-receipt.md, ADR-240, ADR-241): a
// settled bill as the invoice the counter issued has it, for a guest to keep.
// It is the venue's document, read whole from the venue: this draws it and
// works nothing out.

import type { GuestReceiptResponse, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import type { BillRow } from "./bill-logic";
import { tableLabel } from "./Landing";
import { issuedText, paidRows, problemWords, receiptProblem, receiptRows, registrationName } from "./receipt-logic";
import { useReceipt } from "./use-receipt";

interface ReceiptProps {
  code: string;
  table: GuestTableResponse;
  /** The bill it is the receipt of, or null when the phone remembers none. */
  bill: string | null;
  onBack: () => void;
}

export function Receipt({ code, table, bill, onBack }: ReceiptProps) {
  const query = useReceipt(code, bill);
  const receipt = query.data;
  const problem = bill === null ? "none_yet" : query.isError ? receiptProblem(query.error) : null;
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-line bg-chrome px-2">
        <button
          type="button"
          aria-label="Back to the bill"
          onClick={onBack}
          className="flex size-tap items-center justify-center rounded-round outline-line-focus hover:bg-raised-high focus-visible:outline-2"
        >
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            className="size-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M19 12H5M11 6l-6 6 6 6" />
          </svg>
        </button>
        <h1 className="font-serif text-xl">Your receipt</h1>
        <p className="mr-2 ml-auto text-sm text-fg-muted">{tableLabel(table.table.name)}</p>
      </header>

      <main className="flex flex-1 flex-col px-4 pb-[max(--spacing(8),env(safe-area-inset-bottom))]">
        {receipt === undefined ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-4 py-16 text-center">
            {problem === null ? (
              <p role="status" className="text-base text-fg-muted">
                Reading your receipt
              </p>
            ) : (
              <>
                <p role="alert" className="max-w-72 text-base text-fg-2">
                  {problemWords(problem)}
                </p>
                {problem !== "none_yet" && (
                  <Button variant="secondary" onClick={() => void query.refetch()}>
                    Try again
                  </Button>
                )}
              </>
            )}
          </div>
        ) : (
          <Paper receipt={receipt} />
        )}
      </main>
    </div>
  );
}

function Paper({ receipt }: { receipt: GuestReceiptResponse }) {
  const issued = issuedText(receipt);
  const { seller } = receipt;
  return (
    <>
      <section aria-label="Invoice" className="mt-4 rounded-card border border-line bg-raised p-4 text-center shadow-rest">
        <h2 className="font-serif text-xl">{seller.name}</h2>
        {seller.legal_name && seller.legal_name !== seller.name && <p className="text-sm text-fg-muted">{seller.legal_name}</p>}
        {seller.registration && (
          <p className="mt-1 text-sm text-fg-muted">
            {registrationName(seller)} <span className="font-mono break-all">{seller.registration}</span>
          </p>
        )}
        <dl className="mt-3 border-t border-line-soft pt-3 text-left">
          <div className="flex items-baseline justify-between gap-3 py-1 text-base">
            <dt>Invoice</dt>
            <dd className="font-mono break-all tabular-nums">{receipt.invoice_no}</dd>
          </div>
          {issued !== "" && (
            <div className="flex items-baseline justify-between gap-3 py-1 text-base">
              <dt>Issued</dt>
              <dd className="text-right">{issued}</dd>
            </div>
          )}
        </dl>
      </section>

      <ul className="mt-4 rounded-card border border-line bg-raised px-4 shadow-rest">
        {receipt.lines.map((line, i) => (
          <li key={i} className="flex items-start justify-between gap-3 border-b border-line-soft py-3 last:border-b-0">
            <div className="min-w-0">
              <p className="text-base">
                <span className="font-mono tabular-nums">{line.qty}</span> × {line.name}
              </p>
              {line.choices.length > 0 && <p className="mt-0.5 text-sm text-fg-muted">{line.choices.join(", ")}</p>}
            </div>
            <p className="shrink-0 font-mono text-base tabular-nums">{formatMoney(line.amount)}</p>
          </li>
        ))}
      </ul>

      <section aria-label="Summary" className="mt-4 rounded-card border border-line bg-raised p-4">
        <dl>
          <Rows rows={receiptRows(receipt)} />
          <div className="mt-2 flex items-baseline justify-between gap-3 border-t border-line pt-3">
            <dt className="font-serif text-lg">Total</dt>
            <dd className="font-serif text-xl">{formatMoney(receipt.total)}</dd>
          </div>
          <Rows rows={paidRows(receipt)} />
        </dl>
      </section>

      <p className="mt-4 text-center text-sm text-fg-muted">
        This is the invoice {seller.name} issued for your table. To keep it, take a screenshot: this page is here for a few weeks.
      </p>
    </>
  );
}

function Rows({ rows }: { rows: readonly BillRow[] }) {
  return (
    <>
      {rows.map((row) => (
        <div key={row.label} className={cx("flex items-baseline justify-between gap-3 py-1 text-base", row.within && "text-sm text-fg-muted")}>
          <dt>{row.label}</dt>
          <dd className="font-mono tabular-nums">
            {row.minus && "−"}
            {formatMoney(row.amount)}
          </dd>
        </div>
      ))}
    </>
  );
}
