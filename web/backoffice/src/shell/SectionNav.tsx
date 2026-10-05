// The back office's sidebar (D12): 240px, an icon and a name on every entry,
// the current one filled terracotta. The design draws nine entries in one run;
// the back office has eleven, so they keep their groups, each under its label
// (rebuild.md, rule 4). Under 768px it is a wrapped row above the screen. The
// script line at the foot is drawn only where the screen is tall enough for it.

import {
  ArmchairIcon,
  ChartColumnIcon,
  ClipboardListIcon,
  HouseIcon,
  MonitorSmartphoneIcon,
  PackageIcon,
  PrinterIcon,
  ReceiptTextIcon,
  SettingsIcon,
  UsersIcon,
  UtensilsIcon,
  type LucideIcon,
} from "lucide-react";
import { Link } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { visibleNav, type Section, type SectionId } from "./nav-logic";

// A Record over the ids, so a section added to NAV fails the build until it has an icon.
const icons: Record<SectionId, LucideIcon> = {
  dashboard: HouseIcon,
  menu: UtensilsIcon,
  stock: PackageIcon,
  people: UsersIcon,
  zreports: ReceiptTextIcon,
  audit: ClipboardListIcon,
  sales: ChartColumnIcon,
  floor: ArmchairIcon,
  printers: PrinterIcon,
  settings: SettingsIcon,
  devices: MonitorSmartphoneIcon,
};

interface SectionNavProps {
  current: Section | null;
  held: readonly Permission[];
}

export function SectionNav({ current, held }: SectionNavProps) {
  const groups = visibleNav(held);
  return (
    <nav
      aria-label="Sections"
      className="flex shrink-0 flex-col border-b border-line bg-chrome px-3.5 py-4 md:w-60 md:overflow-y-auto md:border-r md:border-b-0 md:[scrollbar-width:thin]"
    >
      {groups.map((g) => (
        <div key={g.label ?? "top"} className="mb-1 last:mb-0">
          {g.label && <h2 className="px-4 pb-1 pt-1.5 text-meta font-bold uppercase text-fg-faint">{g.label}</h2>}
          <ul className="flex flex-wrap gap-0.5 md:flex-col">
            {g.sections.map((s) => (
              <li key={s.id}>
                <NavEntry section={s} active={current?.id === s.id} />
              </li>
            ))}
          </ul>
        </div>
      ))}
      <p aria-hidden="true" className="mt-auto hidden px-2 pt-6 font-script text-total leading-tight text-fg-2 [@media(min-height:900px)]:md:block">
        Good Food
        <br />
        Happier
        <br />
        People ♡
      </p>
    </nav>
  );
}

const entry = "flex min-h-tap items-center gap-3 whitespace-nowrap rounded px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2";

function NavEntry({ section: s, active }: { section: Section; active: boolean }) {
  const Icon = icons[s.id];
  if (!s.built) {
    return (
      <span className={cx(entry, "text-fg-muted")}>
        <Icon aria-hidden="true" size={19} className="shrink-0" />
        {s.label}
        <span className="ml-auto text-meta font-semibold uppercase text-fg-faint">Soon</span>
      </span>
    );
  }
  return (
    <Link
      href={s.path}
      aria-current={active ? "page" : undefined}
      className={cx(entry, active ? "bg-attention font-semibold text-attention-fg" : "text-fg hover:bg-raised-high")}
    >
      <Icon aria-hidden="true" size={19} className="shrink-0" />
      {s.label}
      <kbd aria-label={`Keys: g then ${s.key}`} className={cx("ml-auto font-mono text-meta", !active && "text-fg-faint")}>
        g {s.key}
      </kbd>
    </Link>
  );
}
