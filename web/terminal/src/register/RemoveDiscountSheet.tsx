// Taking a discount off the order (ADR-373): the one it is, why it goes, then
// a manager's approval, as giving it took one. Without this a discount larger
// than what was left trapped the order: a line could not be voided under it,
// and the only way out was to pay.

import { useState } from "react";
import type { DiscountResponse, Override } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { humanise } from "../../../shared/ui/timeline-entries";
import { ApprovalSheet } from "./ApprovalSheet";
import { ReasonPicker } from "./ReasonPicker";
import { removalReasons } from "./cart-logic";

interface RemoveDiscountSheetProps {
  discount: DiscountResponse;
  onSubmit: (reason: string, approval: Override) => Promise<void>;
  onClose: () => void;
}

export function RemoveDiscountSheet({ discount, onSubmit, onClose }: RemoveDiscountSheetProps) {
  const [reason, setReason] = useState<string | null>(null);
  const label = humanise(discount.reason) ?? discount.reason;
  return (
    <ApprovalSheet
      title="Remove discount"
      action="Remove discount"
      ready={reason !== null}
      problem={reason ? null : "Choose a reason."}
      onSubmit={(approval) => onSubmit(reason ?? "", approval)}
      onClose={onClose}
    >
      <div className="flex flex-col gap-5">
        <p className="flex items-baseline justify-between text-base">
          <span>
            Discount · {label}
            {discount.scope === "line" && " (on one line)"}
          </span>
          <Money amount={{ ...discount.value, minor: -discount.value.minor }} tone="danger" />
        </p>
        <p className="text-base text-fg-muted">The order goes back to what it was before the discount. The discount stays in the order&apos;s history.</p>
        <ReasonPicker reasons={removalReasons} chosen={reason} onChoose={setReason} label="Why it goes" />
      </div>
    </ApprovalSheet>
  );
}
