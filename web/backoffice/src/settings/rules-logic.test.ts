import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { RulesResponse } from "../../../shared/api-types";
import { DEFAULT_TIP_MAX_PERCENT, TIP_RULES, rulesChanged, rulesFormFrom, rulesProblem, rulesProblemOf, toRulesInput, type RulesForm } from "./rules-logic";

const saved: RulesResponse = {
  day_start: "04:00",
  variance_threshold: 0,
  tip_rule: "even",
  tip_role_weights: {},
  ticket_late_minutes: 20,
  tips_max_percent: 50,
  currency: "INR",
  minor_per_major: 100,
  roles: ["cashier", "manager", "owner"],
  open_shift_day: "",
};
const per = 100;
const form = (patch: Partial<RulesForm> = {}): RulesForm => ({ ...rulesFormFrom(saved), ...patch });

describe("the rules form", () => {
  it("is what the server holds, the threshold in major units", () => {
    expect(rulesFormFrom(saved)).toEqual({
      dayStart: "04:00",
      variance: "0.00",
      tipRule: "even",
      weights: { cashier: "", manager: "", owner: "" },
      tipMax: "50",
      lateMinutes: "20",
    });
    expect(rulesFormFrom({ ...saved, variance_threshold: 5050 }).variance).toBe("50.50");
  });
  it("lists a weight for each role the venue has, and leaves behind one for a role it no longer has", () => {
    const f = rulesFormFrom({ ...saved, tip_rule: "role_weight", tip_role_weights: { manager: 3, barista: 2 } });
    expect(f.weights).toEqual({ cashier: "", manager: "3", owner: "" });
  });
  it("shows the tip ceiling as saved, 0 included, and the default when the server leaves it out", () => {
    expect(rulesFormFrom({ ...saved, tips_max_percent: 0 }).tipMax).toBe("0");
    expect(rulesFormFrom({ ...saved, tips_max_percent: 1000 }).tipMax).toBe("1000");
    const { tips_max_percent: _left, ...older } = saved;
    expect(rulesFormFrom(older).tipMax).toBe(String(DEFAULT_TIP_MAX_PERCENT));
  });
  it("starts from even when the server names a rule it does not know", () => {
    expect(rulesFormFrom({ ...saved, tip_rule: "coin_toss" }).tipRule).toBe("even");
  });
  it("offers the three ways to share tips", () => {
    expect(TIP_RULES.map((r) => r.value)).toEqual(["even", "hours", "role_weight"]);
  });

  it("sends the threshold in minor units and only the weights that were given", () => {
    const f = form({ variance: "12.5", tipRule: "role_weight", weights: { cashier: "1", manager: " 3 ", owner: "" }, tipMax: " 0 ", lateMinutes: " 15 " });
    expect(toRulesInput(f, per)).toEqual({
      day_start: "04:00",
      variance_threshold: 1250,
      tip_rule: "role_weight",
      tip_role_weights: { cashier: 1, manager: 3 },
      tips_max_percent: 0,
      ticket_late_minutes: 15,
    });
  });

  it("is changed by a real difference, not by stray spaces or a weight written another way", () => {
    const base = form();
    expect(rulesChanged(base, base, per)).toBe(false);
    expect(rulesChanged(form({ variance: "0" }), base, per)).toBe(false); // 0 and 0.00 are the same amount
    expect(rulesChanged(form({ lateMinutes: " 20 " }), base, per)).toBe(false);
    expect(rulesChanged(form({ dayStart: "05:00" }), base, per)).toBe(true);
    expect(rulesChanged(form({ variance: "1" }), base, per)).toBe(true);
    expect(rulesChanged(form({ tipRule: "hours" }), base, per)).toBe(true);
    expect(rulesChanged(form({ lateMinutes: "25" }), base, per)).toBe(true);
    expect(rulesChanged(form({ tipMax: " 50 " }), base, per)).toBe(false);
    expect(rulesChanged(form({ tipMax: "0" }), base, per)).toBe(true);
    expect(rulesChanged(form({ weights: { cashier: "2", manager: "", owner: "" } }), base, per)).toBe(true);
    expect(rulesChanged(form({ weights: { cashier: "  ", manager: "", owner: "" } }), base, per)).toBe(false);
  });
});

describe("what is refused before asking the server", () => {
  it("lets a good form through", () => {
    expect(rulesProblem(form(), per)).toBeNull();
    expect(rulesProblem(form({ tipRule: "role_weight", weights: { cashier: "1", manager: "2", owner: "" } }), per)).toBeNull();
    expect(rulesProblem(form({ dayStart: "00:00", variance: "1000000", lateMinutes: "600" }), per)).toBeNull();
    expect(rulesProblem(form({ lateMinutes: "1" }), per)).toBeNull();
  });
  it("wants a time of day", () => {
    for (const bad of ["", "4:00", "24:00", "12:60", "dawn", "04:00:00"]) {
      expect(rulesProblem(form({ dayStart: bad }), per)?.field, bad).toBe("day_start");
    }
  });
  it("wants an amount from nothing to a million", () => {
    for (const bad of ["", "abc", "-1", "1.234", "1000000.01"]) {
      expect(rulesProblem(form({ variance: bad }), per)?.field, bad).toBe("variance_threshold");
    }
    expect(rulesProblem(form({ variance: "1000000.01" }), per)?.message).toBe("Enter an amount from 0 to 1000000.00.");
    // Yen has no decimals, and its limit is the same count of minor units.
    expect(rulesProblem(form({ variance: "100000000" }), 1)).toBeNull();
    expect(rulesProblem(form({ variance: "100000001" }), 1)?.field).toBe("variance_threshold");
  });
  it("wants a weight from 1 to 100, naming the role", () => {
    for (const bad of ["0", "101", "1.5", "-2", "abc"]) {
      const p = rulesProblem(form({ weights: { cashier: bad, manager: "", owner: "" } }), per);
      expect(p?.field, bad).toBe("tip_role_weights");
      expect(p?.message).toContain("cashier");
    }
    expect(rulesProblem(form({ weights: { cashier: "100", manager: "1", owner: "" } }), per)).toBeNull();
  });
  it("wants a weight for someone when tips are shared by role", () => {
    expect(rulesProblem(form({ tipRule: "role_weight" }), per)?.field).toBe("tip_role_weights");
    // Weights kept while tips are shared another way are fine, and so are none.
    expect(rulesProblem(form({ tipRule: "hours" }), per)).toBeNull();
  });
  it("wants a whole percent from 0 to 1000 for the tip ceiling", () => {
    for (const ok of ["0", "50", " 75 ", "1000"]) {
      expect(rulesProblem(form({ tipMax: ok }), per), ok).toBeNull();
    }
    for (const bad of ["", "-1", "1001", "12.5", "abc", "50%"]) {
      expect(rulesProblem(form({ tipMax: bad }), per)?.field, bad).toBe("tips_max_percent");
    }
    expect(rulesProblem(form({ tipMax: "1001" }), per)?.message).toBe("Enter a whole percent from 0 to 1000. 0 asks for a manager on every tip.");
  });
  it("wants whole minutes from 1 to 600", () => {
    for (const bad of ["", "0", "601", "1.5", "abc", "-5"]) {
      expect(rulesProblem(form({ lateMinutes: bad }), per)?.field, bad).toBe("ticket_late_minutes");
    }
  });
  it("reads the form top to bottom", () => {
    expect(rulesProblem(form({ dayStart: "x", variance: "x", lateMinutes: "x" }), per)?.field).toBe("day_start");
    expect(rulesProblem(form({ tipMax: "x", lateMinutes: "x" }), per)?.field).toBe("tips_max_percent");
  });
});

describe("a failed request", () => {
  it("puts the server's words under the field they are about", () => {
    expect(rulesProblemOf(new ApiError("invalid_setting", "Chef is not a role here.", 422, "tip_role_weights"))).toEqual({
      field: "tip_role_weights",
      message: "Chef is not a role here.",
    });
    expect(rulesProblemOf(new ApiError("invalid_setting", "x", 422, "day_start")).field).toBe("day_start");
    expect(rulesProblemOf(new ApiError("invalid_setting", "x", 422, "tips_max_percent")).field).toBe("tips_max_percent");
    expect(rulesProblemOf(new ApiError("invalid_setting", "x", 422, "currency")).field).toBeNull();
    expect(rulesProblemOf(new ApiError("network", "down")).message).toMatch(/Can't reach/);
  });
});
