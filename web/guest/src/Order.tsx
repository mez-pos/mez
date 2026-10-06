// The order the guest placed (docs/design/screens/guest-order.md): how it
// stands, in the venue's real answer and not a timer, and what was in it as
// the server priced it. Task 5's tracking takes this screen over.
//
// Where guests pay first (ADR-225 to ADR-228) an order is held until it is
// paid for: the screen says what to pay, and Pay leaves for the gateway's
// page. Whether it was paid is the venue's server's to say, never this
// page's.

import type { GuestRoundStatus, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { chargeTones } from "./Bill";
import { itemsText } from "./cart-logic";
import { canPayRound, chargeOpen, checkoutUrlOf, paymentStuck, roundChargeWords, stuckWords } from "./charge-logic";
import { tableLabel } from "./Landing";
import { dueOf, statusWords, type Placed } from "./order-logic";
import { useCharge, type ChargeApi } from "./use-charge";

const tones: Readonly<Record<GuestRoundStatus, string>> = {
  waiting: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  unpaid: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  sent: "border-line bg-accent-tint text-accent-text",
  declined: "border-line bg-danger-tint text-danger",
};

interface OrderProps {
  code: string;
  table: GuestTableResponse;
  placed: Placed;
  /** Asks the venue how the order stands now. */
  onRefresh: () => void;
  onMenu: () => void;
  onBill: () => void;
}

export function Order({ code, table, placed, onRefresh, onMenu, onBill }: OrderProps) {
  const { round } = placed;
  // The charge this phone started for this order, if it started one. An
  // order the venue did not hold has none, and this asks nothing.
  const online = useCharge(code, { round: round.id, onAnswered: onRefresh });
  const stuck = paymentStuck(round.status, online.charge?.status);
  const words = stuck ? stuckWords : statusWords(round.status, table.venue.name, placed.paidFirst);
  const count = round.lines.reduce((sum, l) => sum + l.qty, 0);
  const due = dueOf(round);
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-line bg-chrome px-4">
        <h1 className="font-serif text-xl">Your order</h1>
        <p className="ml-auto text-sm text-fg-muted">{tableLabel(table.table.name)}</p>
      </header>

      <main className="flex flex-1 flex-col px-4 pb-8">
        <section role="status" aria-live="polite" className={cx("mt-4 rounded-card border p-5 text-center", stuck ? tones.declined : tones[round.status])}>
          <h2 className="font-serif text-xl">{words.title}</h2>
          <p className="mt-2 text-base">{words.line}</p>
        </section>

        <ul className="mt-4 rounded-card border border-line bg-raised px-4 shadow-rest">
          {round.lines.map((line, i) => (
            <li key={i} className="flex items-start justify-between gap-3 border-b border-line-soft py-3 last:border-b-0">
              <div className="min-w-0">
                <p className="text-base">
                  <span className="font-mono tabular-nums">{line.qty}</span> × {line.name}
                </p>
                {line.choices.length > 0 && <p className="mt-0.5 text-sm text-fg-muted">{line.choices.join(", ")}</p>}
              </div>
              <p className="shrink-0 font-mono text-base tabular-nums">{formatMoney(line.total)}</p>
            </li>
          ))}
        </ul>

        <section aria-label="Summary" className="mt-4 rounded-card border border-line bg-raised p-4">
          <div className="flex items-baseline justify-between gap-3">
            <h2 className="font-serif text-lg">
              Total <span className="font-sans text-sm font-normal text-fg-muted">({itemsText(count)})</span>
            </h2>
            <p className="font-serif text-xl">{formatMoney(round.subtotal)}</p>
          </div>
          {due === null ? (
            <p className="mt-1 text-sm text-fg-muted">Before any taxes and charges, which are on the bill.</p>
          ) : (
            <div className="mt-2 flex items-baseline justify-between gap-3 border-t border-line-soft pt-2">
              <p className="text-base">To pay, with taxes and charges</p>
              <p className="font-mono text-base font-semibold tabular-nums">{formatMoney(due)}</p>
            </div>
          )}
        </section>

        {round.status === "unpaid" && <PayForOrder placed={placed} online={online} />}

        <div className="mt-auto flex flex-col gap-3 pt-8">
          <Button variant={round.status === "sent" ? "primary" : "secondary"} size="charge" className="w-full" onClick={onMenu}>
            {round.status === "sent" ? "Order more" : "Back to the menu"}
          </Button>
          {/* The bill has this order on it only once it is accepted. */}
          {round.status === "sent" && (
            <Button variant="secondary" className="w-full" onClick={onBill}>
              See the table's bill
            </Button>
          )}
        </div>
      </main>
    </div>
  );
}

/**
 * Pay for an order the venue is holding. The button starts a charge and the
 * phone leaves for the gateway's page; back here, the charge is asked about
 * until it has an answer, and the order is then read again.
 */
function PayForOrder({ placed, online }: { placed: Placed; online: ChargeApi }) {
  const { round } = placed;
  const charge = online.charge;
  const words = charge ? roundChargeWords(charge.status) : null;
  const again = charge && chargeOpen(charge.status) ? checkoutUrlOf(charge.url) : null;
  const due = dueOf(round);
  return (
    <section aria-label="Pay for this order" className="mt-4 rounded-card border border-line bg-raised p-4 text-center">
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
        </div>
      )}
      {/* Not offered while a payment may yet land, or has: a second one is money to refund. */}
      {canPayRound(round.status, charge?.status) && due !== null && (
        <>
          <Button variant="primary" size="charge" className={cx("w-full", charge && "mt-3")} disabled={online.starting} onClick={online.pay}>
            {online.starting ? "Opening the payment page" : `Pay ${formatMoney(due)}`}
          </Button>
          <p className="mt-2 text-sm text-fg-muted">Card, UPI and more, on the payment provider's own page. Your order is sent as soon as it is paid.</p>
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

/**
 * The order's state over the menu and the cart, so a guest who went back to
 * browse still sees it and can return to it.
 */
export function OrderStrip({ placed, venue, onOpen }: { placed: Placed; venue: string; onOpen: () => void }) {
  const status = placed.round.status;
  return (
    <button
      type="button"
      onClick={onOpen}
      className={cx(
        "mx-auto flex min-h-tap w-full max-w-md items-center justify-between gap-3 border-b px-4 py-2 text-left text-base font-semibold outline-offset-[-2px] outline-line-focus focus-visible:outline-2",
        tones[status],
      )}
    >
      <span role="status">Your order: {statusWords(status, venue, placed.paidFirst).brief}</span>
      <span className="shrink-0 underline">View</span>
    </button>
  );
}
