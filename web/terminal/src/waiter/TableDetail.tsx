// "/waiter/table/:id": one table's own screen (T03). It shows the table's one
// order. A table with none to show (free, to clear, or with several orders) is
// the floor's: its panel seats, clears and asks which order.

import { Redirect, useLocation } from "wouter";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { panelTitle, partyTable } from "../floor/floor-logic";
import { failureOf } from "../register/order-messages";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";
import { detailOrder } from "./table-logic";
import { TableDetailOrder } from "./TableDetailOrder";

export function TableDetail({ tableId }: { tableId: number }) {
  const { floor, ticket } = useTerminal();
  const [, navigate] = useLocation();

  if (floor.loading) return <p className="m-auto text-base text-fg-muted">Opening the table</p>;
  if (!floor.data) {
    return <EmptyState icon={<InboxIcon />} line={failureOf(floor.error)} action={{ label: "Try again", onClick: floor.refetch }} className="m-auto" />;
  }
  const tables = floor.data.tables;
  const found = tables.find((t) => t.id === tableId);
  if (!found) {
    return (
      <EmptyState
        icon={<InboxIcon />}
        line="This table is not on the floor any more."
        action={{ label: "Back to the tables", onClick: () => navigate(route.waiter) }}
        className="m-auto"
      />
    );
  }
  const table = partyTable(found, tables);
  const orderId = detailOrder(table);
  if (!orderId) return <Redirect to={route.table(table.id)} replace />;

  return (
    <TableDetailOrder
      key={orderId}
      table={table}
      title={panelTitle(table, tables)}
      orderId={orderId}
      // Items rung up on this tablet and still on their way would be overtaken by a Send from here.
      sendWaits={ticket.orderId === orderId && ticket.pending.length > 0}
      onAddItems={async (held) => {
        if (held) await ticket.resume(orderId);
        else ticket.switchTo(orderId);
        navigate(route.order);
      }}
      onBill={() => navigate(route.pay(orderId))}
    />
  );
}
