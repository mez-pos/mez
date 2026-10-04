// What every screen of the terminal shares: the order in front, the floor, the
// shift, and the Z-report this till's close just issued. They live here, above
// the routes, because each is a hook with state of its own: a screen that
// called them itself would start a fresh ticket every time someone moved to
// another screen and back (ADR-148).

import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { tableNames } from "../floor/floor-logic";
import { useLiveFloor, type LiveFloorApi } from "../floor/use-live-floor";
import type { Ticket } from "../register/use-ticket";
import { useTicket } from "../register/use-ticket";
import { useShift, type ShiftState } from "../shift/use-shift";

export interface Terminal {
  ticket: Ticket;
  floor: LiveFloorApi;
  shift: ShiftState;
  /** Table names by id, for labels. */
  names: ReadonlyMap<number, string>;
  /** The Z-report this till's last close issued, until the page is reloaded. */
  lastZ: number | null;
  setLastZ: (number: number | null) => void;
}

const TerminalContext = createContext<Terminal | null>(null);

export function TerminalProvider({ children }: { children: ReactNode }) {
  const ticket = useTicket();
  const floor = useLiveFloor();
  const shift = useShift();
  const [lastZ, setLastZ] = useState<number | null>(null);
  const names = useMemo(() => tableNames(floor.data), [floor.data]);
  // Not memoised: each hook returns a fresh object every render, so a memo
  // here would never hold.
  const value: Terminal = { ticket, floor, shift, names, lastZ, setLastZ };
  return <TerminalContext.Provider value={value}>{children}</TerminalContext.Provider>;
}

export function useTerminal(): Terminal {
  const value = useContext(TerminalContext);
  if (!value) throw new Error("useTerminal outside the TerminalProvider");
  return value;
}
