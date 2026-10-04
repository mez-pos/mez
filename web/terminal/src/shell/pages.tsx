// One component for each route of the terminal that is an existing screen put
// behind a path (ADR-148). They read what they share from the TerminalProvider
// and move by path, where the register used to switch between them with state.
// The screens themselves are rebuilt from the design in the tasks that follow;
// these only carry them.

import { useEffect, useRef } from "react";
import { Redirect, useLocation } from "wouter";
import { useAuth } from "../../../shared/auth/use-auth";
import { useMenu } from "../../../shared/use-menu";
import { ZReportScreen } from "../../../shared/zreport/ZReportScreen";
import { BillScreen } from "../bills/BillScreen";
import { FloorView } from "../floor/FloorView";
import { ticketLabel } from "../register/ticket-time";
import { CloseDayScreen } from "../shift/CloseDayScreen";
import { TillClosedScreen } from "../shift/TillClosedScreen";
import { TenderScreen } from "../tender/TenderScreen";
import { landingFor } from "./landing";
import { route } from "./routes";
import { useTerminal } from "./terminal-context";

/** "/": a person's own first screen, by their role. */
export function Landing() {
  const auth = useAuth();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen
  return <Redirect to={landingFor(auth.staff.role, auth.permissions)} replace />;
}

/**
 * "/kitchen" for a person signed in on a terminal: the kitchen display is
 * another app of this bundle, paired and listening as a kitchen device of its
 * own, so reaching it is a page load, not a route. Until the kitchen's screen is
 * rebuilt around a signed-in cook (Task 15) it asks for its own pairing here.
 */
export function LeaveForKitchen() {
  useEffect(() => {
    window.location.assign(route.kitchen);
  }, []);
  return <p className="m-auto text-base text-fg-muted">Opening the kitchen display</p>;
}

export function TablesPage() {
  const { floor, ticket } = useTerminal();
  const [, navigate] = useLocation();
  return <FloorView floor={floor} ticket={ticket} onRegister={() => navigate(route.order)} />;
}

/** "/pay/:id": the tender screen, on the order named. */
export function PayPage({ orderId }: { orderId: string }) {
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const asked = useRef(false);

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

  if (behind) return asked.current ? <Redirect to={route.order} replace /> : <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
  if (!ticket.order) return <p className="m-auto text-base text-fg-muted">Opening the bill</p>;
  return <TenderScreen order={ticket.order} ticket={ticket} label={ticketLabel(ticket.order, names)} onBack={() => navigate(route.order)} />;
}

/** "/orders/:id": a settled bill, with its refunds and reprints. */
export function BillPage({ orderId }: { orderId: string }) {
  const { ticket } = useTerminal();
  const [, navigate] = useLocation();
  return (
    <BillScreen
      orderId={orderId}
      onBack={() => navigate(route.order)}
      onTakeBalance={(id) => {
        ticket.switchTo(id);
        navigate(route.pay(id));
      }}
    />
  );
}

/** "/shift": the till is closed until the float is counted in. */
export function ShiftPage() {
  const { shift, lastZ } = useTerminal();
  const [, navigate] = useLocation();
  const menu = useMenu();
  return (
    <TillClosedScreen
      currency={menu.data?.currency ?? "INR"}
      state={shift}
      lastZ={lastZ}
      onViewZ={() => {
        if (lastZ) navigate(route.zReport(lastZ));
      }}
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
