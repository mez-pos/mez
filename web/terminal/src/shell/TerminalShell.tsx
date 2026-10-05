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

import { PrinterIcon } from "lucide-react";
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
import { ConnectionWord } from "./ConnectionWord";
import { landingFor } from "./landing";
import { activeNav, activeWaiterNav, navItems } from "./nav-logic";
import { NavPills } from "./NavPills";
import { ProfileMenu } from "./ProfileMenu";
import { route } from "./routes";
import { StatusBar } from "./StatusBar";
import { useTerminal } from "./terminal-context";
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
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const failing = trouble(queue.jobs);
  const canCloseShift = auth.permissions.includes("shift.manage") && Boolean(shift.shift);
  // Whoever lands on the waiter's home works from the waiter's frame.
  const waiter = landingFor(auth.staff.role, auth.permissions) === route.waiter;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <header className={cx("flex shrink-0 items-center gap-6 border-b border-line bg-chrome px-7", waiter ? "h-22" : "h-18")}>
        <Logo className="h-9" />
        {waiter ? (
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
          <ProfileMenu
            name={auth.staff.name}
            role={auth.staff.role}
            canCloseShift={canCloseShift}
            onLock={() => void store.signOut()}
            onCloseShift={() => navigate(route.shiftClose)}
            onPrintQueue={() => setPrintOpen(true)}
          />
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
