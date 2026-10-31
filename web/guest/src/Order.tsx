// The order the guest placed (docs/design/screens/guest-order.md): how it
// stands, in the venue's real answer and not a timer, and what was in it as
// the server priced it. Once it is sent it follows the kitchen (ADR-236):
// with the kitchen, being made, ready, as someone there does each.
//
// Where guests pay first (ADR-225 to ADR-228) an order is held until it is
// paid for: the screen says what to pay, and Pay leaves for the gateway's
// page. Whether it was paid is the venue's server's to say, never this
// page's.

import type { GuestRoundProgress, GuestRoundStatus, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { useClock } from "../../shared/use-clock";
import { chargeTones } from "./Bill";
import { itemsText, toPayLabel, totalNote } from "./cart-logic";
import { canPayRound, chargeOpen, checkoutUrlOf, paymentStuck, roundChargeWords, stuckWords } from "./charge-logic";
import { tableLabel } from "./Landing";
import { checkNote, dueOf, kitchenSteps, orderWords, pickupCodeOf, placedText, sentNote, stepIndex, waitingNudge, type Placed } from "./order-logic";
import { useCharge, type ChargeApi } from "./use-charge";

const tones: Readonly<Record<GuestRoundStatus, string>> = {
  waiting: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  unpaid: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  sent: "border-line bg-accent-tint text-accent-text",
  declined: "border-line bg-danger-tint text-danger",
};

/** The tone of an order as it stands: a cancelled one reads as a declined one does. */
function toneOf(status: GuestRoundStatus, progress: GuestRoundProgress | undefined): string {
  return status === "sent" && progress === "cancelled" ? tones.declined : tones[status];
}

interface OrderProps {
  code: string;
  table: GuestTableResponse;
  placed: Placed;
  /** Asks the venue how the order stands now. */
  onRefresh: () => void;
  onMenu: () => void;
  onBill: () => void;
  /** Puts this order's dishes in the cart again (ADR-242). It places nothing. */
  onAgain: () => void;
  /** The venue no longer has this order (GST-3). */
  lost?: boolean;
  /** Since when asking about it has failed, or null (GST-3). */
  failingSince?: number | null;
  /** An order to collect (guest-collect.md): no table and no bill, and a code to be called by. */
  collect?: boolean;
}

export function Order({ code, table, placed, onRefresh, onMenu, onBill, onAgain, collect = false, lost = false, failingSince = null }: OrderProps) {
  const { round } = placed;
  // "3 minutes ago" moves on, and a failing connection is said once it has lasted.
  const now = useClock(15_000);
  const note = checkNote(lost, failingSince, now);
  const nudge = waitingNudge(placed, now);
  const declined = round.status === "declined" || (round.status === "sent" && round.progress === "cancelled");
  // The charge this phone started for this order, if it started one. An
  // order the venue did not hold has none, and this asks nothing.
  const online = useCharge(code, { round: round.id, onAnswered: onRefresh });
  const stuck = paymentStuck(round.status, online.charge?.status);
  const words = stuck ? stuckWords : orderWords(placed, table.venue.name, collect);
  const pickup = pickupCodeOf(round);
  const at = round.status === "sent" ? stepIndex(round.progress) : -1;
  const count = round.lines.reduce((sum, l) => sum + l.qty, 0);
  const due = dueOf(round);
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-line bg-chrome px-4">
        <h1 className="font-serif text-xl">Your order</h1>
        <p className="ml-auto text-sm text-fg-muted">{tableLabel(table.table.name)}</p>
      </header>

      <main className="flex flex-1 flex-col px-4 pb-[max(--spacing(8),env(safe-area-inset-bottom))]">
        <section
          role="status"
          aria-live="polite"
          className={cx("mt-4 rounded-card border p-5 text-center", stuck ? tones.declined : toneOf(round.status, round.progress))}
        >
          <h2 className="font-serif text-xl">{words.title}</h2>
          {words.line !== "" && <p className="mt-2 text-base">{words.line}</p>}
          {/* When it went: a guest can judge a slow venue only by the time (GST-3). */}
          <p className="mt-2 text-sm">{placedText(placed.at, now)}</p>
          {nudge && <p className="mt-2 text-base font-semibold">{nudge}</p>}
          {/* Where it is among the kitchen's steps: each is a word, and the one it is at is said as well as drawn. */}
          {at >= 0 && (
            <ol className="mt-4 grid grid-cols-3 gap-2 text-sm">
              {kitchenSteps.map(({ step, label }, i) => (
                <li
                  key={step}
                  aria-current={i === at ? "step" : undefined}
                  className={cx("border-t-4 pt-2", i <= at ? "border-accent font-semibold" : "border-line-strong text-fg-muted")}
                >
                  {label}
                </li>
              ))}
            </ol>
          )}
        </section>
        {/* What this screen cannot know: the venue no longer has the order, or cannot be reached (GST-3). */}
        {note && (
          <p
            role="alert"
            className={cx(
              "mt-3 rounded-card border p-4 text-base",
              note.tone === "danger" ? "border-line bg-danger-tint text-danger" : "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
            )}
          >
            {note.text}
          </p>
        )}
        {/* What the counter calls out. Large, since it is read at arm's length and said aloud. */}
        {pickup !== null && round.progress !== "cancelled" && (
          <section aria-label="Your pickup code" className="mt-4 rounded-card border border-line bg-raised p-5 text-center shadow-card">
            <h2 className="text-base text-fg-muted">Your code</h2>
            <p className="mt-1 font-mono text-display font-bold tracking-[.16em] text-accent-text">{pickup}</p>
          </section>
        )}
        {round.status === "sent" && round.progress !== "cancelled" && (
          <p className="mt-3 text-center text-sm text-fg-muted">{sentNote(placed.paidFirst, collect)}</p>
        )}

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
            <p className="mt-1 text-sm text-fg-muted">{totalNote(table.venue.tax_added, collect ? "paid" : "bill")}</p>
          ) : (
            <div className="mt-2 flex items-baseline justify-between gap-3 border-t border-line-soft pt-2">
              <p className="text-base">{toPayLabel(table.venue.tax_added)}</p>
              <p className="font-mono text-base font-semibold tabular-nums">{formatMoney(due)}</p>
            </div>
          )}
        </section>

        {round.status === "unpaid" && <PayForOrder placed={placed} online={online} />}

        <div className="mt-auto flex flex-col gap-3 pt-8">
          <Button variant={round.status === "sent" ? "primary" : "secondary"} size="charge" className="w-full" onClick={onMenu}>
            {round.status !== "sent" ? "Back to the menu" : collect ? "Order something else" : "Order more"}
          </Button>
          {/* The same again goes to the cart, to be read and placed from there. */}
          {round.status === "sent" && round.progress !== "cancelled" && (
            <Button variant="secondary" className="w-full" onClick={onAgain}>
              Order this again
            </Button>
          )}
          {/* Turned down or cancelled, the cart it emptied comes back: to change and place again (GST-5). */}
          {declined && (
            <Button variant="primary" size="charge" className="w-full" onClick={onAgain}>
              Put these back in my cart
            </Button>
          )}
          {/* The bill has this order on it only once it is accepted. */}
          {round.status === "sent" && !collect && (
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
            <a href={again} className="mt-2 inline-flex min-h-tap items-center font-semibold underline outline-focus focus-visible:outline-3">
              Open the payment page again
            </a>
          )}
        </div>
      )}
      {/* Not offered while a payment may yet land, or has: a second one is money to refund. */}
      {canPayRound(round.status, charge?.status) && due !== null && (
        <>
          <Button variant="primary" size="charge" className={cx("w-full", charge && "mt-3")} disabled={online.starting} onClick={() => online.pay()}>
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
export function OrderStrip({ placed, venue, onOpen, collect = false }: { placed: Placed; venue: string; onOpen: () => void; collect?: boolean }) {
  const { status, progress } = placed.round;
  const pickup = pickupCodeOf(placed.round);
  return (
    <button
      type="button"
      onClick={onOpen}
      className={cx(
        "mx-auto flex min-h-tap w-full max-w-md items-center justify-between gap-3 border-b px-4 py-2 text-left text-base font-semibold outline-offset-[-3px] outline-focus focus-visible:outline-3",
        toneOf(status, progress),
      )}
    >
      <span role="status">
        Your order: {orderWords(placed, venue, collect).brief}
        {pickup !== null && <span className="font-mono"> · {pickup}</span>}
      </span>
      <span className="shrink-0 underline">View</span>
    </button>
  );
}
