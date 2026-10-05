// The order the guest placed (docs/design/screens/guest-order.md): how it
// stands, in the venue's real answer and not a timer, and what was in it as
// the server priced it. Task 5's tracking takes this screen over.

import type { GuestRoundStatus, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { itemsText } from "./cart-logic";
import { tableLabel } from "./Landing";
import { statusWords, type Placed } from "./order-logic";

const tones: Readonly<Record<GuestRoundStatus, string>> = {
  waiting: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
  sent: "border-line bg-accent-tint text-accent-text",
  declined: "border-line bg-danger-tint text-danger",
};

interface OrderProps {
  table: GuestTableResponse;
  placed: Placed;
  onMenu: () => void;
}

export function Order({ table, placed, onMenu }: OrderProps) {
  const { round } = placed;
  const words = statusWords(round.status, table.venue.name);
  const count = round.lines.reduce((sum, l) => sum + l.qty, 0);
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-line bg-chrome px-4">
        <h1 className="font-serif text-xl">Your order</h1>
        <p className="ml-auto text-sm text-fg-muted">{tableLabel(table.table.name)}</p>
      </header>

      <main className="flex flex-1 flex-col px-4 pb-8">
        <section role="status" aria-live="polite" className={cx("mt-4 rounded-card border p-5 text-center", tones[round.status])}>
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
          <p className="mt-1 text-sm text-fg-muted">Before any taxes and charges, which are on the bill.</p>
        </section>

        <div className="mt-auto pt-8">
          <Button variant={round.status === "sent" ? "primary" : "secondary"} size="charge" className="w-full" onClick={onMenu}>
            {round.status === "sent" ? "Order more" : "Back to the menu"}
          </Button>
        </div>
      </main>
    </div>
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
      <span role="status">Your order: {statusWords(status, venue).brief}</span>
      <span className="shrink-0 underline">View</span>
    </button>
  );
}
