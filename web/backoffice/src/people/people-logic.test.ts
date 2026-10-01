import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { Permission, Role } from "../../../shared/api-types";
import {
  mayGive,
  newStaffForm,
  permissionGroups,
  permissionWords,
  pinProblem,
  problemOf,
  roleFormFrom,
  roleWords,
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
  it("word a role by group, leaving empty groups out", () => {
    expect(roleWords(["payment.take", "order.open", "reports.view"])).toEqual([
      { title: "Selling", words: ["Take orders", "Take payment"] },
      { title: "Back office", words: ["See reports"] },
    ]);
    expect(roleWords([])).toEqual([]);
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
    expect(staffFormFrom({ id: "stf_1", name: "Asha", role_id: 3, role_name: "cashier", active: true, since: "x" })).toEqual({
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
