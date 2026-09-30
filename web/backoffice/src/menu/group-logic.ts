// What the menu editor says about modifier groups (backoffice-menu.md,
// "Choices"). The rules are the server's (internal/domain/menu/group.go);
// this words them and turns the group form into the request it checks.

import { ApiError } from "../../../shared/api";
import type { EditorGroup, EditorItem, MenuEditor, MenuGroupInput, MenuModifierInput } from "../../../shared/api-types";
import { amountInputText, parseAmount } from "../../../shared/money";

/** How many a group asks for, in words: "Pick 1", "Pick up to 2", "Pick 1 to 3". */
export function ruleText(g: Pick<EditorGroup, "min_select" | "max_select">): string {
  const { min_select: min, max_select: max } = g;
  if (min === max) return `Pick ${min}`;
  if (min === 0) return `Optional, up to ${max}`;
  return `Pick ${min} to ${max}`;
}

/** How many items offer a group, retired items included. */
export function usedBy(menu: MenuEditor, groupId: number): number {
  return menu.items.filter((i) => i.modifier_groups.includes(groupId)).length;
}

/** The groups an item's form may add: active ones it does not offer yet. */
export function addableGroups(menu: MenuEditor, chosen: readonly number[]): EditorGroup[] {
  return menu.modifier_groups.filter((g) => g.active && !chosen.includes(g.id));
}

/** A group by id, or null if it is not on the menu. */
export function groupById(menu: MenuEditor, id: number): EditorGroup | null {
  return menu.modifier_groups.find((g) => g.id === id) ?? null;
}

/** Whether an item's groups changed, order included. */
export function groupsChanged(item: EditorItem | null, chosen: readonly number[]): boolean {
  const before = item?.modifier_groups ?? [];
  return before.length !== chosen.length || before.some((id, i) => id !== chosen[i]);
}

/**
 * A choice's price as typed: it changes the line's price, so it may be
 * negative ("-20" for no cheese) and empty means no change. Null when it
 * cannot be read. Minor units, never through a float.
 */
export function parseChoicePrice(text: string, currency: string): number | null {
  const t = text.trim();
  if (t === "") return 0;
  const negative = t.startsWith("-") || t.startsWith("−");
  const minor = parseAmount(negative ? t.slice(1) : t, currency);
  if (minor === null) return null;
  return negative ? -minor : minor;
}

export interface ChoiceForm {
  /** Stable across edits, for React's list: the id, or a new row's own key. */
  key: string;
  /** Zero for a choice not saved yet. */
  id: number;
  name: string;
  price: string;
  isDefault: boolean;
}

export interface GroupForm {
  name: string;
  min: string;
  max: string;
  choices: ChoiceForm[];
}

let newRows = 0;

/** A blank choice row. */
export function newChoice(): ChoiceForm {
  newRows += 1;
  return { key: `new-${newRows}`, id: 0, name: "", price: "", isDefault: false };
}

export function emptyGroupForm(): GroupForm {
  return { name: "", min: "0", max: "1", choices: [newChoice()] };
}

export function groupFormFrom(g: EditorGroup): GroupForm {
  return {
    name: g.name,
    min: String(g.min_select),
    max: String(g.max_select),
    choices: g.modifiers.map((m) => ({
      key: `id-${m.id}`,
      id: m.id,
      name: m.name,
      price: m.price.minor === 0 ? "" : amountInputText(m.price),
      isDefault: m.is_default,
    })),
  };
}

/** A refusal as the group form shows it: a field, a choice row's field, or over the form. */
export interface GroupProblem {
  field: "name" | "min_select" | "max_select" | "modifiers" | null;
  /** The choice row it is about, and which of its fields. */
  row?: { index: number; field: "name" | "price" | "id" };
  message: string;
}

const count = /^\d+$/;

/** The request for a group form, or the problem that stops it being sent. */
export function toGroupInput(form: GroupForm, currency: string): { input: MenuGroupInput } | { problem: GroupProblem } {
  if (!count.test(form.min.trim())) return { problem: { field: "min_select", message: "Type a whole number, 0 or more." } };
  if (!count.test(form.max.trim())) return { problem: { field: "max_select", message: "Type a whole number, 1 or more." } };
  const modifiers: MenuModifierInput[] = [];
  for (const [index, c] of form.choices.entries()) {
    const minor = parseChoicePrice(c.price, currency);
    if (minor === null) {
      return { problem: { field: null, row: { index, field: "price" }, message: "Type a price like 40, -20 or leave it empty." } };
    }
    const m: MenuModifierInput = { name: c.name.trim(), price: { minor, currency }, is_default: c.isDefault };
    modifiers.push(c.id ? { ...m, id: c.id } : m);
  }
  return {
    input: { name: form.name.trim(), min_select: Number(form.min.trim()), max_select: Number(form.max.trim()), modifiers },
  };
}

const rowField = /^modifiers\.(\d+)\.(name|price|id)$/;
const groupFields: readonly string[] = ["name", "min_select", "max_select", "modifiers"];

export function groupProblemOf(err: unknown): GroupProblem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the Nofiltr server. Check this computer's connection." };
    if (err.code === "forbidden") return { field: null, message: "Only someone who can edit the menu can do this." };
    const row = rowField.exec(err.field ?? "");
    if (row) {
      return { field: null, row: { index: Number(row[1]), field: row[2] as "name" | "price" | "id" }, message: err.message };
    }
    const field = groupFields.includes(err.field ?? "") ? (err.field as GroupProblem["field"]) : null;
    return { field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}
