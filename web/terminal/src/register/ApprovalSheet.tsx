// A manager's approval for a void, a comp or a discount (Task 4). Two steps
// in one sheet: first what is being done and why (the caller's content),
// then who approves it and their PIN. The server checks that person's PIN
// and permission; the cashier's own session is not enough (ADR-072).

import { useQuery } from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";
import type { Override, StaffListResponse, StaffResponse } from "../../../shared/api-types";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { minPin, PinDots } from "../../../shared/screens/PinDots";
import { applyPinKey, Button, NumPad, Sheet, type NumKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { approvalFailure } from "./order-messages";

interface ApprovalSheetProps {
  title: string;
  /** "Void", "Comp", "Apply discount": the last button. */
  action: string;
  /** The first step: the reason, and for a discount its size. */
  children: ReactNode;
  /** Whether the first step is complete. */
  ready: boolean;
  /** What stops the first step, in words, or null. */
  problem?: string | null;
  /** Sends it with the approval; rejects when the server refuses. */
  onSubmit: (approval: Override) => Promise<void>;
  onClose: () => void;
  /** Tone of the last button: a void is danger, the rest are the accent. */
  danger?: boolean;
}

export function ApprovalSheet({ title, action, children, ready, problem, onSubmit, onClose, danger }: ApprovalSheetProps) {
  const [step, setStep] = useState<"what" | "who">("what");
  const [approver, setApprover] = useState<StaffResponse | null>(null);
  const [pin, setPin] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    if (!approver) return;
    setBusy(true);
    setError(null);
    try {
      await onSubmit({ authorised_by: approver.id, pin });
      onClose();
    } catch (err) {
      setError(approvalFailure(err, approver.name));
      setPin("");
      setBusy(false);
    }
  };

  const footer =
    step === "what" ? (
      <div className="flex flex-col gap-2">
        <p className="min-h-6 text-sm text-fg-muted" role="status">
          {problem}
        </p>
        <Button variant="primary" size="charge" className="w-full" disabled={!ready} onClick={() => setStep("who")}>
          Continue to approval
        </Button>
      </div>
    ) : (
      <div className="flex flex-col gap-2">
        <p className="min-h-6 text-sm text-danger" role="status">
          {error}
        </p>
        <div className="flex gap-tap-gap">
          <Button onClick={() => setStep("what")} disabled={busy}>
            Back
          </Button>
          <Button
            variant={danger ? "danger" : "primary"}
            size="charge"
            className="flex-1"
            disabled={!approver || pin.length < minPin || busy}
            onClick={() => void submit()}
          >
            {action}
          </Button>
        </div>
      </div>
    );

  return (
    <Sheet open onClose={onClose} title={title} footer={footer}>
      {step === "what" ? (
        children
      ) : (
        <Approver
          approver={approver}
          onChoose={(s) => {
            setApprover(s);
            setPin("");
            setError(null);
          }}
          pin={pin}
          // Functional: taps quicker than a render must each count.
          onKey={(k) => setPin((p) => applyPinKey(p, k))}
          disabled={busy}
        />
      )}
    </Sheet>
  );
}

interface ApproverProps {
  approver: StaffResponse | null;
  onChoose: (staff: StaffResponse) => void;
  pin: string;
  onKey: (key: NumKey) => void;
  disabled: boolean;
}

function Approver({ approver, onChoose, pin, onKey, disabled }: ApproverProps) {
  const store = useAuthStore();
  const status = useAuth();
  const me = status.kind === "signed_in" ? status.staff.id : null;
  const staff = useQuery({
    queryKey: ["staff"],
    queryFn: () => store.api.get<StaffListResponse>("/api/v1/staff").then((r) => r.staff),
  });

  // The person signed in is the likeliest approver: a manager voiding their
  // own mistake. A cashier taps the manager's name instead.
  const mine = staff.data?.find((s) => s.id === me) ?? null;
  useEffect(() => {
    if (!approver && mine) onChoose(mine);
  }, [approver, mine, onChoose]);
  const chosen = approver ?? mine;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-base text-fg-2">Who is approving this?</p>
      {staff.isPending && <p className="text-sm text-fg-muted">Loading staff</p>}
      {staff.isError && <p className="text-sm text-danger">Can't load the staff list. Check the connection.</p>}
      <ul className="grid grid-cols-2 gap-tap-gap">
        {staff.data?.map((s) => (
          <li key={s.id}>
            <button
              type="button"
              aria-pressed={chosen?.id === s.id}
              onClick={() => onChoose(s)}
              className={cx(
                "min-h-tap w-full rounded border px-4 text-left text-base",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                chosen?.id === s.id ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line bg-raised text-fg-2",
              )}
            >
              {s.name}
            </button>
          </li>
        ))}
      </ul>
      <div>
        <p className="text-sm text-fg-muted">{chosen ? `${chosen.name}'s PIN` : "Choose who is approving"}</p>
        <PinDots length={pin.length} />
      </div>
      <NumPad layout="pin" label="Approval PIN" disabled={!chosen || disabled} onKey={onKey} />
    </div>
  );
}
