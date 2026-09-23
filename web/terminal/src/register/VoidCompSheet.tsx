// Void or comp a line: a reason, then a manager's approval. A void takes the
// line off the bill; a comp gives it away, and it still counts for the
// kitchen and for stock. Both stay on the ticket, struck or marked, forever.

import { useState } from "react";
import type { LineResponse, Override } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { ApprovalSheet } from "./ApprovalSheet";
import { compReasons, voidReasons } from "./cart-logic";
import { ReasonPicker } from "./ReasonPicker";

interface VoidCompSheetProps {
  kind: "void" | "comp";
  line: LineResponse;
  onSubmit: (reasonCode: string, approval: Override) => Promise<void>;
  onClose: () => void;
}

export function VoidCompSheet({ kind, line, onSubmit, onClose }: VoidCompSheetProps) {
  const [reason, setReason] = useState<string | null>(null);
  const voiding = kind === "void";
  return (
    <ApprovalSheet
      title={`${voiding ? "Void" : "Comp"} ${line.qty} × ${line.name}`}
      action={voiding ? "Void it" : "Comp it"}
      danger={voiding}
      ready={reason !== null}
      problem={reason ? null : "Choose a reason."}
      onSubmit={(approval) => onSubmit(reason ?? "", approval)}
      onClose={onClose}
    >
      <div className="flex flex-col gap-6">
        <p className="flex items-baseline justify-between text-base text-fg-2">
          <span>{voiding ? "Comes off the bill" : "Given away, still made"}</span>
          <Money amount={line.gross} />
        </p>
        <ReasonPicker label="Why?" reasons={voiding ? voidReasons : compReasons} chosen={reason} onChoose={setReason} />
      </div>
    </ApprovalSheet>
  );
}
