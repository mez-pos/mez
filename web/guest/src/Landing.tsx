// M01, the landing (docs/design/screens/guest-landing.md): where the code on
// the table led. It says which table the guest is at and whose it is, and
// has one thing to do.

import type { GuestTableResponse } from "../../shared/api-types";
import { Button } from "../../shared/ui/Button";
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

export function Landing({ table, collect = false, onStart }: LandingProps) {
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-canvas px-5 pt-10 pb-8 text-center text-fg">
      <Logo className="mx-auto h-12" />
      <p className="mt-3 text-meta font-bold tracking-[.16em] text-fg-faint uppercase">Good food brings people together</p>

      <div className="mt-8 rounded-sheet border border-line bg-raised p-6 shadow-card">
        <p className="text-base text-fg-muted">{collect ? "Order ahead" : "You're at"}</p>
        <h1 className="mt-1 font-serif text-total text-accent-text">{collect ? "Order to collect" : tableLabel(table.table.name)}</h1>
        <hr className="my-4 border-line" />
        <p className="font-serif text-xl">Welcome to {table.venue.name}</p>
        <p className="mt-2 text-base text-fg-2">
          {collect ? "Order and pay here. We give you a code, and call it at the counter when your order is ready." : "The whole menu, at your table."}
        </p>
        <p aria-hidden="true" className="mt-3 font-script text-xl text-attention-text">
          A small place for big moments ♡
        </p>
      </div>

      <div className="mt-auto pt-8">
        <Button variant="primary" size="charge" className="w-full" onClick={onStart}>
          Start Ordering
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
      </div>
    </main>
  );
}
