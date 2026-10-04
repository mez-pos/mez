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
import { BillScreen } from "../bills/BillScreen";
import { FloorView } from "../floor/FloorView";
import { ticketLabel } from "../register/ticket-time";
import { CloseDayScreen } from "../shift/CloseDayScreen";
import { OpenShiftScreen } from "../shift/OpenShiftScreen";
import { soldOutNames, soldOutNotice } from "../shift/open-shift-logic";
import { ShiftNotOpen } from "../shift/ShiftNotOpen";
import { TenderScreen } from "../tender/TenderScreen";
import { landingFor } from "./landing";
import { backOffice } from "./nav-logic";
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

  if (!auth.permissions.includes("shift.manage")) {
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
