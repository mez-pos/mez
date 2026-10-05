// The waiter's top bar while an order is taken (T02): the way back to the
// tables, the table the order is at, how many guests, and how it is served. The
// table leads to the floor, where one is picked or the party moved. The design
// has no way back in this bar; a waiter needs one, so an arrow stands first.

import { ArmchairIcon, ArrowLeftIcon, ChevronDownIcon, UsersIcon } from "lucide-react";
import { Link } from "wouter";
import { guestsText, needsTable, railTitle, serviceLocked, serviceOf } from "../register/rail-logic";
import { ServiceControl } from "../register/ServiceControl";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";

export function WaiterOrderBar() {
  const { ticket, names } = useTerminal();
  const { order, orderId, opening } = ticket;
  const service = serviceOf(order, opening);
  const where = order ?? opening;
  const guests = guestsText(where.covers);

  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <Link
        href={route.waiter}
        aria-label="Back to the tables"
        className="flex size-tap shrink-0 items-center justify-center rounded-round bg-sunken outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
      >
        <ArrowLeftIcon aria-hidden="true" size={20} />
      </Link>
      {service === "dine_in" && (
        <Link
          href={route.tables}
          className="inline-flex min-h-charge shrink-0 items-center gap-2.5 whitespace-nowrap rounded-pill bg-attention-tint px-4 text-base font-bold text-attention-text outline-offset-2 outline-attention focus-visible:outline-3"
        >
          <ArmchairIcon aria-hidden="true" size={20} />
          {needsTable(order, opening, service) ? "Pick a table" : railTitle(where, service, names)}
          <ChevronDownIcon aria-hidden="true" size={17} />
        </Link>
      )}
      {guests && (
        <span className="inline-flex min-h-charge shrink-0 items-center gap-2.5 whitespace-nowrap rounded-pill border border-line-strong px-4 text-base font-semibold">
          <UsersIcon aria-hidden="true" size={20} />
          {guests}
        </span>
      )}
      <ServiceControl
        roomy
        value={service}
        locked={serviceLocked(orderId)}
        onChange={(next) => {
          if (next !== service) ticket.startNew({ ...opening, service: next });
        }}
      />
    </div>
  );
}
