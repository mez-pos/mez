// "/waiter/shift": the shift summary (T08). Who is signed in and how long they
// have been at it, four figures for the open shift, their tips by table, and
// End shift. Every figure is the server's for the person asking (ADR-173).
//
// Left out: guest ratings (nothing collects them before phase 11), the area
// beside the role (no one is assigned an area), and Print summary (nothing
// prints one). End shift signs the person out: closing the shift is a
// manager's, with the drawer count, and is not a waiter's to do.

import { useQuery } from "@tanstack/react-query";
import { LogOutIcon } from "lucide-react";
import type { ReactNode } from "react";
import type { MyShiftResponse } from "../../../shared/api-types";
import { initials, roleText } from "../../../shared/auth/format";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useVenue } from "../../../shared/use-venue";
import { failureOf } from "../register/order-messages";
import { useTerminal } from "../shell/terminal-context";
import { onShiftText, shiftSpan, tipBars } from "./shift-summary-logic";

export function ShiftSummary() {
  const auth = useAuth();
  const store = useAuthStore();
  const venue = useVenue();
  const { names } = useTerminal();
  const now = useClock(30_000);
  // Asked afresh each time the screen is opened: it is read at the end of a shift, not watched.
  const mine = useQuery({
    queryKey: ["shift", "mine"],
    queryFn: ({ signal }) => store.api.get<MyShiftResponse>("/api/v1/shift/mine", { signal }),
    staleTime: 0,
    gcTime: 0,
  });
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const data = mine.data;
  const bars = data ? tipBars(data.tips_by_table, names) : [];

  return (
    <div className="flex min-w-0 flex-1 justify-center overflow-hidden px-5 py-4">
      <section aria-label="Your shift" className="flex w-165 max-w-full flex-col overflow-hidden rounded-sheet border border-line bg-canvas shadow-modal">
        <header className="flex min-h-18 shrink-0 flex-wrap items-center gap-x-3.5 border-b border-line bg-chrome px-5 py-2">
          <h1 className="font-serif text-xl">Your shift</h1>
          {data && <p className="ml-auto font-mono text-sm tabular-nums text-fg-muted">{shiftSpan(data.opened_at, now, venue?.timezone ?? null)}</p>}
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          <div className="flex items-center gap-3.5 rounded-card border border-line bg-raised p-4">
            <span aria-hidden="true" className="flex size-15 shrink-0 items-center justify-center rounded-round bg-sunken text-lg font-bold text-fg-2">
              {initials(auth.staff.name)}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate font-serif text-xl">{auth.staff.name}</p>
              <p className="text-sm text-fg-muted">{roleText(auth.staff.role)}</p>
            </div>
            {data && (
              <span className="inline-flex min-h-10 shrink-0 items-center gap-2 rounded-pill bg-chip-success-bg px-4 text-sm font-bold text-chip-success-fg">
                <span aria-hidden="true" className="size-2 rounded-round bg-current" />
                {onShiftText(data, now)}
              </span>
            )}
          </div>

          {!data && (
            <div className="mt-6 flex flex-col items-center gap-4 text-center">
              <p role={mine.isError ? "alert" : undefined} className={mine.isError ? "text-base text-danger" : "text-base text-fg-muted"}>
                {mine.isError ? failureOf(mine.error) : "Adding up your shift"}
              </p>
              {mine.isError && <Button onClick={() => void mine.refetch()}>Try again</Button>}
            </div>
          )}

          {data && (
            <>
              <dl className="mt-3.5 grid grid-cols-2 gap-3">
                <Figure label="Tables served" tone="green">
                  {data.tables}
                </Figure>
                <Figure label="Orders taken">{data.orders}</Figure>
                <Figure label="Sales handled">
                  <Money amount={data.sales} variant="total" />
                </Figure>
                <Figure label="Tips collected" tone="mustard">
                  <Money amount={data.tips} variant="total" className="text-chip-warning-fg" />
                </Figure>
              </dl>
              <p className="mt-2 text-sm text-fg-muted">
                Orders and tables are the ones you opened; sales and tips are on the payments you took. Tips are shared out after the shift closes.
              </p>

              <section aria-label="Tips by table" className="mt-3.5 rounded-card border border-line bg-raised p-4">
                <h2 className="font-serif text-xl">Tips by table</h2>
                {bars.length === 0 ? (
                  <p className="mt-2 text-base text-fg-muted">No tips yet this shift.</p>
                ) : (
                  <ul className="mt-3 grid gap-2.5 text-base">
                    {bars.map((bar) => (
                      <li key={bar.tableId} className="flex items-center gap-3">
                        <span className="w-20 shrink-0 truncate font-bold">{bar.label}</span>
                        <span aria-hidden="true" className="h-2.5 min-w-0 flex-1 rounded-pill bg-sunken">
                          <span className="block h-2.5 rounded-pill bg-highlight" style={{ width: `${bar.percent}%` }} />
                        </span>
                        <Money amount={bar.tips} className="w-24 shrink-0 font-bold" />
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </>
          )}
        </div>

        <footer className="flex shrink-0 flex-wrap items-center gap-3 border-t border-line bg-chrome px-5 py-4">
          <p className="min-w-0 flex-1 text-sm text-fg-muted">End shift signs you out. A manager counts the drawer and closes the shift.</p>
          <Button variant="attention" size="charge" onClick={() => void store.signOut()} className="w-60">
            <LogOutIcon aria-hidden="true" size={18} />
            End shift
          </Button>
        </footer>
      </section>
    </div>
  );
}

const tones = {
  plain: "border-line bg-raised",
  green: "border-chip-success-border bg-accent-tint",
  mustard: "border-chip-warning-border bg-chip-warning-bg",
} as const;

function Figure({ label, tone = "plain", children }: { label: string; tone?: keyof typeof tones; children: ReactNode }) {
  return (
    <div className={cx("rounded-card border p-4", tones[tone])}>
      <dt className="text-sm text-fg-2">{label}</dt>
      <dd className="font-serif text-total leading-tight tabular-nums">{children}</dd>
    </div>
  );
}
