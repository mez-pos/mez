// The frame around every back office screen (D12): a 72px bar with the logo,
// the venue's date and time, whether the server is there and who is signed in;
// the section navigation on the left; and the `g` chords that move between
// sections. Each screen fills the rest and scrolls inside itself.
//
// The design's bar also draws a search and a bell. Nothing answers either, and
// a control that does nothing is not drawn (rebuild.md, rule 3).

import { useQueryClient } from "@tanstack/react-query";
import { CalendarIcon, LogOutIcon, MenuIcon } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { ConnectionWord } from "../../../shared/shell/ConnectionWord";
import { ProfileMenu } from "../../../shared/shell/ProfileMenu";
import { Logo } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { useAway, useRealtimeNotices } from "../../../shared/use-realtime";
import { useVenue } from "../../../shared/use-venue";
import { clockText } from "../../../shared/venue-text";
import { chord, sectionForPath, type ChordState } from "./nav-logic";
import { SectionNav, sectionNavId } from "./SectionNav";

/** The signed-in staff member's permissions; none while not signed in. */
export function usePermissions(): readonly Permission[] {
  const status = useAuth();
  return status.kind === "signed_in" ? status.permissions : [];
}

export function BackOfficeShell({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const store = useAuthStore();
  const [path] = useLocation();
  // Under 768px the sections are a drawer, and this is whether it is open.
  const [navOpen, setNavOpen] = useState(false);
  useChords();
  useFreshWhenAway();
  useEscape(navOpen, () => setNavOpen(false));
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <header className="flex h-18 shrink-0 items-center gap-2 border-b border-line bg-chrome px-3 short:h-15 md:gap-6 md:px-5 2xl:px-7">
        {/* Under 768px: the way to the sections, which are a drawer there. */}
        <button
          type="button"
          aria-label="Menu"
          aria-expanded={navOpen}
          aria-controls={sectionNavId}
          onClick={() => setNavOpen((o) => !o)}
          className="flex size-12 shrink-0 items-center justify-center rounded outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2 md:hidden"
        >
          <MenuIcon aria-hidden="true" size={24} />
        </button>
        {/* The mark alone under 640px, where the wordmark would cost the connection its words;
            never squeezed either way: a flex row shrinks an svg to nothing. */}
        <Logo variant="mark" className="h-7 shrink-0 sm:hidden" />
        <Logo className="hidden h-8 shrink-0 sm:block md:h-9" />
        <div className="ml-auto flex min-w-0 items-center gap-2 whitespace-nowrap md:gap-5">
          <ConnectionWord brief className="min-w-0 text-sm" />
          <VenueClock />
          <ProfileMenu
            compact
            name={auth.staff.name}
            role={auth.staff.role}
            actions={[{ icon: LogOutIcon, label: "Sign out", onClick: () => void store.signOut() }]}
          />
        </div>
      </header>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col md:flex-row">
        <SectionNav current={sectionForPath(path)} held={auth.permissions} open={navOpen} onClose={() => setNavOpen(false)} />
        {/* Positioned, so anything absolute in a screen (every sr-only label) scrolls
            and clips with it. Left to the page, one below the fold makes the whole
            page scroll, bar and navigation with it. */}
        <main className="relative min-h-0 min-w-0 flex-1 overflow-y-auto p-4 xl:p-6">{children}</main>
      </div>
    </div>
  );
}

/**
 * Away from the venue nothing tells a screen that something changed: the
 * relay carries no stream (ADR-309). The connection says "read again" when
 * someone comes back to the page, and here that means every query, since
 * only four of them listen for it themselves. At the venue this does
 * nothing: there the stream names what moved.
 */
function useFreshWhenAway() {
  const away = useAway();
  const queryClient = useQueryClient();
  useRealtimeNotices((notice) => {
    if (away && notice.kind === "resync") void queryClient.invalidateQueries();
  });
}

/** Escape closes what is open, while it is. */
function useEscape(active: boolean, close: () => void) {
  const latest = useRef(close);
  useEffect(() => {
    latest.current = close;
  }, [close]);
  useEffect(() => {
    if (!active) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") latest.current();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active]);
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
