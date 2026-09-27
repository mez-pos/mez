// What the floor plan editor works out (backoffice-floor.md). The plan is
// the server's; this lays an area out as cells, decides what a drop or a
// tap on a cell means, and turns the form into the request the server
// checks. Cells count from 0 on the wire and from 1 on screen.

import { ApiError } from "../../../shared/api";
import type { FloorResponse, TableResponse, TableSettings, TableShape } from "../../../shared/api-types";

/** One cell of an area's grid, and the table in service standing in it. */
export interface Cell {
  x: number;
  y: number;
  table: TableResponse | null;
}

/** The area's grid, row by row, each cell with its table in service. */
export function cellsOf(plan: FloorResponse, areaId: number): Cell[][] {
  const at = new Map<string, TableResponse>();
  for (const t of plan.tables) {
    if (t.active && t.area_id === areaId) at.set(`${t.x},${t.y}`, t);
  }
  return Array.from({ length: plan.grid.rows }, (_, y) =>
    Array.from({ length: plan.grid.columns }, (_, x) => ({ x, y, table: at.get(`${x},${y}`) ?? null })),
  );
}

/** The retired tables of an area, by name. */
export function retiredIn(plan: FloorResponse, areaId: number): TableResponse[] {
  return plan.tables.filter((t) => !t.active && t.area_id === areaId).sort((a, b) => a.name.localeCompare(b.name));
}

/** The area to show: the one chosen if it still exists, else the first in service, else the first. */
export function areaToShow(plan: FloorResponse, chosen: number | null): number | null {
  if (chosen !== null && plan.areas.some((a) => a.id === chosen)) return chosen;
  return (plan.areas.find((a) => a.active) ?? plan.areas[0])?.id ?? null;
}

/** "Inside, column 3, row 2". */
export function placeText(plan: FloorResponse, t: Pick<TableResponse, "area_id" | "x" | "y">): string {
  const area = plan.areas.find((a) => a.id === t.area_id)?.name ?? "No area";
  return `${area}, column ${t.x + 1}, row ${t.y + 1}`;
}

export function seatsText(seats: number): string {
  return seats === 1 ? "1 seat" : `${seats} seats`;
}

/**
 * What moving a table to a cell comes to: the settings to send, or why not
 * (without asking the server what the screen already knows). Its own cell
 * is no move at all.
 */
export type Move = { kind: "move"; settings: TableSettings } | { kind: "none" } | { kind: "refused"; message: string };

export function moveTo(table: TableResponse, cell: Cell | null, areaId: number): Move {
  if (cell === null) return { kind: "refused", message: `Drop ${table.name} on a free cell.` };
  if (cell.table?.id === table.id) return { kind: "none" };
  if (cell.table !== null) {
    return { kind: "refused", message: `Table ${cell.table.name} is already at column ${cell.x + 1}, row ${cell.y + 1}.` };
  }
  return { kind: "move", settings: { ...settingsOf(table), area_id: areaId, x: cell.x, y: cell.y } };
}

export function settingsOf(t: TableResponse): TableSettings {
  return { area_id: t.area_id, name: t.name, seats: t.seats, shape: t.shape, x: t.x, y: t.y };
}

/** The table form as typed: numbers stay text until saved, and the column and row count from 1. */
export interface TableForm {
  name: string;
  seats: string;
  shape: TableShape;
  areaId: number;
  column: string;
  row: string;
}

export function emptyTableForm(areaId: number, x = 0, y = 0): TableForm {
  return { name: "", seats: "2", shape: "square", areaId, column: String(x + 1), row: String(y + 1) };
}

export function tableFormFrom(t: TableResponse): TableForm {
  return { name: t.name, seats: String(t.seats), shape: t.shape, areaId: t.area_id, column: String(t.x + 1), row: String(t.y + 1) };
}

/**
 * The form as the server's request. A number field that is not a whole
 * number goes as -1, which the server refuses under that field, so every
 * refusal has one wording: the server's.
 */
export function toTableSettings(f: TableForm): TableSettings {
  const whole = (s: string): number => (/^\s*\d+\s*$/.test(s) ? Number(s) : -1);
  const place = (s: string): number => {
    const n = whole(s);
    return n < 1 ? -1 : n - 1;
  };
  return { area_id: f.areaId, name: f.name.trim(), seats: whole(f.seats), shape: f.shape, x: place(f.column), y: place(f.row) };
}

/** A refusal as the forms show it: under its field, or over the form. */
export interface Problem {
  field: "name" | "seats" | "shape" | "area_id" | "x" | "y" | null;
  message: string;
}

const fields = ["name", "seats", "shape", "area_id", "x", "y"] as const;

export function problemOf(err: unknown): Problem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the Nofiltr server. Check this computer's connection." };
    if (err.code === "forbidden") return { field: null, message: "Only someone who can change the venue's settings can do this." };
    const field = (fields as readonly string[]).includes(err.field ?? "") ? (err.field as Problem["field"]) : null;
    return { field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}
