// The shift gate, applied: see shift-gate.ts. Until the server has said whether a
// shift is open, nothing but a word is shown, so no screen renders against a
// guess, and an error reaching the server has its way out.

import type { ReactNode } from "react";
import { Redirect, useLocation } from "wouter";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { shiftGate } from "./shift-gate";
import { useTerminal } from "./terminal-context";

export function ShiftGate({ children }: { children: ReactNode }) {
  const { shift } = useTerminal();
  const [path] = useLocation();
  const gate = shiftGate(shift.shift, path);
  if (gate.kind === "go") return <Redirect to={gate.to} replace />;
  if (gate.kind === "wait") {
    return shift.error ? (
      <EmptyState icon={<InboxIcon />} line="Can't reach the mez server to check the shift." action={{ label: "Try again", onClick: shift.retry }} />
    ) : (
      <p className="m-auto text-base text-fg-muted">Checking the shift</p>
    );
  }
  return <>{children}</>;
}
