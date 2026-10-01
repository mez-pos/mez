// The back office's sections: which exist, who may open them, and the keys
// that reach them (docs/design/screens/backoffice-shell.md). The navigation,
// the dashboard and the route guard all read this one list, so they cannot
// disagree about what a staff member may see.

import type { Permission } from "../../../shared/api-types";

export type SectionId =
  | "dashboard"
  | "menu"
  | "stock"
  | "people"
  | "zreports"
  | "sales"
  | "floor"
  | "printers"
  | "settings"
  | "devices";

export interface Section {
  readonly id: SectionId;
  readonly label: string;
  readonly path: string;
  /** Other paths that belong to this section: a report, a shift's tips. */
  readonly also?: readonly string[];
  /** Null: anyone signed in. */
  readonly needs: Permission | null;
  /** The letter after `g`. */
  readonly key: string;
  /** False until its task lands: shown as Soon, not a link. */
  readonly built: boolean;
  /** One line on the dashboard. */
  readonly description: string;
}

export interface NavGroup {
  /** Null for the ungrouped first entry. */
  readonly label: string | null;
  readonly sections: readonly Section[];
}

export const NAV: readonly NavGroup[] = [
  {
    label: null,
    sections: [
      { id: "dashboard", label: "Dashboard", path: "/", needs: null, key: "d", built: true, description: "Today at a glance." },
    ],
  },
  {
    label: "Run the venue",
    sections: [
      { id: "menu", label: "Menu", path: "/menu", needs: "menu.edit", key: "m", built: true, description: "Categories, items, modifiers and prices." },
      { id: "stock", label: "Stock", path: "/stock", needs: "stock.manage", key: "k", built: true, description: "Stock items, recipes, wastage and counts." },
      { id: "people", label: "People", path: "/people", needs: "staff.manage", key: "p", built: true, description: "Staff, roles, PINs, and who voided what." },
    ],
  },
  {
    label: "Reports",
    sections: [
      {
        id: "zreports",
        label: "Z-reports",
        path: "/reports",
        also: ["/shifts"],
        needs: "reports.view",
        key: "r",
        built: true,
        description: "Every closed day, its exports and its tips.",
      },
      { id: "sales", label: "Sales reports", path: "/sales", needs: "reports.view", key: "s", built: false, description: "Sales over time, product mix, staff and voids." },
    ],
  },
  {
    label: "Settings",
    sections: [
      { id: "floor", label: "Floor plan", path: "/floor", needs: "settings.edit", key: "f", built: true, description: "Areas and tables." },
      { id: "printers", label: "Printers", path: "/printers", needs: "settings.edit", key: "t", built: true, description: "Receipt and kitchen printers, and where each ticket prints." },
      { id: "settings", label: "Venue settings", path: "/settings", needs: "settings.edit", key: "v", built: false, description: "Tax, invoice series, currency, shift and tip rules." },
      { id: "devices", label: "Devices", path: "/devices", needs: "device.manage", key: "e", built: false, description: "Tills, kitchen displays and back offices paired here." },
    ],
  },
];

const ALL: readonly Section[] = NAV.flatMap((g) => g.sections);

/** The section with this id. Every id is in NAV, so this always finds one. */
export function section(id: SectionId): Section {
  const s = ALL.find((x) => x.id === id);
  if (!s) throw new Error(`no section ${id}`);
  return s;
}

/** Whether a staff member holding these permissions may open the section. */
export function canOpen(s: Section, held: readonly Permission[]): boolean {
  return s.needs === null || held.includes(s.needs);
}

/** The navigation as this staff member sees it: no entry they cannot open, no empty group. */
export function visibleNav(held: readonly Permission[]): NavGroup[] {
  return NAV.map((g) => ({ label: g.label, sections: g.sections.filter((s) => canOpen(s, held)) })).filter(
    (g) => g.sections.length > 0,
  );
}

/** The section a path belongs to, or null for a path that is none of them. */
export function sectionForPath(path: string): Section | null {
  const within = (base: string) => base !== "/" && (path === base || path.startsWith(`${base}/`));
  if (path === "/" || path === "") return section("dashboard");
  return ALL.find((s) => within(s.path) || (s.also ?? []).some(within)) ?? null;
}

/** How long the second key of a chord may wait for, in milliseconds. */
export const CHORD_WINDOW_MS = 1500;

/** When `g` was pressed and not yet followed, or null. */
export type ChordState = number | null;

export interface KeyInput {
  readonly key: string;
  /** Ctrl, Alt or Meta held. */
  readonly modified: boolean;
  /** Focus is in an input, select, textarea or anything editable. */
  readonly typing: boolean;
  readonly now: number;
}

export interface ChordResult {
  readonly state: ChordState;
  /** The path to go to, when the key finished a chord. */
  readonly go: string | null;
}

/**
 * One keypress through the `g` chord. A chord for a section the staff member
 * cannot open, or one not built yet, is swallowed and goes nowhere.
 */
export function chord(state: ChordState, input: KeyInput, held: readonly Permission[]): ChordResult {
  if (input.modified || input.typing) return { state: null, go: null };
  const key = input.key.toLowerCase();
  const pending = state !== null && input.now - state <= CHORD_WINDOW_MS;
  if (!pending) return { state: key === "g" ? input.now : null, go: null };

  const target = ALL.find((s) => s.key === key);
  if (!target || !target.built || !canOpen(target, held)) return { state: null, go: null };
  return { state: null, go: target.path };
}
