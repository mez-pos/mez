import { describe, expect, it } from "vitest";
import { holdDraft, leaveText, noDrafts, opensElsewhere } from "./leave-logic";

const receipt = Symbol("receipt");
const coffee = Symbol("coffee");

describe("holdDraft", () => {
  it("adds a draft, and lets it go, without changing the drafts it was given", () => {
    const one = holdDraft(noDrafts, receipt, "the receipt");
    expect([...one.values()]).toEqual(["the receipt"]);
    expect(noDrafts.size).toBe(0);
    const none = holdDraft(one, receipt, null);
    expect(none.size).toBe(0);
    expect(one.size).toBe(1);
  });

  it("answers the same drafts when nothing changed, so a form saying it again re-renders nothing", () => {
    const one = holdDraft(noDrafts, receipt, "the receipt");
    expect(holdDraft(one, receipt, "the receipt")).toBe(one);
    expect(holdDraft(one, coffee, null)).toBe(one);
  });

  it("renames a draft whose words changed", () => {
    const one = holdDraft(noDrafts, coffee, "Filter coffee");
    expect([...holdDraft(one, coffee, "Filter coffee, large").values()]).toEqual(["Filter coffee, large"]);
  });
});

describe("leaveText", () => {
  it("names what would be lost, each once (BO-2, BO-5)", () => {
    const one = holdDraft(noDrafts, receipt, "Venue");
    expect(leaveText(one)).toBe("Your changes to Venue are not saved.");
    const two = holdDraft(one, coffee, "Filter coffee");
    expect(leaveText(two)).toBe("Your changes to Venue and Filter coffee are not saved.");
    const same = holdDraft(two, Symbol("again"), "Venue");
    expect(leaveText(same)).toBe("Your changes to Venue and Filter coffee are not saved.");
    expect(leaveText(holdDraft(two, Symbol("third"), "Tax"))).toBe("Your changes to Venue, Filter coffee and Tax are not saved.");
  });

  it("still says something with none", () => {
    expect(leaveText(noDrafts)).toBe("Your changes are not saved.");
  });
});

describe("opensElsewhere", () => {
  const click = { metaKey: false, ctrlKey: false, shiftKey: false, button: 0 };
  it("lets a click to another tab or window through, and asks about a plain one", () => {
    expect(opensElsewhere(click)).toBe(false);
    expect(opensElsewhere({ ...click, ctrlKey: true })).toBe(true);
    expect(opensElsewhere({ ...click, metaKey: true })).toBe(true);
    expect(opensElsewhere({ ...click, shiftKey: true })).toBe(true);
    expect(opensElsewhere({ ...click, button: 1 })).toBe(true);
  });
});
