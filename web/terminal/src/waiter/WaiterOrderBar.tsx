// The waiter's top bar while an order is taken (T02): the way back to the
// tables, the table the order is at, how many guests, and how it is served. The
// table leads to the floor, where one is picked or the party moved. The design
// has no way back in this bar; a waiter needs one, so an arrow stands first: to
// the order's table, or to the tables for an order with none.
//
// On a 1024 tablet the bar shares the header with the connection, Not printed
// and the profile, and must not run under them (TBS-3): once the order is open
// its service cannot change, so it is a word, not three buttons; the choice
// before that is narrow below xl; and the table's name gives way first.

import { ArmchairIcon, ArrowLeftIcon, BikeIcon, ChevronDownIcon, ShoppingBagIcon, UsersIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import type { ServiceType } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { guestsText, needsTable, railTitle, serviceLabels, serviceLocked, serviceOf } from "../register/rail-logic";
import { ServiceControl } from "../register/ServiceControl";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";
import { backFromOrder } from "./table-logic";

const serviceIcons: Readonly<Record<ServiceType, LucideIcon>> = {
  dine_in: UtensilsIcon,
  takeaway: ShoppingBagIcon,
  delivery: BikeIcon,
};

const pill = "inline-flex min-h-charge items-center gap-2.5 whitespace-nowrap rounded-pill px-4 text-base";

export function WaiterOrderBar() {
  const { ticket, names } = useTerminal();
  const { order, orderId, opening } = ticket;
  const service = serviceOf(order, opening);
  const where = order ?? opening;
  const guests = guestsText(where.covers);
  const back = backFromOrder(where);
  const ServiceIcon = serviceIcons[service];
  const choose = (next: ServiceType) => {
    if (next !== service) ticket.startNew({ ...opening, service: next });
  };

  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <Link
        href={back.href}
        aria-label={back.label}
        className="flex size-tap shrink-0 items-center justify-center rounded-round bg-sunken outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
      >
        <ArrowLeftIcon aria-hidden="true" size={20} />
      </Link>
      {service === "dine_in" && (
        <Link
          href={route.tables}
          className={cx(pill, "min-w-0 bg-attention-tint font-bold text-attention-text outline-offset-2 outline-attention focus-visible:outline-3")}
        >
          <ArmchairIcon aria-hidden="true" size={20} className="shrink-0" />
          <span className="min-w-0 truncate">{needsTable(order, opening, service) ? "Pick a table" : railTitle(where, service, names)}</span>
          <ChevronDownIcon aria-hidden="true" size={17} className="shrink-0" />
        </Link>
      )}
      {guests && (
        <span className={cx(pill, "shrink-0 border border-line-strong font-semibold")}>
          <UsersIcon aria-hidden="true" size={20} />
          {guests}
        </span>
      )}
      {serviceLocked(orderId) ? (
        <span className={cx(pill, "shrink-0 border border-line-strong font-semibold")}>
          <ServiceIcon aria-hidden="true" size={20} />
          {serviceLabels[service]}
        </span>
      ) : (
        <>
          <div className="shrink-0 max-xl:hidden">
            <ServiceControl roomy value={service} locked={false} onChange={choose} />
          </div>
          <div className="shrink-0 xl:hidden">
            <ServiceControl value={service} locked={false} onChange={choose} />
          </div>
        </>
      )}
    </div>
  );
}
