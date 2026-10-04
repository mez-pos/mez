// The shift is not open and this person cannot open it (ADR-150): a waiter, or
// anyone whose role lacks `shift.open` (ADR-152). They are told who can, and given the
// one thing they can do, which is to lock the screen so that person can sign in.

import { LockIcon } from "lucide-react";
import type { StaffResponse } from "../../../shared/api-types";
import { initials, roleText } from "../../../shared/auth/format";
import { Button } from "../../../shared/ui";
import { ShiftDetailsCard, ShiftStage, SoldOutCard, type DetailRow } from "./ShiftStage";

interface ShiftNotOpenProps {
  openers: readonly StaffResponse[];
  details: readonly DetailRow[];
  soldOut: { title: string; body: string } | null;
  onLock: () => void;
}

export function ShiftNotOpen({ openers, details, soldOut, onLock }: ShiftNotOpenProps) {
  return (
    <ShiftStage
      main={
        <>
          <p className="text-meta font-bold uppercase text-attention-text">Shift not open</p>
          <h1 className="mt-2.5 font-serif text-total">{"The shift isn't open yet"}</h1>
          <p className="mt-2 text-base text-fg-muted">{"Orders can't be taken until the cash drawer is counted and the shift is opened."}</p>

          <h2 className="mt-8 text-meta font-bold uppercase text-fg-muted">Who can open it</h2>
          {openers.length > 0 ? (
            <ul className="mt-3 flex flex-col gap-2">
              {openers.map((person) => (
                <li key={person.id} className="flex items-center gap-3 rounded border border-line p-3">
                  <span
                    aria-hidden="true"
                    className="flex size-10 shrink-0 items-center justify-center rounded-round bg-accent-tint text-sm font-bold text-accent-text"
                  >
                    {initials(person.name)}
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-base font-semibold">{person.name}</span>
                    <span className="block text-sm text-fg-muted">{roleText(person.role)}</span>
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="mt-3 text-base">Ask the owner or a manager.</p>
          )}

          <p className="mt-6 text-base text-fg-muted">Lock this screen and they can sign in.</p>
          <Button size="charge" className="mt-4 w-full sm:w-auto" onClick={onLock}>
            <LockIcon aria-hidden="true" size={19} />
            Lock
          </Button>
        </>
      }
      aside={
        <>
          <ShiftDetailsCard rows={details} />
          {soldOut && <SoldOutCard title={soldOut.title} body={soldOut.body} menuHref={null} />}
        </>
      }
    />
  );
}
