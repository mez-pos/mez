// A draft is never dropped silently (ADR-381 item 5, BO-2, BO-5, ADR-387).
// A form with unsaved changes says so with useDraftHeld; every way out of it
// asks "Leave without saving?" first: the sidebar, a `g` chord and Sign out
// (the shell's), another settings section or another menu item (a screen's
// own, through useLeave). Closing or reloading the tab gets the browser's
// own prompt. Leaving runs the move; the forms left behind unmount and let
// their drafts go.

import { createContext, useCallback, useContext, useEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";
import { useLocation } from "wouter";
import { Button, Modal } from "../../../shared/ui";
import { holdDraft, leaveText, noDrafts, opensElsewhere, type Drafts } from "./leave-logic";

interface LeaveGuard {
  hold: (key: symbol, what: string | null) => void;
  leave: (go: () => void) => void;
}

const LeaveGuardContext = createContext<LeaveGuard | null>(null);

export function LeaveGuardProvider({ children }: { children: ReactNode }) {
  const [drafts, setDrafts] = useState<Drafts>(noDrafts);
  const [asking, setAsking] = useState<(() => void) | null>(null);
  // Read by leave(), which is handed out once and must see the drafts as they are now.
  const held = useRef(drafts);
  useEffect(() => {
    held.current = drafts;
  }, [drafts]);

  const hold = useCallback((key: symbol, what: string | null) => setDrafts((d) => holdDraft(d, key, what)), []);
  const leave = useCallback((go: () => void) => {
    if (held.current.size === 0) go();
    else setAsking(() => go);
  }, []);
  const [guard] = useState<LeaveGuard>(() => ({ hold, leave }));

  // The browser's own prompt when the tab is closed or reloaded on a draft.
  const any = drafts.size > 0;
  useEffect(() => {
    if (!any) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [any]);

  return (
    <LeaveGuardContext.Provider value={guard}>
      {children}
      <Modal
        open={asking !== null}
        onClose={() => setAsking(null)}
        title="Leave without saving?"
        footer={
          <>
            <Button onClick={() => setAsking(null)}>Keep editing</Button>
            <Button
              variant="primary"
              onClick={() => {
                const go = asking;
                setAsking(null);
                go?.();
              }}
            >
              Leave
            </Button>
          </>
        }
      >
        {leaveText(drafts)}
      </Modal>
    </LeaveGuardContext.Provider>
  );
}

/**
 * Says this form holds a draft while dirty is true: what is the words the
 * question names it by ("the receipt", "Filter coffee"). Outside the shell it
 * does nothing.
 */
export function useDraftHeld(dirty: boolean, what: string): void {
  const guard = useContext(LeaveGuardContext);
  const [key] = useState(() => Symbol(what));
  useEffect(() => {
    guard?.hold(key, dirty ? what : null);
  }, [guard, key, dirty, what]);
  useEffect(() => () => guard?.hold(key, null), [guard, key]);
}

/** Runs a move away from the open drafts now, or once the person says to leave them. */
export function useLeave(): (go: () => void) => void {
  return useContext(LeaveGuardContext)?.leave ?? run;
}

const run = (go: () => void) => go();

/**
 * A link's click, asked about first: one that opens another tab, or goes
 * where the page already is, leaves nothing and is let through.
 */
export function useGuardedLink(): (e: MouseEvent, path: string) => void {
  const leave = useLeave();
  const [location, navigate] = useLocation();
  return (e, path) => {
    if (opensElsewhere(e) || path === location) return;
    e.preventDefault();
    leave(() => navigate(path));
  };
}
