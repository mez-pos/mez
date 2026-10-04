// The 400px order rail (D03): where the order is and how many are at it, how it
// is served, its lines, its totals, and Save Order and Pay. Everything the
// design does not draw a button for is under the "..." (MoreActions).

import { ArmchairIcon, ChevronDownIcon, UsersIcon } from "lucide-react";
import type { Diet, LineResponse } from "../../../shared/api-types";
import { Button, StatusChip } from "../../../shared/ui";
import { MoreActions } from "./MoreActions";
import { RailFooter } from "./RailFooter";
import { RailLines } from "./RailLines";
import { guestsText, needsTable, railTitle, serviceLocked, serviceOf } from "./rail-logic";
import { ServiceControl } from "./ServiceControl";
import { readyLines, sendPlan as planFor } from "./send-logic";
import { chargeBlock as blockFor, chargeLabel as labelFor } from "../tender/tender-logic";
import type { Ticket } from "./use-ticket";
import type { ServiceType } from "../../../shared/api-types";

interface OrderRailProps {
  ticket: Ticket;
  currency: string;
  /** Table names by id, from the floor. */
  names: ReadonlyMap<number, string>;
  dietFor: (line: LineResponse) => Diet | undefined;
  onOpenLine: (line: LineResponse) => void;
  onVoidLine: (line: LineResponse) => void;
  onNote: () => void;
  onDiscount: () => void;
  onOrders: () => void;
  onHistory: () => void;
  onPastBills: () => void;
  onTables: () => void;
  onPay: () => void;
  /** Says why an action the screen started was refused. */
  onFail: (what: string) => (err: unknown) => void;
}

export function OrderRail(props: OrderRailProps) {
  const { ticket, currency, names, dietFor, onOpenLine, onVoidLine, onNote, onDiscount, onOrders, onHistory, onPastBills, onTables, onPay, onFail } = props;
  const { order, orderId, opening } = ticket;
  const plan = planFor(order);
  const service = serviceOf(order, opening);
  const where = order ?? opening;
  const title = railTitle(where, service, names);
  const guests = guestsText(where.covers);
  const held = order?.status === "held";
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const pickTable = needsTable(order, opening, service);

  const chooseService = (next: ServiceType) => {
    if (next !== service) ticket.startNew({ ...opening, service: next });
  };

  return (
    <aside aria-label="Order" className="flex w-100 shrink-0 flex-col border-l border-line bg-chrome">
      <header className="flex items-center gap-3 border-b border-line px-5 py-4">
        <div className="min-w-0 flex-1">
          {pickTable ? (
            <button
              type="button"
              onClick={onTables}
              className="inline-flex min-h-tap items-center gap-2 rounded-pill border border-line-strong px-4 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
            >
              <ArmchairIcon aria-hidden="true" size={18} />
              Pick a table
              <ChevronDownIcon aria-hidden="true" size={16} />
            </button>
          ) : (
            <h2 className="truncate font-serif text-xl">{title}</h2>
          )}
          {orderId && <p className="mt-0.5 truncate font-mono text-sm text-fg-muted">{orderId}</p>}
        </div>
        {guests && (
          <span className="inline-flex min-h-8 items-center gap-2 rounded-pill bg-sunken px-3 text-sm font-semibold">
            <UsersIcon aria-hidden="true" size={15} />
            {guests}
          </span>
        )}
        <MoreActions
          orderCount={Math.max(ticket.open?.orders.length ?? 0, orderId ? 1 : 0)}
          hasOrder={orderId !== null}
          canDiscount={live && !held && !order?.invoiced}
          nextCourse={!held && plan.next ? plan.next.course : null}
          onOrders={onOrders}
          onHistory={onHistory}
          onDiscount={onDiscount}
          onFire={(course) => ticket.fire(course).catch(onFail(`Couldn't fire course ${course}`))}
          onPastBills={onPastBills}
        />
      </header>

      <div className="border-b border-line px-5 py-3.5">
        <ServiceControl value={service} locked={serviceLocked(orderId)} onChange={chooseService} />
      </div>

      {held && (
        <div role="status" className="flex items-center justify-between gap-3 border-b border-line bg-raised px-5 py-3">
          <span className="flex items-center gap-2 text-sm text-fg-2">
            <StatusChip status={{ kind: "order", status: "held" }} /> Saved: nothing can be added.
          </span>
          <Button onClick={() => orderId && ticket.resume(orderId).catch(onFail("Couldn't resume"))}>Resume</Button>
        </div>
      )}

      <RailLines
        order={order}
        pending={ticket.pending}
        ready={readyLines(order)}
        dietFor={dietFor}
        onOpenLine={onOpenLine}
        onVoidLine={onVoidLine}
        onRemoveLine={(line) => ticket.removeLine(line.id).catch(onFail(`Couldn't remove ${line.name}`))}
        onQty={(line, next) => ticket.changeQty(line.id, line.qty, next).catch(onFail(`Couldn't change ${line.name}`))}
        onRetry={ticket.retry}
        onDrop={ticket.remove}
        onNote={onNote}
      />

      <RailFooter
        order={order}
        currency={currency}
        sendPlan={plan}
        chargeBlock={blockFor(order, ticket.pending.length)}
        chargeLabel={labelFor(order)}
        onSend={() => ticket.send().catch(onFail("Couldn't send"))}
        onSave={() => ticket.hold().catch(onFail("Couldn't save"))}
        onPay={onPay}
      />
    </aside>
  );
}
