import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { Permission, Person, Role } from "../../../shared/api-types";
import {
  appliesText,
  chosenRole,
  lockedNote,
  mayGive,
  mayUnlock,
  minPinLength,
  pinHint,
  roleChangePinHint,
  staffStateOf,
  peopleSummary,
  roleChanged,
  staffRows,
  newStaffForm,
  permissionGroups,
  permissionWords,
  pinProblem,
  problemOf,
  roleFormFrom,
  staffFormFrom,
  toggled,
  toPersonInput,
  toRoleInput,
} from "./people-logic";

const manager: Permission[] = ["order.open", "payment.take", "line.void", "staff.manage", "reports.view"];
const roles: Role[] = [
  { id: 1, name: "owner", permissions: Object.keys(permissionWords) as Permission[], people: 1 },
  { id: 2, name: "manager", permissions: manager, people: 1 },
  { id: 3, name: "cashier", permissions: ["order.open", "payment.take"], people: 2 },
  { id: 4, name: "kitchen", permissions: ["kitchen.view"], people: 0 },
];

describe("the permission words", () => {
  it("put every permission in exactly one group", () => {
    const grouped = permissionGroups.flatMap((g) => g.permissions);
    expect(new Set(grouped).size).toBe(grouped.length);
    expect([...grouped].sort()).toEqual(Object.keys(permissionWords).sort());
  });
});

describe("mayGive", () => {
  it("allows only what is held", () => {
    expect(mayGive(manager, ["order.open"])).toBe(true);
    expect(mayGive(manager, manager)).toBe(true);
    expect(mayGive(manager, ["order.open", "settings.edit"])).toBe(false);
    expect(mayGive(manager, [])).toBe(true);
  });
});

describe("pinProblem", () => {
  it("wants 4 to 8 digits, typed the same twice", () => {
    expect(pinProblem("1357", "1357")).toBeNull();
    expect(pinProblem("12345678", "12345678")).toBeNull();
    expect(pinProblem("123", "123")).toMatchObject({ field: "pin" });
    expect(pinProblem("123456789", "123456789")).toMatchObject({ field: "pin" });
    expect(pinProblem("12a4", "12a4")).toMatchObject({ field: "pin" });
    expect(pinProblem("1357", "1375")).toMatchObject({ field: "again" });
  });
});

describe("the staff form", () => {
  it("starts a new person on the smallest role you may give", () => {
    expect(newStaffForm(roles, manager).roleId).toBe(3);
    expect(newStaffForm(roles, ["order.open"]).roleId).toBe(0);
  });
  it("sends a PIN only when adding", () => {
    const form = { ...newStaffForm(roles, manager), name: " Kiran ", pin: "1357", again: "1357" };
    expect(toPersonInput(form, true)).toEqual({ input: { name: "Kiran", role_id: 3, pin: "1357" } });
    expect(toPersonInput(form, false)).toEqual({ input: { name: "Kiran", role_id: 3 } });
    expect(toPersonInput({ ...form, again: "" }, true)).toMatchObject({ problem: { field: "again" } });
  });
  it("reads a person in", () => {
    expect(staffFormFrom({ id: "stf_1", name: "Asha", role_id: 3, role_name: "cashier", active: true, since: "x", locked_at: null, cloud: false })).toEqual({
      name: "Asha",
      roleId: 3,
      pin: "",
      again: "",
    });
  });
});

describe("the role form", () => {
  it("keeps permissions in the canonical order however they are ticked", () => {
    let form = roleFormFrom(null);
    form = toggled(form, "reports.view", true);
    form = toggled(form, "order.open", true);
    form = toggled(form, "reports.view", true);
    expect(form.permissions).toEqual(["order.open", "reports.view"]);
    form = toggled(form, "order.open", false);
    expect(toRoleInput({ ...form, name: " Barista " })).toEqual({ name: "Barista", permissions: ["reports.view"] });
  });
  it("reads a role in without sharing its list", () => {
    const form = roleFormFrom(roles[2]!);
    form.permissions.push("line.void");
    expect(roles[2]!.permissions).toEqual(["order.open", "payment.take"]);
  });
});

describe("problemOf", () => {
  it("keeps the server's words and field", () => {
    expect(problemOf(new ApiError("role_name_taken", "Taken.", 409, "name"))).toEqual({ field: "name", message: "Taken." });
    expect(problemOf(new ApiError("beyond_your_permissions", "Beyond.", 403))).toEqual({ field: null, message: "Beyond." });
    expect(problemOf(new ApiError("forbidden", "x", 403, "staff.manage")).message).toMatch(/manage staff/);
    expect(problemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});

describe("the staff and roles screen", () => {
  const person = (id: string, name: string, role_id: number, active = true): Person => ({
    id,
    name,
    role_id,
    role_name: "",
    active,
    since: "2026-09-01T00:00:00Z",
    locked_at: null,
    cloud: false,
  });
  const staff = [person("a", "Priya", 1), person("b", "Kabir", 2), person("c", "Rohan", 2), person("d", "Old Sam", 2, false)];

  it("counts the people, and the deactivated apart", () => {
    expect(peopleSummary(staff)).toBe("3 people · 1 deactivated");
    expect(peopleSummary(staff.slice(0, 1))).toBe("1 person");
    expect(peopleSummary([])).toBe("0 people");
  });
  it("lists the active staff, and the deactivated last when asked", () => {
    expect(staffRows(staff, false).map((p) => p.name)).toEqual(["Priya", "Kabir", "Rohan"]);
    expect(staffRows(staff, true).map((p) => p.name)).toEqual(["Priya", "Kabir", "Rohan", "Old Sam"]);
  });
  it("says who a role applies to, from the active staff", () => {
    expect(appliesText(staff, { id: 2, people: 3 })).toBe("Applies to Kabir and Rohan.");
    expect(appliesText(staff, { id: 1, people: 1 })).toBe("Applies to Priya.");
    expect(appliesText(staff, { id: 9, people: 0 })).toBe("Nobody has this role yet.");
    expect(appliesText([person("z", "Old Sam", 9, false)], { id: 9, people: 1 })).toBe("Held only by deactivated staff, so it cannot be deleted.");
    const many = ["A", "B", "C", "D", "E"].map((n) => person(n, n, 3));
    expect(appliesText(many.slice(0, 3), { id: 3, people: 3 })).toBe("Applies to A, B and C.");
    expect(appliesText(many, { id: 3, people: 5 })).toBe("Applies to A, B, C and 2 more.");
  });
  it("knows when a role's form differs from what is saved", () => {
    const role: Role = { id: 2, name: "Waiter", permissions: ["order.open", "order.edit"], people: 2 };
    expect(roleChanged(role, { name: "Waiter", permissions: ["order.edit", "order.open"] })).toBe(false);
    expect(roleChanged(role, { name: " Waiter ", permissions: ["order.open", "order.edit"] })).toBe(false);
    expect(roleChanged(role, { name: "Server", permissions: ["order.open", "order.edit"] })).toBe(true);
    expect(roleChanged(role, { name: "Waiter", permissions: ["order.open"] })).toBe(true);
    expect(roleChanged(role, { name: "Waiter", permissions: ["order.open", "order.edit", "payment.take"] })).toBe(true);
  });
  it("keeps the role asked for, falls back to the first, and to none", () => {
    const roles: Role[] = [
      { id: 1, name: "Owner", permissions: [], people: 1 },
      { id: 2, name: "Waiter", permissions: [], people: 2 },
    ];
    expect(chosenRole(roles, 2)?.name).toBe("Waiter");
    expect(chosenRole(roles, 99)?.name).toBe("Owner");
    expect(chosenRole(roles, null)?.name).toBe("Owner");
    expect(chosenRole([], 1)).toBeNull();
  });
});

describe("a PIN long enough for the role (ADR-260)", () => {
  const short = "A PIN for a role that manages staff, devices or settings is 6 to 8 digits.";
  it("wants six digits for a role that manages staff, devices or settings", () => {
    expect(minPinLength(manager)).toBe(6);
    expect(minPinLength(["device.manage"])).toBe(6);
    expect(minPinLength(["settings.edit", "order.open"])).toBe(6);
    expect(minPinLength(["order.open", "payment.take", "reports.view", "menu.edit"])).toBe(4);
    expect(minPinLength([])).toBe(4);
  });
  it("words the shorter PIN apart from one of the wrong form", () => {
    expect(pinProblem("1357", "1357", 6)).toEqual({ field: "pin", message: short });
    expect(pinProblem("135791", "135791", 6)).toBeNull();
    expect(pinProblem("12a4", "12a4", 6)).toEqual({ field: "pin", message: "A PIN is 4 to 8 digits." });
    expect(pinHint(4)).toBe("4 to 8 digits.");
    expect(pinHint(6)).toBe("6 to 8 digits: this role manages staff, devices or settings.");
  });
  it("checks a new person's PIN against the role chosen", () => {
    const form = { name: "Rohan", roleId: 2, pin: "1357", again: "1357" };
    expect(toPersonInput(form, true, roles)).toEqual({ problem: { field: "pin", message: short } });
    expect(toPersonInput({ ...form, roleId: 3 }, true, roles)).toEqual({ input: { name: "Rohan", role_id: 3, pin: "1357" } });
    expect(toPersonInput({ ...form, pin: "135791", again: "135791" }, true, roles)).toMatchObject({ input: { pin: "135791" } });
  });
  it("asks for a longer PIN when someone moves to a role that needs one", () => {
    expect(roleChangePinHint(roles, 3, 2)).toBe("This role needs a PIN of 6 digits or more. Ask them to set a 6-digit PIN.");
    expect(roleChangePinHint(roles, 2, 1)).toBeNull();
    expect(roleChangePinHint(roles, 2, 3)).toBeNull();
    expect(roleChangePinHint(roles, 3, 3)).toBeNull();
    expect(roleChangePinHint(roles, 3, 99)).toBeNull();
  });
});

describe("a hard lock (ADR-260)", () => {
  const now = new Date("2026-10-03T10:00:00Z");
  const base: Person = { id: "a", name: "Asha", role_id: 3, role_name: "cashier", active: true, since: "2026-09-01T00:00:00Z", locked_at: null, cloud: false };
  const locked = (at: string, extra: Partial<Person> = {}): Person => ({ ...base, locked_at: at, ...extra });

  it("puts the lock in the state chip, after a deactivation", () => {
    expect(staffStateOf(base)).toBe("active");
    expect(staffStateOf(locked("2026-10-03T09:48:00Z"))).toBe("locked");
    expect(staffStateOf(locked("2026-10-03T09:48:00Z", { active: false }))).toBe("off");
    expect(staffStateOf({ ...base, active: false })).toBe("off");
  });
  it("says why and when", () => {
    expect(lockedNote(base, now)).toBeNull();
    expect(lockedNote(locked("2026-10-03T09:59:40Z"), now)).toBe("Too many wrong PINs, just now");
    expect(lockedNote(locked("2026-10-03T09:48:00Z"), now)).toBe("Too many wrong PINs, 12 min ago");
    expect(lockedNote(locked("2026-10-03T07:00:00Z"), now)).toBe("Too many wrong PINs, 3 h ago");
    expect(lockedNote(locked("2026-10-01T12:00:00Z"), now)).toBe("Too many wrong PINs, on 1 Oct 2026");
  });
  it("offers Unlock for someone else who is locked", () => {
    expect(mayUnlock(locked("2026-10-03T09:48:00Z"), "b")).toBe(true);
    expect(mayUnlock(locked("2026-10-03T09:48:00Z"), "a")).toBe(false);
    expect(mayUnlock(base, "b")).toBe(false);
    expect(mayUnlock(locked("2026-10-03T09:48:00Z", { active: false }), "b")).toBe(true);
  });
});
