// One component for each route of the terminal that is an existing screen put
// behind a path (ADR-148). They read what they share from the TerminalProvider
// and move by path, where the register used to switch between them with state.
// The screens themselves are rebuilt from the design in the tasks that follow;
// these only carry them. The pay and shift routes are in PayPage.tsx and
// ShiftPages.tsx, fetched apart from these (TECH-25, ADR-408).

import { Redirect, useLocation, useSearch } from "wouter";
import { useAuth } from "../../../shared/auth/use-auth";
import { useToast } from "../../../shared/ui";
import { OrdersScreen } from "../bills/OrdersScreen";
import { tabFrom } from "../bills/orders-logic";
import { wordsOf } from "../register/order-messages";
import { landingFor } from "./landing";
import { route } from "./routes";
import { useTerminal } from "./terminal-context";

/** "/": a person's own first screen, by their role. */
export function Landing() {
  const auth = useAuth();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen
  return <Redirect to={landingFor(auth.staff.role, auth.permissions)} replace />;
}

/** "/orders" and "/orders/:id": the day's orders, and the one picked in the panel. */
export function OrdersPage({ orderId }: { orderId: string | null }) {
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const search = useSearch();
  const toast = useToast();
  return (
    <OrdersScreen
      selectedId={orderId}
      names={names}
      initialTab={tabFrom(search)}
      // Picking another row replaces this one in the history: Back leaves the screen.
      onSelect={(id) => navigate(route.bill(id), { replace: orderId !== null })}
      onNewOrder={() => navigate(route.order)}
      onOpenOrder={(order) => {
        // A held order is resumed, as the orders sheet does; one the server
        // will not give back stays here with the reason, rather than the
        // register opening on an order that was never resumed.
        if (order.status === "held") {
          ticket.resume(order.id).then(
            () => navigate(route.order),
            (err: unknown) => toast(wordsOf(err), { tone: "warning" }),
          );
          return;
        }
        ticket.switchTo(order.id);
        navigate(route.order);
      }}
      onPay={(order) => {
        ticket.switchTo(order.id);
        navigate(route.pay(order.id));
      }}
    />
  );
}
