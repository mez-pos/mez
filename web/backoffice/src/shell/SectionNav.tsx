// The back office's sidebar (D12): 240px from 1280 wide and 208px under, an
// icon and a name on every entry, the current one filled terracotta. The
// design draws nine entries in one run; the back office has eleven, so they
// keep their groups, each under its label (rebuild.md, rule 4).
//
// Its widths are the frame's own, not `w-side` (TECH-17): an entry is a fixed
// run of icon, name and chord ("Sales reports  g s" needs about 200px), and
// w-side is 184 to 191px from 1024 to 1366 wide, where it would cut it. The
// screens beside it are what take the token widths.
//
// Under 768px there is no room for a sidebar beside a screen, and laid across
// the top it took half of a phone. There it is a drawer: closed until the
// bar's menu button opens it, over the screen, from the left, and closed
// again by choosing a section, by the scrim, or by Escape (ADR-311).

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
import { useGuardedLink } from "./leave-guard";
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

/** The id the bar's menu button names as what it opens. */
export const sectionNavId = "section-nav";

interface SectionNavProps {
  current: Section | null;
  held: readonly Permission[];
  /** Under 768px: whether the drawer is open. From 768px it is always there. */
  open: boolean;
  onClose: () => void;
}

export function SectionNav({ current, held, open, onClose }: SectionNavProps) {
  const groups = visibleNav(held);
  return (
    <>
      {/* The scrim: what is behind the drawer, and a tap on it closes. Never at 768px or over. */}
      {open && <button type="button" aria-label="Close the menu" tabIndex={-1} onClick={onClose} className="fixed inset-0 z-30 bg-(--scrim) md:hidden" />}
      <nav
        id={sectionNavId}
        aria-label="Sections"
        className={cx(
          "shrink-0 flex-col bg-chrome px-3 py-4 xl:px-3.5",
          // A drawer under 768px: over the screen, the height of the page.
          "fixed inset-y-0 left-0 z-40 w-72 max-w-[85vw] overflow-y-auto border-r border-line shadow-modal",
          open ? "flex" : "hidden",
          // A sidebar from there: in the page, always.
          "md:static md:z-auto md:flex md:w-52 md:max-w-none md:shadow-none md:scrollbar-thin xl:w-60",
        )}
      >
        {groups.map((g) => (
          <div key={g.label ?? "top"} className="mb-1 last:mb-0">
            {g.label && <h2 className="px-4 pb-1 pt-1.5 text-meta font-bold uppercase text-fg-faint">{g.label}</h2>}
            <ul className="flex flex-col gap-0.5">
              {g.sections.map((s) => (
                <li key={s.id}>
                  <NavEntry section={s} active={current?.id === s.id} onPick={onClose} />
                </li>
              ))}
            </ul>
          </div>
        ))}
        {/* What the letters at the right are (BO-14). Where the screen is short the
            entries need the height, and each letter says it on hover. */}
        <p className="mt-auto hidden px-4 pt-4 text-sm text-fg-muted md:block short:hidden">Keys: press g, then the letter beside a section.</p>
      </nav>
    </>
  );
}

const entry = "flex min-h-tap items-center gap-3 whitespace-nowrap rounded px-4 text-base outline-offset-2 outline-focus focus-visible:outline-3";

function NavEntry({ section: s, active, onPick }: { section: Section; active: boolean; onPick: () => void }) {
  const Icon = icons[s.id];
  // Asks first when the screen open holds a draft (ADR-387).
  const guarded = useGuardedLink();
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
      onClick={(e) => {
        onPick();
        guarded(e, s.path);
      }}
      aria-current={active ? "page" : undefined}
      className={cx(entry, active ? "bg-attention font-semibold text-attention-fg" : "text-fg hover:bg-raised-high")}
    >
      <Icon aria-hidden="true" size={19} className="shrink-0" />
      {s.label}
      {/* The chords are a keyboard's. Under 768px it is a phone, which has none. */}
      <kbd
        aria-label={`Keys: g then ${s.key}`}
        title={`Press g, then ${s.key}, to open ${s.label}`}
        className={cx("ml-auto hidden font-mono text-meta md:inline", !active && "text-fg-faint")}
      >
        g {s.key}
      </kbd>
    </Link>
  );
}
