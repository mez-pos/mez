// The shift's routes, "/shift", "/shift/close" and "/shift/z/:n": used to open
// and close the day, so fetched while the till is idle rather than parsed at
// boot (TECH-25, ADR-408).

import { Redirect, useLocation } from "wouter";
import { roleText } from "../../../shared/auth/format";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { useMenu } from "../../../shared/use-menu";
import { useVenue } from "../../../shared/use-venue";
import { ZReportScreen } from "../../../shared/zreport/ZReportScreen";
import { CloseDayScreen } from "../shift/CloseDayScreen";
import { OpenShiftScreen } from "../shift/OpenShiftScreen";
import { soldOutNames, soldOutNotice } from "../shift/open-shift-logic";
import { ShiftClosing } from "../shift/ShiftClosing";
import { ShiftNotOpen } from "../shift/ShiftNotOpen";
import { backOffice } from "./nav-logic";
import { route } from "./routes";
import { useTerminal } from "./terminal-context";

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
  const { shift, setLastZ, names } = useTerminal();
  const auth = useAuth();
  const store = useAuthStore();
  const [, navigate] = useLocation();
  const menu = useMenu();
  const venue = useVenue();
  if (!shift.shift) return null; // the gate sends a closed till to /shift
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen
  // The menu offers the close only to someone with shift.manage; the address
  // typed by hand must say the same, or a cashier reaches a count the server
  // then refuses step by step (ADR-197). Once the drawer is counted the gate
  // sends every till here, so a cashier is told the day is closing and given
  // Lock: sent back to the order screen, the gate would send them here again,
  // for ever.
  if (!auth.permissions.includes("shift.manage")) {
    if (!shift.shift.counted) return <Redirect to={route.order} replace />;
    const details = [
      ...(venue ? [{ label: "Outlet", value: venue.name }] : []),
      { label: "Terminal", value: auth.device.name },
      { label: roleText(auth.staff.role), value: auth.staff.name },
    ];
    return <ShiftClosing details={details} onLock={() => void store.signOut()} />;
  }
  return (
    <CloseDayScreen
      shift={shift.shift}
      currency={menu.data?.currency ?? "INR"}
      commands={shift.commands}
      onBack={() => navigate(route.order)}
      names={names}
      onSeeOpenOrders={() => navigate(route.openOrders)}
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
