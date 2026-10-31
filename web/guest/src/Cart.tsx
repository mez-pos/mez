// M04's cart and M11a (docs/design/screens/guest-cart.md): what the guest has
// chosen, how many of each, what it comes to, and Place Order. The list is
// the phone's own until then; what is sent is ids and counts, and the venue
// prices it (ADR-210, ADR-213).

import { useEffect, useState, type ReactNode } from "react";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { DietMark } from "../../shared/ui/DietMark";
import { Qty } from "../../shared/ui/Qty";
import { MAX_QTY, itemsText, totalNote } from "./cart-logic";
import { Notice } from "./Notice";
import type { CartApi } from "./use-cart";
import type { OrderApi } from "./use-order";

interface CartProps {
  cart: CartApi;
  order: OrderApi;
  onBack: () => void;
  /** The venue has the order: on to how it stands. */
  onPlaced: () => void;
  /** Left out where there is no bill: an order to collect is paid for by itself. */
  onBill?: (() => void) | undefined;
  /** Ordering to collect: there is no bill, and it is paid for before it is sent. */
  collect?: boolean;
  /** The bill adds tax to the menu's prices (the venue's tax_added, ADR-386). */
  taxAdded: boolean;
  /** The order placed, over the head, sticking with it (GST-4). */
  strip?: ReactNode;
}

export function Cart({ cart, order, onBack, onPlaced, onBill, collect = false, taxAdded, strip }: CartProps) {
  const place = () => {
    void order.place().then((ok) => {
      if (ok) onPlaced();
    });
  };
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      {/* The order's strip and the head stick together: a decline is never scrolled out of sight (GST-4). */}
      <div className="sticky top-0 z-10">
        {strip}
        <header className="flex h-14 items-center gap-3 border-b border-line bg-chrome px-2">
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
          <h1 className="font-serif text-xl">Your cart</h1>
          {cart.count > 0 && <p className="rounded-pill bg-accent-tint px-3 py-1 text-sm font-bold text-accent-text">{itemsText(cart.count)}</p>}
          {cart.count > 0 && !order.placing && <ClearButton onClear={cart.clear} />}
        </header>
      </div>

      <main className="flex flex-1 flex-col px-4 pb-8">
        <Notice text={cart.notice} onDismiss={cart.dismissNotice} />

        {cart.count === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center py-16 text-center">
            <h2 className="font-serif text-xl">Your cart is empty</h2>
            <p className="mt-2 max-w-64 text-base text-fg-2">Add something delicious and it'll show up right here.</p>
            <Button variant="primary" className="mt-5" onClick={onBack}>
              Browse the menu
            </Button>
            {onBill && (
              <Button variant="ghost" className="mt-2" onClick={onBill}>
                See the table's bill
              </Button>
            )}
          </div>
        ) : (
          <>
            <ul className="mt-4 flex flex-col gap-3">
              {cart.lines.map((line) => (
                <li key={line.key} className="rounded-card border border-line bg-raised p-4 shadow-rest">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-start gap-1.5">
                      <DietMark diet={line.item.diet} className="mt-1" />
                      <h2 className="font-serif text-lg leading-snug">{line.item.name}</h2>
                    </div>
                    <p className="shrink-0 font-serif text-lg">{formatMoney(line.total)}</p>
                  </div>
                  {line.options && <p className="mt-1 text-sm text-fg-muted">{line.options}</p>}
                  <div className="mt-3 flex items-center justify-between gap-3">
                    <p className="text-sm text-fg-muted">{line.qty > 1 ? `${formatMoney(line.unit)} each` : ""}</p>
                    {/* Down from one takes the line out: there is no other way to remove a dish. */}
                    <Qty value={line.qty} onChange={(qty) => cart.setQty(line.key, qty)} bounds={{ min: 0, max: MAX_QTY }} label={line.item.name} />
                  </div>
                </li>
              ))}
            </ul>

            <button
              type="button"
              onClick={onBack}
              className="mt-3 flex min-h-tap items-center justify-between gap-3 rounded-card border border-line bg-accent-tint px-4 py-3 text-left outline-offset-2 outline-line-focus focus-visible:outline-2"
            >
              <span>
                <span className="block text-base font-bold text-accent-text">Add more dishes</span>
                <span className="block text-sm text-fg-2">Back to the menu</span>
              </span>
              <span aria-hidden="true" className="text-xl text-accent-text">
                +
              </span>
            </button>

            <section aria-label="Summary" className="mt-4 rounded-card border border-line bg-raised p-4">
              <div className="flex items-baseline justify-between gap-3">
                <h2 className="font-serif text-lg">
                  Total <span className="font-sans text-sm font-normal text-fg-muted">({itemsText(cart.count)})</span>
                </h2>
                <p className="font-serif text-xl">{formatMoney(cart.total)}</p>
              </div>
              <p className="mt-1 text-sm text-fg-muted">{totalNote(taxAdded, collect ? "pay" : "bill")}</p>
            </section>

            <p className="mt-4 text-center text-sm text-fg-muted">Allergies or a special request? Please tell a member of staff.</p>

            {order.error && (
              <p role="alert" className="mt-4 rounded-card border border-line bg-danger-tint p-4 text-base text-danger">
                {order.error}
              </p>
            )}
            <Button variant="primary" size="charge" className="mt-4 w-full" disabled={order.placing} onClick={place}>
              {order.placing ? "Placing your order" : `Place Order · ${formatMoney(cart.total)}`}
            </Button>
          </>
        )}
      </main>
    </div>
  );
}

/**
 * Clear, asked twice: the first tap arms it and says so, a second within a
 * few seconds empties the cart. No dialog for one question on a phone.
 */
function ClearButton({ onClear }: { onClear: () => void }) {
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(t);
  }, [armed]);
  return (
    <Button variant="danger" size="md" className="mr-2 ml-auto" onClick={armed ? onClear : () => setArmed(true)}>
      {armed ? "Tap again to clear" : "Clear"}
    </Button>
  );
}
