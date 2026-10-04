// The 240px sidebar of the tables screen (D05): Floor Plan and Table List, the
// two ways to look at the same tables, and Order History. The one chosen is
// terracotta, as the order screen's categories are. The design also lists
// Reservations, Waitlist, Customers and Settings: none exists in v1, and a
// control that does nothing is not drawn (rebuild.md, rule 3).

import { HistoryIcon, LayoutGridIcon, ListIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import { cx } from "../../../shared/ui/cx";
import { route } from "../shell/routes";

export type TablesView = "plan" | "list";

interface TablesSidebarProps {
  view: TablesView;
  onView: (view: TablesView) => void;
}

const views: readonly { view: TablesView; label: string; icon: LucideIcon }[] = [
  { view: "plan", label: "Floor Plan", icon: LayoutGridIcon },
  { view: "list", label: "Table List", icon: ListIcon },
];

const entry = "flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base outline-offset-2 outline-attention focus-visible:outline-3";

export function TablesSidebar({ view, onView }: TablesSidebarProps) {
  return (
    <nav aria-label="Tables" className="flex w-60 shrink-0 flex-col gap-1 overflow-y-auto border-r border-line bg-chrome px-3.5 py-4">
      {views.map(({ view: v, label, icon: Icon }) => {
        const active = v === view;
        return (
          <button
            key={v}
            type="button"
            aria-pressed={active}
            onClick={() => onView(v)}
            className={cx(entry, active ? "bg-attention font-semibold text-attention-fg" : "text-fg hover:bg-raised-high")}
          >
            <Icon aria-hidden="true" size={19} className="shrink-0" />
            {label}
          </button>
        );
      })}
      <Link href={route.orders} className={cx(entry, "text-fg hover:bg-raised-high")}>
        <HistoryIcon aria-hidden="true" size={19} className="shrink-0" />
        Order History
      </Link>
      <p aria-hidden="true" className="mt-auto px-2 pt-6 font-script text-total leading-tight text-fg-2">
        Good Food
        <br />
        Happier
        <br />
        People ♡
      </p>
    </nav>
  );
}
