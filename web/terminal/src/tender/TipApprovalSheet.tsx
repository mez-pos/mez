// A manager's approval for a tip over the venue's ceiling (ADR-261), on the
// payment screen and the waiter's bill alike. What is being approved, then who
// approves it and their PIN: the server checks that they hold tip.approve,
// and the cashier who keyed the tip cannot approve it alone.

import type { Money as Amount, Override } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { ApprovalSheet } from "../register/ApprovalSheet";

interface TipApprovalSheetProps {
  tip: Amount;
  /** What the payment settles of the bill: the ceiling is a share of it. */
  amount: Amount;
  /** The venue's ceiling, a whole percent, or null when the till has not read it. */
  ceiling: number | null;
  /** The last button: "Approve", or "Approve and record" when it sends the payment too. */
  action: string;
  /** Sends or keeps the approval; rejects when the server refuses the approval itself. */
  onSubmit: (approval: Override) => Promise<void>;
  onClose: () => void;
}

export function TipApprovalSheet({ tip, amount, ceiling, action, onSubmit, onClose }: TipApprovalSheetProps) {
  const why =
    ceiling === null
      ? " needs a manager's approval."
      : ceiling === 0
        ? " needs a manager's approval: this venue asks for one on every tip."
        : ` is over this venue's ceiling of ${ceiling}% of the payment.`;
  return (
    <ApprovalSheet title="Approve the tip" action={action} ready onSubmit={onSubmit} onClose={onClose}>
      <div className="flex flex-col gap-3">
        <p className="text-base text-fg-2">
          A tip of <Money amount={tip} className="font-bold text-fg" /> on a payment of <Money amount={amount} />
          {why}
        </p>
        <p className="text-sm text-fg-muted">
          A card or UPI tip is never counted in the drawer, and every tip is shared out with the shift's tips. Someone who may approve tips enters their PIN.
        </p>
      </div>
    </ApprovalSheet>
  );
}
