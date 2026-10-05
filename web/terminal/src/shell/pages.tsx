// One component for each route of the terminal that is an existing screen put
// behind a path (ADR-148). They read what they share from the TerminalProvider
// and move by path, where the register used to switch between them with state.
// The screens themselves are rebuilt from the design in the tasks that follow;
// these only carry them.

import { useEffect, useRef } from "react";
import { Redirect, useLocation } from "wouter";
import { roleText } from "../../../shared/auth/format";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { useMenu } from "../../../shared/use-menu";
import { useVenue } from "../../../shared/use-venue";
import { ZReportScreen } from "../../../shared/zreport/ZReportScreen";
import { OrdersScreen } from "../bills/OrdersScreen";
import { railTitle } from "../register/rail-logic";
import { CloseDayScreen } from "../shift/CloseDayScreen";
import { OpenShiftScreen } from "../shift/OpenShiftScreen";
import { soldOutNames, soldOutNotice } from "../shift/open-shift-logic";
import { ShiftNotOpen } from "../shift/ShiftNotOpen";
import { PayScreen } from "../tender/PayScreen";
import { WaiterBill } from "../waiter/WaiterBill";
import { landingFor } from "./landing";
import { backOffice } from "./nav-logic";
import { route } from "./routes";
import { useTerminal } from "./terminal-context";
import { useWaiterFrame } from "./use-waiter-frame";

/** "/": a person's own first screen, by their role. */
export function Landing() {
  const auth = useAuth();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen
  return <Redirect to={landingFor(auth.staff.role, auth.permissions)} replace />;
}

/** "/pay/:id": the payment screen, on the order named. A waiter's is the bill at the table (T05). */
export function PayPage({ orderId }: { orderId: string }) {
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const asked = useRef(false);
  const waiter = useWaiterFrame();

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

  // A waiter whose bill is closed is back at the tables, where the table is cleared.
  if (behind)
    return asked.current ? <Redirect to={waiter ? route.waiter : route.order} replace /> : <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
  if (!ticket.order) return <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
  if (waiter) {
    const title = railTitle(ticket.order, ticket.order.service, names);
    return <WaiterBill order={ticket.order} ticket={ticket} title={title} onClosed={() => navigate(route.waiter)} />;
  }
  return <PayScreen order={ticket.order} ticket={ticket} title={railTitle(ticket.order, ticket.order.service, names)} onBack={() => navigate(route.order)} />;
}

/** "/orders" and "/orders/:id": the day's orders, and the one picked in the panel. */
export function OrdersPage({ orderId }: { orderId: string | null }) {
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  return (
    <OrdersScreen
      selectedId={orderId}
      names={names}
      // Picking another row replaces this one in the history: Back leaves the screen.
      onSelect={(id) => navigate(route.bill(id), { replace: orderId !== null })}
      onNewOrder={() => navigate(route.order)}
      onOpenOrder={(order) => {
        // A held order is resumed, as the orders sheet does.
        if (order.status === "held") void ticket.resume(order.id);
        else ticket.switchTo(order.id);
        navigate(route.order);
      }}
      onPay={(order) => {
        ticket.switchTo(order.id);
        navigate(route.pay(order.id));
      }}
    />
  );
}

/**
 * "/shift": no shift is open, so nothing can be rung up until the float is
 * counted in. Someone who may open it sees D02; anyone else is told who can.
 */
export function ShiftPage() {
  const auth = useAuth();
  const store = useAuthStore();
  const { shift, lastZ } = useTerminal();
  const [, navigate] = useLocation();
  const menu = useMenu();
  const venue = useVenue();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const details = [
    ...(venue ? [{ label: "Outlet", value: venue.name }] : []),
    { label: "Terminal", value: auth.device.name },
    { label: roleText(auth.staff.role), value: auth.staff.name },
  ];
  const soldOut = soldOutNotice(soldOutNames(menu.data?.items ?? [], new Date()));
  const lockOut = () => void store.signOut();

  if (!auth.permissions.includes("shift.open")) {
    return <ShiftNotOpen openers={shift.openers} details={details} soldOut={soldOut} onLock={lockOut} />;
  }
  return (
    <OpenShiftScreen
      currency={menu.data?.currency ?? "INR"}
      open={shift.commands.open}
      details={details}
      soldOut={soldOut}
      menuHref={auth.permissions.includes("menu.edit") ? backOffice.menu : null}
      lastZ={lastZ}
      onViewZ={() => {
        if (lastZ) navigate(route.zReport(lastZ));
      }}
      onCancel={lockOut}
    />
  );
}

/** "/shift/close": count the drawer, explain the variance, close, and the Z-report is issued. */
export function ShiftClosePage() {
  const { shift, setLastZ } = useTerminal();
  const [, navigate] = useLocation();
  const menu = useMenu();
  if (!shift.shift) return null; // the gate sends a closed till to /shift
  return (
    <CloseDayScreen
      shift={shift.shift}
      currency={menu.data?.currency ?? "INR"}
      commands={shift.commands}
      onBack={() => navigate(route.order)}
      onClosed={(z) => {
        setLastZ(z);
        navigate(route.shift);
      }}
    />
  );
}

/** "/shift/z/:n": the Z-report this till's close issued. */
export function ZReportPage({ number }: { number: number }) {
  const [, navigate] = useLocation();
  return <ZReportScreen number={number} backLabel="Back" onBack={() => navigate(route.shift)} />;
}
