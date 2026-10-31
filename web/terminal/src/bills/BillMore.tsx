// The order panel's More (terminal-past-bills.md): the invoice as issued, the
// order's history and voiding a payment, which are needed now and then and
// cost the panel a row of buttons each when drawn (TBS-16). It opens upward:
// it sits at the foot of the panel.

import { BanIcon, ChevronDownIcon, FileTextIcon, HistoryIcon } from "lucide-react";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { cx } from "../../../shared/ui/cx";
import { ActionItem } from "../register/MoreActions";

interface BillMoreProps {
  /** Shown only once the bill has an invoice. */
  onInvoice: (() => void) | null;
  onHistory: () => void;
  /** Shown only while a payment can be voided. */
  onVoid: (() => void) | null;
}

export function BillMore({ onInvoice, onHistory, onVoid }: BillMoreProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  return (
    <div ref={box} onBlur={leaves} className="relative flex flex-auto">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(
          "inline-flex min-h-tap flex-auto items-center justify-center gap-2 whitespace-nowrap rounded-pill border border-line-strong bg-raised px-4 text-base font-semibold",
          "outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3",
          open && "bg-raised-high",
        )}
      >
        More
        <ChevronDownIcon aria-hidden="true" size={16} className={cx(open && "rotate-180")} />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="More for this bill"
          className="absolute bottom-full right-0 z-20 mb-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          {onInvoice && <ActionItem icon={FileTextIcon} label="Invoice" onClick={pick(onInvoice)} />}
          <ActionItem icon={HistoryIcon} label="Order history" onClick={pick(onHistory)} />
          {onVoid && <ActionItem icon={BanIcon} label="Void a payment" onClick={pick(onVoid)} />}
        </div>
      )}
    </div>
  );
}
