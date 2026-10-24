// A manager's approval for a void, a comp or a discount (Task 4). Two steps
// in one sheet: first what is being done and why (the caller's content),
// then who approves it and their PIN. The server checks that person's PIN
// and permission; the cashier's own session is not enough (ADR-072).

import { useEffect, useState, type ReactNode } from "react";
import { isUnreachable } from "../../../shared/api";
import type { Override, StaffResponse } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { minPin, PinDots } from "../../../shared/screens/PinDots";
import { applyPinKey, Button, Modal, NumPad, Sheet, type NumKey } from "../../../shared/ui";
import { keyFromKeyboard } from "../../../shared/ui/numpad-keys";
import { cx } from "../../../shared/ui/cx";
import { useStaffList } from "../../../shared/use-staff-list";
import { approvalFailure } from "./order-messages";

interface ApprovalSheetProps {
  title: string;
  /** "Void", "Comp", "Apply discount": the last button. */
  action: ReactNode;
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
  /** Drawn in the middle of the screen, as D09 draws the refund, with a mark over its title. */
  modal?: { icon: ReactNode };
}

export function ApprovalSheet({ title, action, children, ready, problem, onSubmit, onClose, danger, modal }: ApprovalSheetProps) {
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
      // A refusal needs the PIN again; a server not reached keeps it for the retry.
      if (!isUnreachable(err)) setPin("");
      setBusy(false);
    }
  };

  // A keyboard types the PIN wherever the focus is, as the sign-in screen
  // does, and Enter approves: the pad's own listener heard only a focused
  // pad, which a tap on the approver's name is not.
  useEffect(() => {
    if (step !== "who" || !approver) return;
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.defaultPrevented || event.target instanceof HTMLInputElement || busy) return;
      if (event.key === "Enter") {
        if (pin.length < minPin) return;
        event.preventDefault();
        void submit();
        return;
      }
      const k = keyFromKeyboard(event.key);
      if (k) {
        event.preventDefault();
        setPin((p) => applyPinKey(p, k));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- submit is rebuilt each render; the state it reads is listed
  }, [step, approver, pin, busy]);

  const footer =
    step === "what" ? (
      <div className="flex flex-col gap-2">
        <p className="min-h-6 text-sm text-fg-muted" role="status">
          {problem}
        </p>
        <div className="flex gap-tap-gap">
          {modal && (
            <Button size="charge" className="flex-1" onClick={onClose}>
              Cancel
            </Button>
          )}
          <Button variant="primary" size="charge" className="flex-1" disabled={!ready} onClick={() => setStep("who")}>
            Continue to approval
          </Button>
        </div>
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

  const body =
    step === "what" ? (
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
    );

  if (modal) {
    return (
      <Modal open onClose={onClose} title={title} size="lg" icon={modal.icon}>
        {body}
        <div className="mt-5">{footer}</div>
      </Modal>
    );
  }
  return (
    <Sheet open onClose={onClose} title={title} footer={footer}>
      {body}
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
  const status = useAuth();
  const me = status.kind === "signed_in" ? status.staff.id : null;
  const staff = useStaffList();

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
