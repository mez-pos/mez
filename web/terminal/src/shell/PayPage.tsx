// "/pay/:id", apart from the routes the till boots with: the pay screen is
// fetched while the till is idle (TECH-25, ADR-408), as are the waiter's bill
// and the invoice they draw.

import { useEffect, useRef, useState } from "react";
import { Redirect, useLocation } from "wouter";
import { Button, EmptyState, InboxIcon } from "../../../shared/ui";
import { failureOf } from "../register/order-messages";
import { railTitle } from "../register/rail-logic";
import { PayScreen } from "../tender/PayScreen";
import { WaiterBill } from "../waiter/WaiterBill";
import { route } from "./routes";
import { useTerminal } from "./terminal-context";
import { useWaiterFrame } from "./use-waiter-frame";

/** "/pay/:id": the payment screen, on the order named. A waiter's is the bill at the table (T05). */
export function PayPage({ orderId }: { orderId: string }) {
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const asked = useRef(false);
  const waiter = useWaiterFrame();

  // The order paid here stays in front while this screen shows its paid
  // panel (ADR-383). Leaving by any other way than Next order lets it go.
  const { release } = ticket;
  useEffect(() => release, [release]);

  // Arriving by a link, the order named may not be the one in front: bring it
  // there, once. If the ticket moves on after that (the bill was paid and
  // closed, so a new ticket starts), the order screen is where to be.
  const behind = ticket.orderId !== orderId;
  useEffect(() => {
    if (behind && !asked.current) {
      asked.current = true;
      ticket.switchTo(orderId);
    }
  }, [behind, orderId, ticket]);
  // Whether the order named has been in front: read from the ticket as it
  // renders, so no ref is read here. Once it has, being behind again means
  // the ticket moved on.
  const [wasInFront, setWasInFront] = useState(!behind);
  if (!behind && !wasInFront) setWasInFront(true);

  // A waiter whose bill is closed is back at the tables, where the table is cleared.
  if (behind)
    return wasInFront ? <Redirect to={waiter ? route.waiter : route.order} replace /> : <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
  if (!ticket.order) {
    if (ticket.loading || ticket.error === null) return <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
    // The read failed: say so, with the way to try again and the way out.
    // Without them the only way off this screen was Lock.
    return (
      <div className="m-auto flex flex-col items-center gap-4">
        <EmptyState icon={<InboxIcon />} line={failureOf(ticket.error)} action={{ label: "Try again", onClick: ticket.refetch }} />
        <Button onClick={() => navigate(waiter ? route.waiter : route.order)}>{waiter ? "Back to the tables" : "Back to the order"}</Button>
      </div>
    );
  }
  if (waiter) {
    const title = railTitle(ticket.order, ticket.order.service, names);
    return <WaiterBill order={ticket.order} ticket={ticket} title={title} onClosed={() => navigate(route.waiter)} />;
  }
  return <PayScreen order={ticket.order} ticket={ticket} title={railTitle(ticket.order, ticket.order.service, names)} onBack={() => navigate(route.order)} />;
}
