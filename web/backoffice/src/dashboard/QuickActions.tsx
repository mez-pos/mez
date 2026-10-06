// The dashboard's shortcuts (D12, "Quick actions") and the brand's card under
// them. A shortcut is one of the navigation's own sections, so it is shown to
// whoever may open that section and to nobody else.

import { ChartColumnIcon, LeafIcon, PackageIcon, UsersIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { Card } from "../../../shared/ui";
import { canOpen, section, type SectionId } from "../shell/nav-logic";

// The design's first shortcut is "Add menu item"; the link opens the menu, so it says that.
const shortcuts: readonly { id: SectionId; label: string; icon: LucideIcon }[] = [
  { id: "menu", label: "Edit the menu", icon: UtensilsIcon },
  { id: "stock", label: "Inventory", icon: PackageIcon },
  { id: "sales", label: "View reports", icon: ChartColumnIcon },
  { id: "people", label: "Manage staff", icon: UsersIcon },
];

/** The shortcuts this staff member may use. */
export function quickActions(held: readonly Permission[]) {
  return shortcuts.filter((a) => canOpen(section(a.id), held));
}

export function QuickActions({ held }: { held: readonly Permission[] }) {
  const actions = quickActions(held);
  if (actions.length === 0) return null;
  return (
    <Card padding="lg">
      <h2 className="font-serif text-xl">Quick actions</h2>
      <ul className="mt-3 grid grid-cols-[repeat(auto-fit,minmax(9.5rem,1fr))] gap-2.5">
        {actions.map(({ id, label, icon: Icon }) => (
          <li key={id}>
            <Link
              href={section(id).path}
              className="flex min-h-tap items-center gap-2.5 rounded border border-line-strong px-3 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
            >
              <Icon aria-hidden="true" size={17} className="shrink-0" />
              <span className="truncate">{label}</span>
            </Link>
          </li>
        ))}
      </ul>
    </Card>
  );
}

/** The brand's card: a leaf and a line of script. Decoration, so nothing reads it out. */
export function BrandCard() {
  return (
    <div
      aria-hidden="true"
      className="relative flex min-h-28 flex-1 items-center overflow-hidden rounded-card border border-chip-success-border bg-accent-tint p-6"
    >
      <LeafIcon size={140} strokeWidth={1} className="absolute -bottom-6 -left-6 text-accent opacity-15" />
      <p className="relative ml-auto text-right font-script text-total leading-tight text-fg-2">
        Good Food,
        <br />
        Brighter Days ♡
      </p>
    </div>
  );
}
