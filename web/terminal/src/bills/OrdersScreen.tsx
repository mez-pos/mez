// The Orders screen (D09): the trading day's orders in one table, open and
// settled together, with the tabs that narrow it by status, a search, and the
// order picked down the right (OrderPanel.tsx). The day's list and what it
// collected are the server's (GET /orders/today); this screen counts the tabs
// and filters the rows. A search that reads like a number also asks for
// settled bills from before today, which the day's list does not hold.
//
// The design's Filters and Export are left out: the tabs are the filters there
// are, and the till exports nothing (the back office does). In Export's place
// is New Order, the way back to the order screen.

import { PlusIcon } from "lucide-react";
import { useState } from "react";
import type { DayStatus, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money, SearchField, SearchIcon, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { failureOf } from "../register/order-messages";
import { serviceLabels } from "../register/rail-logic";
import { OrderPanel } from "./OrderPanel";
import { countText, filterOrders, itemsText, orderNumber, ordersTabs, pastStatus, tabCounts, tableText, timeText, type OrdersTab } from "./orders-logic";
import { useDayOrders, usePastBills } from "./use-bills";

interface OrdersScreenProps {
  /** The order in the panel, from the path. */
  selectedId: string | null;
  names: ReadonlyMap<number, string>;
  onSelect: (orderId: string) => void;
  onNewOrder: () => void;
  onOpenOrder: (order: OrderResponse) => void;
  onPay: (order: OrderResponse) => void;
}

const columns = "grid grid-cols-[9.5rem_4.5rem_4rem_5.5rem_minmax(0,1fr)_6.5rem_7.5rem_4.5rem] items-center gap-3 2xl:gap-4";

export function OrdersScreen({ selectedId, names, onSelect, onNewOrder, onOpenOrder, onPay }: OrdersScreenProps) {
  const day = useDayOrders();
  const [tab, setTab] = useState<OrdersTab>("all");
  const [query, setQuery] = useState("");
  // Bills from before today are found by their number alone.
  const past = usePastBills(query, query !== "");
  const now = new Date();

  const orders = day.data?.orders ?? [];
  const counts = tabCounts(orders);
  const rows = filterOrders(orders, tab, query, names);
  const today = new Set(orders.map((o) => o.id));
  const earlier = query === "" ? [] : (past.data?.bills ?? []).filter((b) => !today.has(b.id) && (tab === "all" || tab === pastStatus(b)));

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Orders" className="flex min-w-0 flex-1 flex-col px-6 pb-5 pt-5 2xl:px-7 2xl:pt-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="font-serif text-total leading-tight">Orders</h1>
            <p className="mt-1 text-base text-fg-muted" aria-live="polite">
              {day.data ? (
                <>
                  {countText(orders.length)} · <Money amount={day.data.collected} /> collected
                </>
              ) : day.isError ? (
                ""
              ) : (
                "Loading today's orders"
              )}
            </p>
          </div>
          <Button variant="attention" onClick={onNewOrder}>
            <PlusIcon aria-hidden="true" size={18} />
            New Order
          </Button>
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-2.5">
          <div role="tablist" aria-label="Orders by status" className="flex flex-wrap gap-2.5">
            {ordersTabs.map((t) => (
              <button
                key={t.id}
                type="button"
                role="tab"
                aria-selected={tab === t.id}
                onClick={() => setTab(t.id)}
                className={cx(pillClass(tab === t.id), "min-h-tap")}
              >
                {t.label} <span className="font-mono">({counts[t.id]})</span>
              </button>
            ))}
          </div>
          <SearchField label="Search order # or table" onSearch={setQuery} className="ml-auto w-72 2xl:w-80" />
        </div>

        <div className="mt-4 flex min-h-0 flex-1 flex-col overflow-hidden rounded-card border border-line bg-raised">
          <div className="overflow-x-auto overflow-y-auto">
            <div className="min-w-184 px-2">
              <div className={cx(columns, "border-b border-line px-3 py-3.5 text-meta font-bold uppercase text-fg-muted")}>
                <span>Order #</span>
                <span>Time</span>
                <span>Table</span>
                <span>Type</span>
                <span>Items</span>
                <span className="text-right">Amount</span>
                <span>Status</span>
                <span />
              </div>

              {day.isError && (
                <p role="alert" className="px-3 py-6 text-base text-danger">
                  {failureOf(day.error)}
                </p>
              )}

              <ol aria-label="Today's orders">
                {rows.map((o) => (
                  <OrderRow
                    key={o.id}
                    selected={o.id === selectedId}
                    number={orderNumber(o)}
                    time={timeText(o.opened_at, now)}
                    table={tableText(o.table_id, names)}
                    type={serviceLabels[o.service]}
                    items={itemsText(o.items)}
                    amount={o.total}
                    status={o.status}
                    onPick={() => onSelect(o.id)}
                  />
                ))}
              </ol>

              {earlier.length > 0 && (
                <>
                  <h2 className="border-b border-line px-3 pb-2 pt-5 text-meta font-bold uppercase text-fg-muted">Before today</h2>
                  <ol aria-label="Bills before today">
                    {earlier.map((b) => (
                      <OrderRow
                        key={b.id}
                        selected={b.id === selectedId}
                        number={b.invoice_no || b.id}
                        time={timeText(b.closed_at, now)}
                        table={tableText(b.table_id, names)}
                        type=""
                        items=""
                        amount={b.total}
                        status={pastStatus(b)}
                        onPick={() => onSelect(b.id)}
                      />
                    ))}
                  </ol>
                </>
              )}

              {day.data && rows.length === 0 && earlier.length === 0 && (
                <EmptyState
                  icon={query ? <SearchIcon /> : <InboxIcon />}
                  line={query ? `No order matches "${query}".` : tab === "all" ? "No orders yet today." : "No orders here yet today."}
                />
              )}
            </div>
          </div>
        </div>
      </section>

      {selectedId ? (
        <OrderPanel key={selectedId} orderId={selectedId} names={names} onOpenOrder={onOpenOrder} onPay={onPay} />
      ) : (
        <aside aria-label="Order" className="flex w-96 shrink-0 items-center justify-center border-l border-line bg-chrome p-6 2xl:w-110">
          <EmptyState icon={<InboxIcon />} line="Pick an order to see it." />
        </aside>
      )}
    </div>
  );
}

interface OrderRowProps {
  selected: boolean;
  number: string;
  time: string;
  table: string;
  type: string;
  items: string;
  amount: Amount;
  status: DayStatus;
  onPick: () => void;
}

/** A row is one button: the whole of it opens the order in the panel. */
function OrderRow({ selected, number, time, table, type, items, amount, status, onPick }: OrderRowProps) {
  return (
    <li className="border-b border-line-soft">
      <button
        type="button"
        aria-current={selected ? "true" : undefined}
        onClick={onPick}
        className={cx(
          columns,
          "min-h-14 w-full rounded px-3 text-left text-base -outline-offset-2 outline-attention focus-visible:outline-3",
          selected ? "bg-accent-tint" : "hover:bg-raised-high",
        )}
      >
        <span className="truncate font-mono font-bold">{number}</span>
        <span className="font-mono text-fg-2">{time}</span>
        <span className="truncate font-semibold">{table}</span>
        <span className="truncate text-fg-2">{type}</span>
        <span className="truncate">{items}</span>
        <Money amount={amount} className="font-bold" />
        <StatusChip status={{ kind: "day", status }} className="justify-self-start" />
        <span
          aria-hidden="true"
          className="flex h-9 items-center justify-center rounded-pill border border-line-strong bg-raised text-base font-semibold text-fg"
        >
          View
        </span>
      </button>
    </li>
  );
}
