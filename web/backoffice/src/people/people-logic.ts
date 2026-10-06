// What the People screen says about staff and roles (backoffice-people.md).
// The rules are the server's (ADR-115); this words them, mirrors the one
// the screen can show ahead (you give only what you hold), and turns the
// forms into the requests the server checks.

import { ApiError } from "../../../shared/api";
import type { Permission, Person, PersonInput, Role, RoleInput } from "../../../shared/api-types";
import type { StaffState } from "../../../shared/ui/status";
import { dateText } from "../devices/devices-logic";
import { agoText } from "../printers/printers-logic";

/** Each permission in words. A Record, so a permission added on the server fails the typecheck here until it has words. */
export const permissionWords: Readonly<Record<Permission, string>> = {
  "order.open": "Take orders",
  "order.edit": "Change what is on an order",
  "payment.take": "Take payment",
  "kitchen.view": "See the kitchen board",
  "kitchen.work": "Start tickets and mark them ready",
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
  { title: "Kitchen", note: "A kitchen display on the wall needs neither: it is paired, not signed in.", permissions: ["kitchen.work"] },
  {
    title: "Corrections",
    note: "Others ask for these on the order screen.",
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

/** A refusal as a form shows it: under its field, or over the form. */
export interface Problem {
  field: string | null;
  message: string;
}

/** What makes a role's PIN need six digits (ADR-260): the server's list, in internal/auth/pin.go. */
const POWERFUL: readonly Permission[] = ["staff.manage", "device.manage", "settings.edit"];

/** The shortest PIN a role allows: six for one that manages staff, devices or settings, else four. */
export function minPinLength(perms: readonly Permission[]): number {
  return perms.some((p) => POWERFUL.includes(p)) ? 6 : 4;
}

/** The hint under a PIN field, for a role's shortest PIN. */
export function pinHint(min: number): string {
  return min > 4 ? `${min} to 8 digits: this role manages staff, devices or settings.` : "4 to 8 digits.";
}

/** Why a PIN typed twice cannot be sent, or null. The server checks it again. */
export function pinProblem(pin: string, again: string, min = 4): Problem | null {
  if (!/^\d{4,8}$/.test(pin)) return { field: "pin", message: "A PIN is 4 to 8 digits." };
  if (pin.length < min) return { field: "pin", message: `A PIN for a role that manages staff, devices or settings is ${min} to 8 digits.` };
  if (pin !== again) return { field: "again", message: "The two PINs are not the same." };
  return null;
}

/** The shortest PIN for a role, by its id; four for one not listed. */
export function minPinLengthOf(roles: readonly Role[], roleId: number): number {
  return minPinLength(roles.find((r) => r.id === roleId)?.permissions ?? []);
}

/**
 * The hint when a person moves to a role that needs a longer PIN than the
 * one they had, or null. Their PIN is kept as a hash, so how long it is
 * cannot be known: they are asked to set a new one (ADR-260).
 */
export function roleChangePinHint(roles: readonly Role[], fromRoleId: number, toRoleId: number): string | null {
  const to = minPinLengthOf(roles, toRoleId);
  if (to <= minPinLengthOf(roles, fromRoleId)) return null;
  return `This role needs a PIN of ${to} digits or more. Ask them to set a ${to}-digit PIN.`;
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

/** The request for a staff form; a PIN only when adding, long enough for the role chosen. */
export function toPersonInput(form: StaffForm, adding: boolean, roles: readonly Role[] = []): { input: PersonInput } | { problem: Problem } {
  const base: PersonInput = { name: form.name.trim(), role_id: form.roleId };
  if (!adding) return { input: base };
  const problem = pinProblem(form.pin, form.again, minPinLengthOf(roles, form.roleId));
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

/** The line under the title: "9 people", or "9 people Â· 2 deactivated". */
export function peopleSummary(staff: readonly Pick<Person, "active">[]): string {
  const active = staff.filter((p) => p.active).length;
  const off = staff.length - active;
  const people = active === 1 ? "1 person" : `${active} people`;
  return off > 0 ? `${people} Â· ${off} deactivated` : people;
}

/** The table's rows: the active staff in the server's order, then the deactivated when asked for. */
export function staffRows(staff: readonly Person[], showOff: boolean): Person[] {
  const active = staff.filter((p) => p.active);
  return showOff ? [...active, ...staff.filter((p) => !p.active)] : active;
}

/**
 * Who a role applies to: "Applies to Rohan and Arjun.", from the active staff
 * who hold it. A role only deactivated people hold says so: it cannot be
 * deleted, and the table hides them until asked.
 */
export function appliesText(staff: readonly Person[], role: Pick<Role, "id" | "people">): string {
  const names = staff.filter((p) => p.active && p.role_id === role.id).map((p) => p.name);
  if (names.length === 0) return role.people > 0 ? "Held only by deactivated staff, so it cannot be deleted." : "Nobody has this role yet.";
  if (names.length === 1) return `Applies to ${names[0]}.`;
  if (names.length <= 4) return `Applies to ${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}.`;
  return `Applies to ${names.slice(0, 3).join(", ")} and ${names.length - 3} more.`;
}

/** Whether the form differs from the saved role: its name, or what it grants, in any order. */
export function roleChanged(role: Role, form: RoleForm): boolean {
  if (form.name.trim() !== role.name) return true;
  const saved = new Set(role.permissions);
  return form.permissions.length !== saved.size || form.permissions.some((p) => !saved.has(p));
}

/** The role to show: the one asked for if it still exists, else the first. */
export function chosenRole(roles: readonly Role[], wanted: number | null): Role | null {
  return roles.find((r) => r.id === wanted) ?? roles[0] ?? null;
}

/** A person's chip: deactivated first, then a hard lock (ADR-260), else active. */
export function staffStateOf(p: Pick<Person, "active" | "locked_at">): StaffState {
  if (!p.active) return "off";
  return p.locked_at ? "locked" : "active";
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** Under a locked person's chip: why, and when. Null for someone not locked. */
export function lockedNote(p: Pick<Person, "locked_at">, now: Date): string | null {
  if (!p.locked_at) return null;
  const when = now.getTime() - Date.parse(p.locked_at) >= DAY_MS ? `on ${dateText(p.locked_at)}` : agoText(p.locked_at, now);
  return `Too many wrong PINs, ${when}`;
}

/** Whether a row offers Unlock: someone locked, and not you. The server checks you may reach them. */
export function mayUnlock(p: Pick<Person, "id" | "locked_at">, me: string): boolean {
  return p.locked_at !== null && p.id !== me;
}
