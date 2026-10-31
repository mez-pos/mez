// M01, the landing (docs/design/screens/guest-landing.md): where the code on
// the table led. It says which table the guest is at and whose it is, and
// has one thing to do. The venue's name is the brand here; mez is said small
// at the foot (GST-10).

import type { GuestTableResponse } from "../../shared/api-types";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { Logo } from "../../shared/ui/Logo";

interface LandingProps {
  table: GuestTableResponse;
  /** Ordering to collect: the guest is at no table. */
  collect?: boolean;
  onStart: () => void;
}

/** "Table T4" for a table named "T4", "Window table" left as it is, and "To collect" for no table. */
export function tableLabel(name: string): string {
  if (name === "") return "To collect";
  return /^table\b/i.test(name) || /\btable$/i.test(name) ? name : `Table ${name}`;
}

/** "Ordering by mez", small, at the foot of the landing and the menu: the software, not the venue (GST-10). */
export function OrderingBy({ className }: { className?: string }) {
  return (
    <p className={cx("flex items-center justify-center gap-1.5 text-sm text-fg-muted", className)}>
      Ordering by <Logo className="h-4" />
    </p>
  );
}

export function Landing({ table, collect = false, onStart }: LandingProps) {
  return (
    // On a phone the button is at the foot, where a thumb is. On a screen wider
    // than the column the whole stands in the middle, the button under the
    // card, not far below it (GST-14).
    <main className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas px-5 pt-10 pb-[max(--spacing(8),env(safe-area-inset-bottom))] text-center text-fg md:justify-center">
      {/* The venue's name where mez's logo was, and no claim made for it (GST-10). */}
      <p className="text-base text-fg-muted">Welcome to</p>
      <p className="mt-1 font-serif text-total leading-tight">{table.venue.name}</p>

      <div className="mt-8 rounded-sheet border border-line bg-raised p-6 shadow-card">
        <p className="text-base text-fg-muted">{collect ? "Order ahead" : "You're at"}</p>
        <h1 className="mt-1 font-serif text-total text-accent-text">{collect ? "Order to collect" : tableLabel(table.table.name)}</h1>
        <hr className="my-4 border-line" />
        <p className="text-base text-fg-2">
          {collect ? "Order and pay here. We give you a code, and call it at the counter when your order is ready." : "The whole menu, at your table."}
        </p>
      </div>

      <div className="mt-auto pt-8 md:mt-0">
        <Button variant="primary" size="charge" className="w-full" onClick={onStart}>
          Start ordering
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            className="ml-2.5 size-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M5 12h14M13 6l6 6-6 6" />
          </svg>
        </Button>
        <OrderingBy className="mt-5" />
      </div>
    </main>
  );
}
