// M08's bill (docs/design/screens/guest-bill.md): what the table has had and
// what it comes to, with its tax, as the venue works it out, and how to pay
// what is left: online where the venue has a gateway, by UPI to the venue's
// own address where it takes UPI, and otherwise where to pay in person.

import type { GuestBillResponse, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { billState, paidTold, payLine, summaryRows, tellFailure, upiAfter, upiLinkOf } from "./bill-logic";
import { tableLabel } from "./Landing";
import { problemOf } from "./menu-logic";
import { chargeOpen, chargeWords, checkoutUrlOf } from "./charge-logic";
import { askingOf, canSplit, peopleRange, restLabel, startingChoice, stepPeople, type SplitChoice, type SplitWay } from "./split-logic";
import { offersReceipt } from "./receipt-logic";
import { useCharge, type ChargeApi } from "./use-charge";
import { keepBill, keptBill } from "./use-receipt";
import { needsSession } from "./session-logic";
import { billKey, phoneName, readUpiQr, tellPaid, useBill } from "./use-guest";

interface BillProps {
  code: string;
  table: GuestTableResponse;
  onBack: () => void;
  onReceipt: () => void;
}

export function Bill({ code, table, onBack, onReceipt }: BillProps) {
  const query = useBill(code);
  const online = useCharge(code);
  const bill = query.data;
  // The bill names itself while it is open. Kept, it is what the receipt is
  // asked for by once the table's code means the next bill (ADR-240).
  const named = bill?.bill;
  useEffect(() => keepBill(code, named), [code, named]);
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
            ) : needsSession(query.error) ? (
              // The table has a bill this phone is not yet part of (ADR-262).
              <>
                <div role="status" className="w-full max-w-sm rounded-card border border-line bg-raised p-5 text-left shadow-rest">
                  <h2 className="font-serif text-lg">Ask a member of staff to accept your first order</h2>
                  <p className="mt-2 text-base text-fg-2">
                    The table&apos;s bill shows on this phone once staff have let it join the table. Order from the menu and they will be asked, or ask
                    them in person.
                  </p>
                </div>
                <Button variant="secondary" onClick={() => void query.refetch()}>
                  Try again
                </Button>
              </>
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

            <PayOnline bill={bill} online={online} />
            <PayByUPI bill={bill} code={code} quiet={bill.online === true} />
            <p className="mt-4 rounded-card border border-line bg-sunken p-4 text-center text-base text-fg-2">
              {payLine(billState(bill), upiLinkOf(bill) !== null)}
            </p>
            {offersReceipt(billState(bill), keptBill(code)) && (
              <Button variant={bill.online || upiLinkOf(bill) !== null ? "secondary" : "primary"} size="charge" className="mt-4 w-full" onClick={onReceipt}>
                See your receipt
              </Button>
            )}
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
// One primary on the screen: where paying online is offered it is that, and
// this is drawn as the secondary.
const upiLoud = "bg-accent text-accent-fg shadow-action hover:bg-accent-hover";
const upiQuiet = "border border-line-strong bg-raised text-fg hover:bg-raised-high";

function PayByUPI({ bill, code, quiet }: { bill: GuestBillResponse; code: string; quiet: boolean }) {
  // The QR is read only once it is asked for.
  const [shown, setShown] = useState(false);
  const link = upiLinkOf(bill);
  if (link === null || !bill.upi) return null;
  return (
    <section aria-label="Pay by UPI" className="mt-4 rounded-card border border-line bg-raised p-4 text-center">
      <a
        href={link}
        className={cx(
          "flex min-h-charge w-full items-center justify-center rounded-pill px-5 text-lg font-bold outline-offset-2 outline-attention focus-visible:outline-3",
          quiet ? upiQuiet : upiLoud,
        )}
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
      <details className="mt-3 border-t border-line-soft pt-3 text-left" onToggle={(e) => setShown(e.currentTarget.open)}>
        <summary className="flex min-h-tap cursor-pointer items-center justify-center text-base font-semibold text-accent-text outline-line-focus focus-visible:outline-2">
          Pay from another phone
        </summary>
        {shown && <UpiQr code={code} amountMinor={bill.upi.amount.minor} alt={`QR code to pay ${formatMoney(bill.upi.amount)} to ${bill.upi.payee} by UPI`} />}
        <p className="mt-2 text-center text-sm text-fg-muted">Scan with any UPI app. It pays {formatMoney(bill.upi.amount)}, what is left on this bill now.</p>
      </details>
    </section>
  );
}

/**
 * The bill's UPI QR, read with the phone's session (ADR-262) and shown from
 * memory. A blank square while it is read, or if it could not be: the link
 * above still pays.
 */
function UpiQr({ code, amountMinor, alt }: { code: string; amountMinor: number; alt: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    const ask = new AbortController();
    let shown: string | null = null;
    readUpiQr(code, amountMinor, ask.signal)
      .then((picture) => {
        shown = URL.createObjectURL(picture);
        setSrc(shown);
      })
      .catch(() => setSrc(null));
    return () => {
      ask.abort();
      if (shown !== null) URL.revokeObjectURL(shown);
    };
  }, [code, amountMinor]);
  if (src === null) return <div aria-hidden="true" className="mx-auto mt-2 block size-58 rounded bg-white" />;
  return <img src={src} alt={alt} width={232} height={232} className="mx-auto mt-2 block size-58 rounded bg-white" />;
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

export const chargeTones = {
  waiting: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  good: "border-line bg-accent-tint text-accent-text",
  bad: "border-line bg-danger-tint text-danger",
} as const;

/**
 * Pay what is left online, through the venue's gateway, and how a payment
 * this phone started stands. The gateway's page is another site: the phone
 * leaves for it, and the bill says what happened when it is back. Whether
 * it was paid is the venue's server's to say, never this page's.
 */
function PayOnline({ bill, online }: { bill: GuestBillResponse; online: ChargeApi }) {
  const [choice, setChoice] = useState<SplitChoice>(startingChoice);
  const phone = phoneName();
  const splits = canSplit(bill, phone);
  const asking = askingOf(bill, splits ? choice : startingChoice, phone);
  const charge = online.charge;
  const words = charge ? chargeWords(charge.status) : null;
  const again = charge && chargeOpen(charge.status) ? checkoutUrlOf(charge.url) : null;
  if (!charge && !bill.online) return null;
  return (
    <section aria-label="Pay online" className="mt-4 rounded-card border border-line bg-raised p-4 text-center">
      {charge && words && (
        <div role="status" aria-live="polite" className={cx("rounded border p-3", chargeTones[words.tone])}>
          <p className="text-base font-semibold">
            {words.title} · {formatMoney(charge.amount)}
          </p>
          <p className="mt-1 text-base">{words.line}</p>
          {again && (
            <a href={again} className="mt-2 inline-flex min-h-tap items-center font-semibold underline outline-line-focus focus-visible:outline-2">
              Open the payment page again
            </a>
          )}
          {/* Until it is given up, an open page holds its part of the bill and nobody else can pay it (ADR-234). */}
          {chargeOpen(charge.status) && (
            <Button variant="secondary" className="mt-2 w-full" disabled={online.lettingGo} onClick={online.letGo}>
              {online.lettingGo ? "One moment" : "I am not paying this"}
            </Button>
          )}
          {!chargeOpen(charge.status) && (
            <Button variant="ghost" className="mt-1" onClick={online.dismiss}>
              OK
            </Button>
          )}
        </div>
      )}
      {/* Not offered while one is open: a second payment for the same bill is money to refund. */}
      {bill.online && !(charge && chargeOpen(charge.status)) && (
        <>
          {splits && <SplitChooser bill={bill} choice={choice} onChoose={setChoice} gap={charge !== undefined} />}
          {asking.ok ? (
            <Button
              variant="primary"
              size="charge"
              className={cx("w-full", (charge || splits) && "mt-3")}
              disabled={online.starting}
              onClick={() => online.pay(asking.request)}
            >
              {online.starting ? "Opening the payment page" : `Pay ${formatMoney(asking.amount)} online`}
            </Button>
          ) : (
            <p role="status" className={cx("text-base text-fg-2", (charge || splits) && "mt-3")}>
              {asking.why}
            </p>
          )}
          <p className="mt-2 text-sm text-fg-muted">Card, UPI and more, on the payment provider's own page. This bill is marked paid by itself.</p>
        </>
      )}
      {online.error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {online.error}
        </p>
      )}
    </section>
  );
}

const ways: readonly { way: SplitWay; label: string }[] = [
  { way: "rest", label: "" },
  { way: "even", label: "Split evenly" },
  { way: "amount", label: "An amount" },
];

interface SplitChooserProps {
  bill: GuestBillResponse;
  choice: SplitChoice;
  onChoose: (choice: SplitChoice) => void;
  /** Whether something is drawn above it in the card. */
  gap: boolean;
}

/**
 * What of the bill this guest pays (M08's Split the bill, ADR-235): all
 * that is left, an equal part among so many, or a figure they type. The
 * figures are the server's; the button under it says the one chosen.
 */
function SplitChooser({ bill, choice, onChoose, gap }: SplitChooserProps) {
  const range = peopleRange(bill);
  return (
    <fieldset className={cx("text-left", gap && "mt-3")}>
      <legend className="font-serif text-lg">Split the bill</legend>
      <div className="mt-2 grid grid-cols-3 gap-2">
        {ways.map(({ way, label }) => (
          <label
            key={way}
            className={cx(
              "flex min-h-tap cursor-pointer items-center justify-center rounded border px-2 text-center text-base font-semibold",
              "has-focus-visible:outline-2 has-focus-visible:outline-line-focus",
              choice.way === way ? "border-accent bg-accent-tint text-accent-text" : "border-line-strong bg-raised text-fg",
            )}
          >
            <input type="radio" name="split" className="sr-only" checked={choice.way === way} onChange={() => onChoose({ ...choice, way })} />
            {way === "rest" ? restLabel(bill) : label}
          </label>
        ))}
      </div>
      {choice.way === "even" && range && (
        <div className="mt-3 flex items-center justify-between gap-3">
          <Button
            aria-label="One fewer person"
            disabled={choice.people <= range.min}
            onClick={() => onChoose({ ...choice, people: stepPeople(bill, choice.people, -1) })}
          >
            −
          </Button>
          <p role="status" className="text-base">
            Among <span className="font-mono tabular-nums">{choice.people}</span> people
          </p>
          <Button
            aria-label="One more person"
            disabled={choice.people >= range.max}
            onClick={() => onChoose({ ...choice, people: stepPeople(bill, choice.people, 1) })}
          >
            +
          </Button>
        </div>
      )}
      {choice.way === "amount" && bill.split && (
        <label className="mt-3 block text-base">
          The amount you are paying, up to {formatMoney(bill.split.unclaimed)}
          <input
            type="text"
            inputMode="decimal"
            autoComplete="off"
            value={choice.typed}
            onChange={(e) => onChoose({ ...choice, typed: e.target.value })}
            className="mt-1 block min-h-tap w-full rounded border border-line-strong bg-raised px-3 text-right font-mono text-lg tabular-nums outline-line-focus focus-visible:outline-2"
          />
        </label>
      )}
    </fieldset>
  );
}
