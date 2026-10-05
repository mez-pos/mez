// The frame around every signed-in screen of the terminal (D05, D03): a 72px
// top bar, the screen, and a 40px status bar. Each screen fills the middle and
// scrolls inside itself, so the order's total never leaves the screen.
//
// The top bar carries the logo, the navigation, and on the right who is signed
// in. A screen's own header controls (the table, the service type, search on
// the order screen) are the screen's to draw, in the space after the
// navigation, when the screens are rebuilt.
//
// A waiter's frame is T01's: an 88px bar with the tall tiles of their own
// navigation, and no status bar, on every screen they reach. Their bar says
// whether the server is there in the status bar's own words (ADR-164), beside
// the way to try it again and the "Not printed" pill.

import { DoorClosedIcon, LockIcon, PrinterIcon } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { Logo } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useServerVersion, useVenue } from "../../../shared/use-venue";
import { PrintQueueSheet } from "../print/PrintQueueSheet";
import { trouble } from "../print/print-queue";
import { usePrintJobs } from "../print/use-print-jobs";
import { ConnectionRetry } from "./ConnectionRetry";
import { ConnectionWord } from "../../../shared/shell/ConnectionWord";
import { activeNav, activeWaiterNav, navItems } from "./nav-logic";
import { NavPills } from "./NavPills";
import { ProfileMenu, type ProfileAction } from "../../../shared/shell/ProfileMenu";
import { route } from "./routes";
import { StatusBar } from "./StatusBar";
import { useTerminal } from "./terminal-context";
import { useWaiterFrame } from "./use-waiter-frame";
import { WaiterOrderBar } from "../waiter/WaiterOrderBar";
import { WaiterBack } from "./WaiterBack";
import { WaiterNav } from "./WaiterNav";

export function TerminalShell({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const store = useAuthStore();
  const [path, navigate] = useLocation();
  const { shift } = useTerminal();
  const venue = useVenue();
  const version = useServerVersion();
  const queue = usePrintJobs();
  const [printOpen, setPrintOpen] = useState(false);
  const waiter = useWaiterFrame();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const failing = trouble(queue.jobs);
  // Closing the shift needs shift.manage and a shift to close.
  const canCloseShift = auth.permissions.includes("shift.manage") && Boolean(shift.shift);
  const account: ProfileAction[] = [
    { icon: LockIcon, label: "Lock", onClick: () => void store.signOut() },
    ...(canCloseShift ? [{ icon: DoorClosedIcon, label: "Close the shift", onClick: () => navigate(route.shiftClose) }] : []),
    { icon: PrinterIcon, label: "Print queue", onClick: () => setPrintOpen(true) },
  ];

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <header className={cx("flex shrink-0 items-center gap-6 border-b border-line bg-chrome px-7", waiter ? "h-22" : "h-18")}>
        <Logo className="h-9" />
        {waiter && path === route.order ? (
          // Taking an order, the bar is the order's: where, how many, how served (T02).
          <WaiterOrderBar />
        ) : waiter && path.startsWith(`${route.waiter}/table/`) ? (
          <WaiterBack />
        ) : waiter && path.startsWith("/pay/") ? null : waiter ? ( // The bill carries its own way back (T05).
          <WaiterNav active={activeWaiterNav(path)} onPrintQueue={() => setPrintOpen(true)} />
        ) : (
          <NavPills items={navItems(auth.permissions)} active={activeNav(path)} />
        )}
        <div className="ml-auto flex items-center gap-5 whitespace-nowrap">
          {waiter && <ConnectionWord brief className="text-sm" />}
          <ConnectionRetry />
          {failing > 0 && (
            <button
              type="button"
              onClick={() => setPrintOpen(true)}
              aria-label={`Print queue: ${failing} not printed`}
              className="inline-flex min-h-tap items-center gap-2 rounded-pill bg-danger-tint px-4 text-base font-semibold text-danger outline-offset-2 outline-line-focus focus-visible:outline-2"
            >
              <PrinterIcon aria-hidden="true" size={17} />
              Not printed <span className="font-mono tabular-nums">{failing}</span>
            </button>
          )}
          <ProfileMenu name={auth.staff.name} role={auth.staff.role} actions={account} />
          <p className="hidden text-right font-script text-xl leading-tight text-fg-2 2xl:block" aria-hidden="true">
            A better table for
            <br />
            brighter days ♡
          </p>
        </div>
      </header>
      <main className="flex min-h-0 flex-1">{children}</main>
      {!waiter && <StatusBar version={version} venueName={venue?.name ?? null} timezone={venue?.timezone ?? null} />}
      {printOpen && <PrintQueueSheet queue={queue} onClose={() => setPrintOpen(false)} />}
    </div>
  );
}
