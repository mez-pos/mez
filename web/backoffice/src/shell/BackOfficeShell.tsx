// The frame around every back office screen: the shared signed-in header,
// the section navigation on the left, and the `g` chords that move between
// sections (docs/design/screens/backoffice-shell.md).

import { useEffect, useRef, type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { SignedInShell } from "../../../shared/screens/SignedInShell";
import { chord, sectionForPath, visibleNav, type ChordState, type Section } from "./nav-logic";

/** The signed-in staff member's permissions; none while not signed in. */
export function usePermissions(): readonly Permission[] {
  const status = useAuth();
  return status.kind === "signed_in" ? status.permissions : [];
}

export function BackOfficeShell({ children }: { children: ReactNode }) {
  const [path] = useLocation();
  const current = sectionForPath(path);
  useChords();

  return (
    <SignedInShell area={current?.label ?? "Back office"} exitLabel="Sign out" fill>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col md:flex-row">
        <SectionNav current={current} />
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto p-8">{children}</div>
      </div>
    </SignedInShell>
  );
}

function SectionNav({ current }: { current: Section | null }) {
  const groups = visibleNav(usePermissions());
  return (
    <nav
      aria-label="Sections"
      className="shrink-0 border-b border-line bg-chrome px-3 py-4 md:w-60 md:overflow-y-auto md:border-r md:border-b-0"
    >
      {groups.map((g) => (
        <div key={g.label ?? "top"} className="mb-4 last:mb-0">
          {g.label && <h2 className="px-3 pb-1.5 text-meta font-semibold uppercase text-fg-faint">{g.label}</h2>}
          <ul className="flex flex-wrap gap-1 md:flex-col">
            {g.sections.map((s) => (
              <li key={s.id}>
                <NavEntry section={s} active={current?.id === s.id} />
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}

// The active section is the terracotta fill, the design's sidebar voice.
const entry = "flex min-h-10 items-center justify-between gap-3 rounded-pill px-4 text-base";

function NavEntry({ section: s, active }: { section: Section; active: boolean }) {
  if (!s.built) {
    return (
      <span className={`${entry} text-fg-muted`}>
        {s.label}
        <span className="text-meta font-semibold uppercase text-fg-faint">Soon</span>
      </span>
    );
  }
  return (
    <Link
      href={s.path}
      aria-current={active ? "page" : undefined}
      className={
        active
          ? `${entry} bg-attention font-semibold text-attention-fg`
          : `${entry} text-fg-2 hover:bg-attention-tint focus-visible:bg-attention-tint`
      }
    >
      {s.label}
      <kbd aria-label={`Keys: g then ${s.key}`} className={active ? "font-mono text-meta" : "font-mono text-meta text-fg-faint"}>
        g {s.key}
      </kbd>
    </Link>
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
