// M08's bill (docs/design/screens/guest-bill.md): what the table has had and
// what it comes to, with its tax, as the venue works it out. Paying from the
// phone is 4e; until then this says where to pay.

import type { GuestBillResponse, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { billState, paidTold, payLine, summaryRows, tellFailure, upiAfter, upiLinkOf } from "./bill-logic";
import { tableLabel } from "./Landing";
import { problemOf } from "./menu-logic";
import { billKey, tellPaid, upiQrUrl, useBill } from "./use-guest";

interface BillProps {
  code: string;
  table: GuestTableResponse;
  onBack: () => void;
}

export function Bill({ code, table, onBack }: BillProps) {
  const query = useBill(code);
  const bill = query.data;
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-line bg-chrome px-2">
        <button
          type="button"
          aria-label="Back to the menu"
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
        <h1 className="font-serif text-xl">The bill</h1>
        <p className="mr-2 ml-auto text-sm text-fg-muted">{tableLabel(table.table.name)}</p>
      </header>

      <main className="flex flex-1 flex-col px-4 pb-8">
        {bill === undefined ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-4 py-16 text-center">
            {query.isPending ? (
              <p role="status" className="text-base text-fg-muted">
                Reading the bill
              </p>
            ) : (
              <>
                <p role="alert" className="max-w-64 text-base text-fg-2">
                  {problemOf(query.error) === "unreachable"
                    ? "Can't reach the kitchen to read the bill. Check your connection and try again."
                    : "The bill could not be read. Try again, or ask a member of staff."}
                </p>
                <Button variant="secondary" onClick={() => void query.refetch()}>
                  Try again
                </Button>
              </>
            )}
          </div>
        ) : (
          <>
            {bill.lines.length > 0 && (
              <ul className="mt-4 rounded-card border border-line bg-raised px-4 shadow-rest">
                {bill.lines.map((line, i) => (
                  <li key={i} className="flex items-start justify-between gap-3 border-b border-line-soft py-3 last:border-b-0">
                    <div className="min-w-0">
                      <p className="text-base">
                        <span className="font-mono tabular-nums">{line.qty}</span> × {line.name}
                      </p>
                      {line.choices.length > 0 && <p className="mt-0.5 text-sm text-fg-muted">{line.choices.join(", ")}</p>}
                    </div>
                    {line.comped ? (
                      <p className="shrink-0 text-base font-semibold text-accent-text">On the house</p>
                    ) : (
                      <p className="shrink-0 font-mono text-base tabular-nums">{formatMoney(line.amount)}</p>
                    )}
                  </li>
                ))}
              </ul>
            )}

            {bill.open && (
              <section aria-label="Summary" className="mt-4 rounded-card border border-line bg-raised p-4">
                <dl>
                  {summaryRows(bill).map((row) => (
                    <div key={row.label} className={cx("flex items-baseline justify-between gap-3 py-1 text-base", row.within && "text-sm text-fg-muted")}>
                      <dt>{row.label}</dt>
                      <dd className="font-mono tabular-nums">
                        {row.minus && "−"}
                        {formatMoney(row.amount)}
                      </dd>
                    </div>
                  ))}
                  <div className="mt-2 flex items-baseline justify-between gap-3 border-t border-line pt-3">
                    <dt className="font-serif text-lg">Bill total</dt>
                    <dd className="font-serif text-xl">{formatMoney(bill.total)}</dd>
                  </div>
                  {bill.paid.minor > 0 && (
                    <>
                      <div className="flex items-baseline justify-between gap-3 py-1 text-base">
                        <dt>Paid</dt>
                        <dd className="font-mono tabular-nums">{formatMoney(bill.paid)}</dd>
                      </div>
                      <div className="flex items-baseline justify-between gap-3 py-1 text-base font-bold">
                        <dt>Left to pay</dt>
                        <dd className="font-mono tabular-nums">{formatMoney(bill.balance_due)}</dd>
                      </div>
                    </>
                  )}
                </dl>
              </section>
            )}

            <PayByUPI bill={bill} code={code} />
            <p className="mt-4 rounded-card border border-line bg-sunken p-4 text-center text-base text-fg-2">
              {payLine(billState(bill), upiLinkOf(bill) !== null)}
            </p>
            <p className="mt-3 text-center text-sm text-fg-muted">This is the whole table's bill, whoever ordered.</p>
          </>
        )}
      </main>
    </div>
  );
}

/**
 * Pay what is left by UPI, when the venue takes it. A link and not a
 * button: the phone hands it to its UPI app, and this page learns nothing
 * of what happens there.
 */
function PayByUPI({ bill, code }: { bill: GuestBillResponse; code: string }) {
  const link = upiLinkOf(bill);
  if (link === null || !bill.upi) return null;
  return (
    <section aria-label="Pay by UPI" className="mt-4 rounded-card border border-line bg-raised p-4 text-center">
      <a
        href={link}
        className="flex min-h-charge w-full items-center justify-center rounded-pill bg-accent px-5 text-lg font-bold text-accent-fg shadow-action outline-offset-2 outline-attention hover:bg-accent-hover focus-visible:outline-3"
      >
        Pay {formatMoney(bill.upi.amount)} by UPI
      </a>
      <p className="mt-3 text-sm text-fg-muted">
        To {bill.upi.payee}, <span className="font-mono break-all">{bill.upi.address}</span>
      </p>
      {/* Once staff are told, the green line under it says the same and more. */}
      {bill.paid_told !== true && <p className="mt-2 text-base text-fg-2">{upiAfter}</p>}
      <IHavePaid code={code} told={bill.paid_told === true} />
      {/* For paying from another phone. Closed until asked for: a code on screen invites a scan by the next table. */}
      <details className="mt-3 border-t border-line-soft pt-3 text-left">
        <summary className="flex min-h-tap cursor-pointer items-center justify-center text-base font-semibold text-accent-text outline-line-focus focus-visible:outline-2">
          Pay from another phone
        </summary>
        <img
          src={upiQrUrl(code, bill.upi.amount.minor)}
          alt={`QR code to pay ${formatMoney(bill.upi.amount)} to ${bill.upi.payee} by UPI`}
          width={232}
          height={232}
          loading="lazy"
          className="mx-auto mt-2 block size-58 rounded bg-white"
        />
        <p className="mt-2 text-center text-sm text-fg-muted">Scan with any UPI app. It pays {formatMoney(bill.upi.amount)}, what is left on this bill now.</p>
      </details>
    </section>
  );
}

/**
 * "I have paid": tells staff to look for the payment. Whether they have been
 * told is the server's to say, on the bill, so it reads the same after a
 * reload and on every phone at the table.
 */
function IHavePaid({ code, told }: { code: string; told: boolean }) {
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  if (told) {
    return (
      <p role="status" className="mt-3 rounded border border-line bg-accent-tint p-3 text-base font-semibold text-accent-text">
        {paidTold}
      </p>
    );
  }
  const tell = () => {
    setBusy(true);
    setError("");
    tellPaid(code)
      .catch((err: unknown) => setError(tellFailure(err)))
      // Told or refused, the bill is what says where things stand.
      .then(() => queryClient.invalidateQueries({ queryKey: billKey(code) }))
      .finally(() => setBusy(false));
  };
  return (
    <>
      <Button variant="secondary" className="mt-3 w-full" disabled={busy} onClick={tell}>
        {busy ? "Telling staff" : "I have paid"}
      </Button>
      {error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {error}
        </p>
      )}
    </>
  );
}
