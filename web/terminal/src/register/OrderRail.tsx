// The 400px order rail (D03): where the order is and how many are at it, how it
// is served, its lines, its totals, and Save Order and Pay. Everything the
// design does not draw a button for is under the "..." (MoreActions).
//
// A waiter's rail (T02): how it is served is chosen in the top bar and shown
// here as a chip, each line says its course, Clear takes off what the kitchen
// has not seen, and the foot is Save and Send to Kitchen.

import { ArmchairIcon, ChevronDownIcon, UsersIcon } from "lucide-react";
import { useState } from "react";
import type { Diet, LineResponse } from "../../../shared/api-types";
import { Button, Confirm, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { clearableLines } from "./cart-logic";
import { ConfirmSend } from "./ConfirmSend";
import { CourseView } from "./CourseView";
import { MoreActions } from "./MoreActions";
import { RailFooter } from "./RailFooter";
import { RailLines } from "./RailLines";
import { guestsText, needsTable, railTitle, saveParks, serviceLabels, serviceLocked, serviceOf } from "./rail-logic";
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
  /** The waiter's rail (T02). */
  waiter?: boolean;
  /** A course's name by its number; the waiter's rail shows it. */
  courseName?: (course: number | undefined) => string | null;
  /** After Save has parked the order. */
  onSaved?: () => void;
}

export function OrderRail(props: OrderRailProps) {
  const { ticket, currency, names, dietFor, onOpenLine, onVoidLine, onNote, onDiscount, onOrders, onHistory, onPastBills, onTables, onPay, onFail } = props;
  const { waiter = false, courseName, onSaved } = props;
  const [clearing, setClearing] = useState(false);
  const [confirmingSend, setConfirmingSend] = useState(false);
  const [tab, setTab] = useState<"order" | "courses">("order");
  const { order, orderId, opening } = ticket;
  const plan = planFor(order);
  const service = serviceOf(order, opening);
  const where = order ?? opening;
  const title = railTitle(where, service, names);
  const guests = guestsText(where.covers);
  const held = order?.status === "held";
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  // The waiter's top bar is where a table is picked; the rail only says where.
  const pickTable = !waiter && needsTable(order, opening, service);
  const chargeBlock = blockFor(order, ticket.pending.length);
  const chargeLabel = labelFor(order);
  const clearable = clearableLines(order);

  /** Clear: each line the kitchen has not seen, taken off in turn as its own bin would. */
  const clear = async () => {
    setClearing(false);
    try {
      for (const line of clearable) await ticket.removeLine(line.id);
    } catch (err) {
      onFail("Couldn't clear the order")(err);
    }
  };

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
        {waiter && (
          <span className="inline-flex min-h-8 items-center rounded-pill bg-chip-success-bg px-3 text-sm font-bold text-chip-success-fg">
            {serviceLabels[service]}
          </span>
        )}
        <MoreActions
          orderCount={Math.max(ticket.open?.orders.length ?? 0, orderId ? 1 : 0)}
          hasOrder={orderId !== null}
          canDiscount={live && !held && !order?.invoiced}
          // A waiter fires from Course View, where the course is named.
          nextCourse={!waiter && !held && plan.next ? plan.next.course : null}
          onOrders={onOrders}
          onHistory={onHistory}
          onDiscount={onDiscount}
          onFire={(course) => ticket.fire(course).catch(onFail(`Couldn't fire course ${course}`))}
          onPastBills={onPastBills}
          {...(waiter ? { pay: { label: chargeLabel === "Pay" ? "Bill and pay" : chargeLabel, blocked: chargeBlock !== null, onPay } } : {})}
        />
      </header>

      {!waiter && (
        <div className="border-b border-line px-5 py-3.5">
          <ServiceControl value={service} locked={serviceLocked(orderId)} onChange={chooseService} />
        </div>
      )}

      {held && (
        <div role="status" className="flex items-center justify-between gap-3 border-b border-line bg-raised px-5 py-3">
          <span className="flex items-center gap-2 text-sm text-fg-2">
            <StatusChip status={{ kind: "order", status: "held" }} /> Saved: nothing can be added.
          </span>
          <Button onClick={() => orderId && ticket.resume(orderId).catch(onFail("Couldn't resume"))}>Resume</Button>
        </div>
      )}

      {waiter && (
        <div role="tablist" aria-label="Order views" className="flex gap-6 border-b border-line px-5">
          {(
            [
              ["order", `Current Order (${order?.lines.filter((l) => l.status !== "voided").length ?? 0})`],
              ["courses", "Course View"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              onClick={() => setTab(id)}
              className={cx(
                "min-h-tap border-b-3 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                tab === id ? "border-attention font-bold text-attention-text" : "border-transparent text-fg-muted hover:text-fg",
              )}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      {waiter && tab === "courses" ? (
        <CourseView
          order={order}
          courseName={courseName ?? (() => null)}
          ready={readyLines(order)}
          canFire={!held}
          onFire={(course) => ticket.fire(course).catch(onFail("Couldn't fire the course"))}
        />
      ) : (
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
          {...(waiter && courseName ? { courseName: (line: LineResponse) => courseName(line.course) } : {})}
          {...(waiter && !held ? { clear: { count: clearable.length, onClear: () => setClearing(true) } } : {})}
        />
      )}

      <RailFooter
        order={order}
        currency={currency}
        sendPlan={plan}
        chargeBlock={chargeBlock}
        chargeLabel={chargeLabel}
        // A waiter reads what will go before it goes (T04).
        onSend={waiter ? () => setConfirmingSend(true) : () => ticket.send().catch(onFail("Couldn't send"))}
        onSave={() => {
          if (!saveParks(waiter, where)) {
            onSaved?.();
            return;
          }
          ticket
            .hold()
            .then(() => onSaved?.())
            .catch(onFail("Couldn't save"));
        }}
        onPay={onPay}
        waiter={waiter}
      />

      {confirmingSend && order && (
        <ConfirmSend
          order={order}
          title={title}
          service={serviceLabels[service]}
          courseName={(line) => courseName?.(line.course) ?? null}
          onSend={ticket.send}
          onClose={() => setConfirmingSend(false)}
        />
      )}

      <Confirm open={clearing} onClose={() => setClearing(false)} onConfirm={() => void clear()} title="Clear the order?" confirmLabel="Clear" tone="danger">
        {clearable.length === 1 ? "The 1 item" : `The ${clearable.length} items`} the kitchen has not seen will be taken off. Anything already sent stays.
      </Confirm>
    </aside>
  );
}
