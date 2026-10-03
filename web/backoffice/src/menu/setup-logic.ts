// The Stations and courses view of the menu editor (docs/design/screens/backoffice-menu.md):
// the rows each list shows, what retiring one asks and says, and how a refusal
// reads. A station is where an item is made, a course when it is fired.
// The server checks everything again (ADR-134); what is checked here costs
// nothing. No React in this file, so all of it is tested directly.

import type { CourseRetiredResponse, MenuEditor, StationRetiredResponse } from "../../../shared/api-types";
import { problemWith, type Problem } from "../settings/problem";

/** A station or a course as a row of its list. */
export interface SetupRow {
  id: number;
  name: string;
  /** What names it beside the name: a station's code, a course's number. */
  tag: string;
  active: boolean;
  /** How many items name it, retired items included: what retiring it moves. */
  items: number;
}

export type SetupKind = "station" | "course";

export function stationRows(menu: MenuEditor): SetupRow[] {
  return menu.stations.map((s) => ({
    id: s.id,
    name: s.name,
    tag: s.code,
    active: s.active,
    items: menu.items.filter((i) => i.station_id === s.id).length,
  }));
}

export function courseRows(menu: MenuEditor): SetupRow[] {
  return menu.courses.map((c) => ({
    id: c.id,
    name: c.name,
    tag: `Course ${c.number}`,
    active: c.active,
    items: menu.items.filter((i) => i.course_id === c.id).length,
  }));
}

/** The rows an item can move to when one retires: in service, and not itself. */
export function destinations(rows: readonly SetupRow[], leaving: number): SetupRow[] {
  return rows.filter((r) => r.active && r.id !== leaving);
}

/** "no items", "1 item", "7 items". */
export function itemsText(n: number): string {
  return n === 0 ? "No items" : n === 1 ? "1 item" : `${n} items`;
}

/** What can be refused before asking the server: items with nowhere to go. */
export function retireProblem(row: SetupRow, moveTo: number, kind: SetupKind): Problem<"move_to"> | null {
  if (row.items > 0 && moveTo === 0) {
    return { field: "move_to", message: `Choose the ${kind} its ${row.items === 1 ? "item goes" : "items go"} to.` };
  }
  return null;
}

/** What retiring a station did, in a sentence for a toast. */
export function stationRetiredText(name: string, to: { name: string } | null, r: StationRetiredResponse): string {
  const parts = [`${name} retired.`];
  if (to && r.items_moved > 0) parts.push(`${itemsText(r.items_moved)} moved to ${to.name}.`);
  if (to && r.printer_moved) parts.push(`Its printer now serves ${to.name}.`);
  return parts.join(" ");
}

export function courseRetiredText(name: string, to: { name: string } | null, r: CourseRetiredResponse): string {
  const parts = [`${name} retired.`];
  if (to && r.items_moved > 0) parts.push(`${itemsText(r.items_moved)} moved to ${to.name}.`);
  return parts.join(" ");
}

export type SetupField = "name" | "code" | "move_to";
const setupFields: readonly SetupField[] = ["name", "code", "move_to"];

/** A failed request as words, under the field of the form it is about. */
export const setupProblemOf = (err: unknown): Problem<SetupField> => problemWith(err, setupFields);

export const MAX_NAME = 60;

/** What can be refused in a name before asking: none given, or too long. */
export function nameProblem(name: string): Problem<"name"> | null {
  const n = name.trim();
  if (n === "") return { field: "name", message: "Give it a name." };
  if ([...n].length > MAX_NAME) return { field: "name", message: `Keep the name to ${MAX_NAME} characters.` };
  return null;
}
