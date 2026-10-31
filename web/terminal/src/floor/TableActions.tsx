// The panel's More Actions (D05): what the floor can do to a party that the
// design draws no button for. Moving it, joining and parting tables (ADR-088),
// firing the next course, the order's history, and letting go guests who
// ordered nothing. It opens upward: it sits at the foot of the panel.

import { ArrowRightLeftIcon, ChevronDownIcon, ClockIcon, FlameIcon, LinkIcon, LogOutIcon, UnlinkIcon } from "lucide-react";
import { cx } from "../../../shared/ui/cx";
import { ActionItem } from "../register/MoreActions";
import { useDisclosure } from "../../../shared/shell/use-disclosure";

interface TableActionsProps {
  /** The party is over more than one table. */
  joined: boolean;
  /** The next course held back, if one is. */
  nextCourse: number | null;
  /** Seated and nothing ordered: the guests can simply leave. */
  canRelease: boolean;
  disabled: boolean;
  onMove: () => void;
  onJoin: () => void;
  onPart: () => void;
  onFire: (course: number) => void;
  onHistory: () => void;
  onRelease: () => void;
}

export function TableActions({ joined, nextCourse, canRelease, disabled, onMove, onJoin, onPart, onFire, onHistory, onRelease }: TableActionsProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  return (
    <div ref={box} onBlur={leaves} className="relative shrink-0">
      <button
        ref={trigger}
        type="button"
        disabled={disabled}
        aria-expanded={open}
        aria-controls={panelId}
        aria-label="More actions"
        onClick={toggle}
        className={cx(
          "inline-flex min-h-charge items-center justify-center gap-2 whitespace-nowrap rounded-pill border border-line-strong bg-raised px-4 text-base font-semibold",
          "outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3 disabled:pointer-events-none disabled:opacity-40",
          open && "bg-raised-high",
        )}
      >
        {/* "More" where the rail is narrow: the chevron says the rest. */}
        <span className="hidden 2xl:inline">More Actions</span>
        <span className="2xl:hidden">More</span>
        <ChevronDownIcon aria-hidden="true" size={16} className={cx(open && "rotate-180")} />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="More actions"
          className="absolute bottom-full left-0 z-20 mb-2 w-72 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          <ActionItem icon={ArrowRightLeftIcon} label="Move to another table" onClick={pick(onMove)} />
          <ActionItem icon={LinkIcon} label="Join tables" onClick={pick(onJoin)} />
          {joined && <ActionItem icon={UnlinkIcon} label="Part tables" onClick={pick(onPart)} />}
          {nextCourse !== null && <ActionItem icon={FlameIcon} label={`Fire course ${nextCourse}`} onClick={pick(() => onFire(nextCourse))} />}
          <ActionItem icon={ClockIcon} label="Order history" onClick={pick(onHistory)} />
          {canRelease && <ActionItem icon={LogOutIcon} label="Guests left" onClick={pick(onRelease)} />}
        </div>
      )}
    </div>
  );
}
