// What the People screen says about staff and roles (backoffice-people.md).
// The rules are the server's (ADR-115); this words them, mirrors the one
// the screen can show ahead (you give only what you hold), and turns the
// forms into the requests the server checks.

import { ApiError } from "../../../shared/api";
import type { Permission, Person, PersonInput, Role, RoleInput } from "../../../shared/api-types";

/** Each permission in words. A Record, so a permission added on the server fails the typecheck here until it has words. */
export const permissionWords: Readonly<Record<Permission, string>> = {
  "order.open": "Take orders",
  "order.edit": "Change orders on the ticket",
  "payment.take": "Take payment",
  "kitchen.view": "Use the kitchen display",
  "line.void": "Void items",
  "line.comp": "Comp items",
  "discount.give": "Give discounts",
  "payment.void": "Void payments",
  "refund.give": "Give refunds",
  "order.reopen": "Reopen closed bills",
  "invoice.reprint": "Reprint invoices",
  "shift.open": "Open the shift",
  "shift.manage": "Close the shift",
  "cash.move": "Move cash in and out of the drawer",
  "menu.edit": "Edit the menu",
  "stock.manage": "Keep the stock",
  "staff.manage": "Manage staff and roles",
  "reports.view": "See reports",
  "settings.edit": "Change venue settings",
  "device.manage": "Pair and remove devices",
};

export interface PermissionGroup {
  title: string;
  note?: string;
  permissions: readonly Permission[];
}

export const permissionGroups: readonly PermissionGroup[] = [
  { title: "Selling", permissions: ["order.open", "order.edit", "payment.take", "kitchen.view"] },
  {
    title: "Corrections",
    note: "Others ask for these at the till.",
    permissions: ["line.void", "line.comp", "discount.give", "payment.void", "refund.give", "order.reopen", "invoice.reprint"],
  },
  { title: "Shift and cash", permissions: ["shift.open", "shift.manage", "cash.move"] },
  {
    title: "Back office",
    permissions: ["menu.edit", "stock.manage", "staff.manage", "reports.view", "settings.edit", "device.manage"],
  },
];

/** Whether someone holding held may give, or reach, a role granting these (ADR-115). */
export function mayGive(held: readonly Permission[], grants: readonly Permission[]): boolean {
  return grants.every((p) => held.includes(p));
}

/** A role's permissions in words, grouped as the form groups them; empty groups left out. */
export function roleWords(grants: readonly Permission[]): { title: string; words: string[] }[] {
  return permissionGroups
    .map((g) => ({ title: g.title, words: g.permissions.filter((p) => grants.includes(p)).map((p) => permissionWords[p]) }))
    .filter((g) => g.words.length > 0);
}

/** A refusal as a form shows it: under its field, or over the form. */
export interface Problem {
  field: string | null;
  message: string;
}

/** Why a PIN typed twice cannot be sent, or null. The server checks it again. */
export function pinProblem(pin: string, again: string): Problem | null {
  if (!/^\d{4,8}$/.test(pin)) return { field: "pin", message: "A PIN is 4 to 8 digits." };
  if (pin !== again) return { field: "again", message: "The two PINs are not the same." };
  return null;
}

export interface StaffForm {
  name: string;
  roleId: number;
  pin: string;
  again: string;
}

export function newStaffForm(roles: readonly Role[], held: readonly Permission[]): StaffForm {
  // The least a new person could start as: the first role you may give, fewest permissions first.
  const giveable = roles.filter((r) => mayGive(held, r.permissions)).sort((a, b) => a.permissions.length - b.permissions.length);
  return { name: "", roleId: giveable[0]?.id ?? 0, pin: "", again: "" };
}

export function staffFormFrom(p: Person): StaffForm {
  return { name: p.name, roleId: p.role_id, pin: "", again: "" };
}

/** The request for a staff form; a PIN only when adding. */
export function toPersonInput(form: StaffForm, adding: boolean): { input: PersonInput } | { problem: Problem } {
  const base: PersonInput = { name: form.name.trim(), role_id: form.roleId };
  if (!adding) return { input: base };
  const problem = pinProblem(form.pin, form.again);
  return problem ? { problem } : { input: { ...base, pin: form.pin } };
}

export interface RoleForm {
  name: string;
  permissions: Permission[];
}

export function roleFormFrom(r: Role | null): RoleForm {
  return { name: r?.name ?? "", permissions: r ? [...r.permissions] : [] };
}

/** The role form with one permission ticked or not, in the canonical order. */
export function toggled(form: RoleForm, p: Permission, on: boolean): RoleForm {
  const set = new Set(form.permissions);
  if (on) set.add(p);
  else set.delete(p);
  const order = permissionGroups.flatMap((g) => g.permissions);
  return { ...form, permissions: order.filter((x) => set.has(x)) };
}

export function toRoleInput(form: RoleForm): RoleInput {
  return { name: form.name.trim(), permissions: form.permissions };
}

export function problemOf(err: unknown): Problem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the mez server. Check this computer's connection." };
    if (err.code === "forbidden") return { field: null, message: "Only someone who can manage staff can do this." };
    return { field: err.field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}
