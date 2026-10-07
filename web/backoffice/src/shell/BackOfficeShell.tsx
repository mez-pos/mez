// The frame around every back office screen (D12): a 72px bar with the logo,
// the venue's date and time, whether the server is there and who is signed in;
// the section navigation on the left; and the `g` chords that move between
// sections. Each screen fills the rest and scrolls inside itself.
//
// The design's bar also draws a search and a bell. Nothing answers either, and
// a control that does nothing is not drawn (rebuild.md, rule 3).

import { CalendarIcon, LogOutIcon } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import { useLocation } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { ConnectionWord } from "../../../shared/shell/ConnectionWord";
import { ProfileMenu } from "../../../shared/shell/ProfileMenu";
import { Logo } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { useVenue } from "../../../shared/use-venue";
import { clockText } from "../../../shared/venue-text";
import { chord, sectionForPath, type ChordState } from "./nav-logic";
import { SectionNav } from "./SectionNav";

/** The signed-in staff member's permissions; none while not signed in. */
export function usePermissions(): readonly Permission[] {
  const status = useAuth();
  return status.kind === "signed_in" ? status.permissions : [];
}

export function BackOfficeShell({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const store = useAuthStore();
  const [path] = useLocation();
  useChords();
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <header className="flex h-18 shrink-0 items-center gap-6 border-b border-line bg-chrome px-5 short:h-15 2xl:px-7">
        <Logo className="h-9" />
        <div className="ml-auto flex items-center gap-5 whitespace-nowrap">
          <ConnectionWord brief className="text-sm" />
          <VenueClock />
          <ProfileMenu name={auth.staff.name} role={auth.staff.role} actions={[{ icon: LogOutIcon, label: "Sign out", onClick: () => void store.signOut() }]} />
        </div>
      </header>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col md:flex-row">
        <SectionNav current={sectionForPath(path)} held={auth.permissions} />
        {/* Positioned, so anything absolute in a screen (every sr-only label) scrolls
            and clips with it. Left to the page, one below the fold makes the whole
            page scroll, bar and navigation with it. */}
        <main className="relative min-h-0 min-w-0 flex-1 overflow-y-auto p-4 xl:p-6">{children}</main>
      </div>
    </div>
  );
}

/** The date and time in the venue's zone, so the bar is right on a laptop set to another. */
function VenueClock() {
  const venue = useVenue();
  const now = useClock(1000);
  return (
    <span className="hidden h-11 items-center gap-2 rounded-pill bg-sunken px-4 font-mono text-sm font-semibold tabular-nums lg:inline-flex">
      <CalendarIcon aria-hidden="true" size={17} />
      {clockText(new Date(now), venue?.timezone ?? null)}
    </span>
  );
}

/** Listens for `g` chords on the whole page while the shell is mounted. */
function useChords() {
  const [, navigate] = useLocation();
  const held = usePermissions();
  const state = useRef<ChordState>(null);
  // Kept in a ref so the listener is added once, not on every sign-in change.
  const latest = useRef({ held, navigate });
  useEffect(() => {
    latest.current = { held, navigate };
  }, [held, navigate]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const r = chord(
        state.current,
        { key: e.key, modified: e.ctrlKey || e.altKey || e.metaKey, typing: isTyping(e.target), now: e.timeStamp },
        latest.current.held,
      );
      state.current = r.state;
      if (r.go) {
        e.preventDefault();
        latest.current.navigate(r.go);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
}

/** Whether a key pressed here is text being typed rather than a command. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "SELECT", "TEXTAREA"].includes(target.tagName);
}
