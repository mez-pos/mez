// The drawer has been counted and the day is being closed, and this person
// cannot close it (no `shift.manage`): a cashier or a waiter on another till,
// or anyone signing in after the count. Nothing more can be rung up on this
// shift, so they are told what is happening and given the one thing they can
// do, which is to lock the screen. Without this screen the gate and the close
// page sent each other the same person for ever.

import { LockIcon } from "lucide-react";
import { Button } from "../../../shared/ui";
import { ShiftDetailsCard, ShiftStage, type DetailRow } from "./ShiftStage";

interface ShiftClosingProps {
  details: readonly DetailRow[];
  onLock: () => void;
}

export function ShiftClosing({ details, onLock }: ShiftClosingProps) {
  return (
    <ShiftStage
      main={
        <>
          <p className="text-meta font-bold uppercase text-attention-text">Closing the day</p>
          <h1 className="mt-2.5 font-serif text-total">The drawer has been counted</h1>
          <p className="mt-2 text-base text-fg-muted">
            A manager is closing the day. Nothing more can be rung up on this shift; once the Z-report is issued, a new shift can be opened.
          </p>
          <p className="mt-6 text-base text-fg-muted">Lock this screen so the next person can sign in.</p>
          <Button size="charge" className="mt-4 w-full sm:w-auto" onClick={onLock}>
            <LockIcon aria-hidden="true" size={19} />
            Lock
          </Button>
        </>
      }
      aside={<ShiftDetailsCard rows={details} />}
    />
  );
}
