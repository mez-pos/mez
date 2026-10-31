// The top navigation: Orders, Tables and, for those who may open them, Menu and
// Reports (D05). A pill is 48px, not the design's 44: the floor for anything
// tapped during service is 48 (ADR-014). The pressed pill is green with ivory
// text, as the design draws it.

import { ArmchairIcon, ChartColumnIcon, ReceiptTextIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import { cx } from "../../../shared/ui/cx";
import type { NavId, NavItem } from "./nav-logic";

const icons: Record<NavId, LucideIcon> = {
  orders: ReceiptTextIcon,
  tables: ArmchairIcon,
  menu: UtensilsIcon,
  reports: ChartColumnIcon,
};

interface NavPillsProps {
  items: readonly NavItem[];
  active: NavId | null;
  /**
   * Asked before an external link is followed: the back office is another
   * bundle, and a page load drops whatever the till holds in memory. False
   * keeps the person here.
   */
  mayLeave?: () => boolean;
}

export function NavPills({ items, active, mayLeave }: NavPillsProps) {
  return (
    <nav aria-label="Main" className="flex gap-1.5">
      {items.map((item) => {
        const Icon = icons[item.id];
        const pressed = item.id === active;
        const shape = cx(
          "inline-flex min-h-tap items-center gap-2.5 rounded-pill px-5 text-base outline-offset-2 outline-focus focus-visible:outline-3",
          pressed ? "bg-accent font-semibold text-accent-fg" : "font-medium text-fg hover:bg-raised-high",
        );
        const body = (
          <>
            <Icon aria-hidden="true" size={18} />
            {item.label}
          </>
        );
        return item.external ? (
          <a
            key={item.id}
            href={item.href}
            className={shape}
            onClick={(event) => {
              if (mayLeave && !mayLeave()) event.preventDefault();
            }}
          >
            {body}
          </a>
        ) : (
          <Link key={item.id} href={item.href} aria-current={pressed ? "page" : undefined} className={shape}>
            {body}
          </Link>
        );
      })}
    </nav>
  );
}
