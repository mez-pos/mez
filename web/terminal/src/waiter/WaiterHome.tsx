// The waiter's home (T01): a greeting, the tables as cards under a row of
// pills that sort them by what they want, and a bar along the bottom with the
// orders still open and New Order. A card with guests at it leads to the table's
// own screen (T03); a free table, or one to clear, leads to the floor, where its
// panel seats and clears.
//
// Left out, as nothing rings them before the guest channel (phase 11): the bell,
// Table Requests and QR Orders. Reserved is not a state a table has (D06 is out
// of scope); the fifth pill is To Clear, which is one.

import { LayoutGridIcon, LeafIcon, PlusIcon, TimerIcon } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import { useAuth } from "../../../shared/auth/use-auth";
import { Button, EmptyState, InboxIcon, SearchField } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useVenue } from "../../../shared/use-venue";
import { partyTable } from "../floor/floor-logic";
import { toneClasses } from "../floor/tones";
import { failureOf } from "../register/order-messages";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";
import { TableCard } from "./TableCard";
import { detailOrder } from "./table-logic";
import { filterCounts, firstName, greeting, groupOrder, hourIn, tableGroups, tablesShown, type TableFilter } from "./waiter-logic";

export function WaiterHome() {
  const auth = useAuth();
  const { floor, ticket, names } = useTerminal();
  const venue = useVenue();
  const [, navigate] = useLocation();
  const [filter, setFilter] = useState<TableFilter>("all");
  const [query, setQuery] = useState("");
  const now = useClock(60_000);
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const data = floor.data;
  const tables = data?.tables ?? [];
  const counts = filterCounts(tables);
  const shown = data ? tablesShown(data, filter, query) : [];
  const hello = greeting(hourIn(new Date(now), venue?.timezone ?? null));
  const active = ticket.open?.orders.length ?? 0;

  const body = () => {
    if (floor.loading) return <p className="py-12 text-center text-base text-fg-muted">Loading the tables</p>;
    if (!data) return <EmptyState icon={<InboxIcon />} line={failureOf(floor.error)} action={{ label: "Try again", onClick: floor.refetch }} />;
    if (tables.length === 0) return <EmptyState icon={<InboxIcon />} line="No tables yet. Lay out the floor in the back office." />;
    if (shown.length === 0) return <EmptyState icon={<InboxIcon />} line={query ? `No table matches "${query}".` : "No tables here right now."} />;
    return (
      <ul aria-label="Tables" className="grid grid-cols-2 gap-4 lg:grid-cols-3 xl:grid-cols-4">
        {shown.map((t) => (
          // Ivory under the card's wash, so the leaf behind does not show through it.
          <li key={t.id} className="rounded-card bg-raised">
            <TableCard
              table={t}
              names={names}
              onOpen={(table) => {
                // A table joined to a party stands for the party's own table.
                const party = partyTable(table, tables);
                navigate(detailOrder(party) ? route.waiterTable(party.id) : route.table(table.id));
              }}
            />
          </li>
        ))}
      </ul>
    );
  };

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="relative min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-7 py-6">
        <LeafIcon aria-hidden="true" size={230} strokeWidth={1} className="pointer-events-none absolute -left-8 bottom-14 text-accent opacity-10" />

        <div className="relative flex items-start justify-between gap-8">
          <div className="min-w-0">
            <h1 className="font-serif text-total leading-tight">
              {hello.hello}, {firstName(auth.staff.name)}!
            </h1>
            <p className="mt-1.5 text-base text-fg-muted">Here are the tables for {hello.when}.</p>
          </div>
          <p aria-hidden="true" className="hidden pt-1 text-center font-script text-total leading-tight text-fg-2 xl:block">
            Great Food,
            <br />
            Brighter Days ♡
          </p>
          <div className="hidden w-80 shrink-0 items-center gap-3.5 rounded-card border border-chip-success-border bg-accent-tint px-5 py-4 lg:flex">
            <div className="flex-1">
              <p className="font-serif text-lg leading-tight">
                Same Tables.
                <br />
                New Stories.
              </p>
              <p className="mt-1.5 text-sm text-fg">Let's make them delicious.</p>
            </div>
            <span aria-hidden="true" className="flex size-16 shrink-0 items-center justify-center rounded bg-chip-success-bg text-accent">
              <LeafIcon size={30} strokeWidth={1.5} />
            </span>
          </div>
        </div>

        <div className="relative mt-6 flex flex-wrap items-center gap-2.5">
          <div role="group" aria-label="Show tables" className="flex flex-wrap gap-2.5">
            <FilterPill pressed={filter === "all"} count={counts.all} onPick={() => setFilter("all")}>
              <LayoutGridIcon aria-hidden="true" size={19} />
              All Tables
            </FilterPill>
            {groupOrder.map((group) => {
              const tone = toneClasses(tableGroups[group].tone);
              return (
                <FilterPill key={group} pressed={filter === group} count={counts[group]} badge={cx(tone.fill, tone.ink)} onPick={() => setFilter(group)}>
                  <span aria-hidden="true" className={cx("size-2.5 rounded-round", tone.dot)} />
                  {tableGroups[group].label}
                </FilterPill>
              );
            })}
          </div>
          <SearchField label="Search table number" onSearch={setQuery} debounceMs={100} className="ml-auto w-64" />
        </div>

        <div className="relative mt-5">{body()}</div>
      </div>

      <footer className="flex h-24 shrink-0 items-center gap-3.5 border-t border-line bg-chrome px-7">
        <Link
          href={route.orders}
          className="inline-flex min-h-charge items-center gap-2.5 rounded-pill border border-line-strong px-5 text-base font-semibold outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3"
        >
          <TimerIcon aria-hidden="true" size={20} />
          Active Orders
          <span className="flex h-6.5 min-w-6.5 items-center justify-center rounded-pill bg-accent px-2 font-mono text-sm tabular-nums text-accent-fg">
            {active}
          </span>
        </Link>
        <p aria-hidden="true" className="ml-auto hidden font-script text-xl text-fg-2 lg:block">
          Good Food, Happier People ♡
        </p>
        <Button
          variant="primary"
          size="charge"
          className="ml-auto h-16 px-8 lg:ml-0"
          onClick={() => {
            // A fresh order in front; the order screen asks for its table and how it is served.
            ticket.startNew({ channel: "counter", covers: 0 });
            navigate(route.order);
          }}
        >
          <PlusIcon aria-hidden="true" size={22} strokeWidth={2.2} />
          New Order
        </Button>
      </footer>
    </div>
  );
}

interface FilterPillProps {
  pressed: boolean;
  count: number;
  /** The count's fill and ink while the pill is not pressed; none for All Tables. */
  badge?: string;
  onPick: () => void;
  children: ReactNode;
}

function FilterPill({ pressed, count, badge, onPick, children }: FilterPillProps) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onPick}
      className={cx(
        "inline-flex h-13 items-center gap-2.5 rounded-pill border px-5 text-base outline-offset-2 outline-attention focus-visible:outline-3",
        pressed ? "border-accent bg-accent font-semibold text-accent-fg" : "border-line-strong bg-raised font-medium text-fg hover:bg-raised-high",
      )}
    >
      {children}
      <span
        className={cx(
          "flex h-6 min-w-6 items-center justify-center rounded-pill px-2 font-mono text-sm font-bold tabular-nums",
          pressed ? "bg-accent-fg/20" : (badge ?? "bg-sunken text-fg-2"),
        )}
      >
        {count}
      </span>
    </button>
  );
}
