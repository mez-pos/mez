import { describe, expect, it, vi } from "vitest";
import { askMirror, headerMirror, mirrorAsOf, mirrorText, MirrorWatch, venueOffline, venueOfflineAfterMs, type MirrorMark } from "./mirror";

const answer = (mirror?: string) => new Response(null, { headers: mirror === undefined ? {} : { [headerMirror]: mirror } });
const at = (iso: string) => Date.parse(iso);

describe("mirrorAsOf", () => {
  const tests: { name: string; header: string | undefined; want: { asOf: number | null } | null }[] = [
    { name: "an answer from a venue says nothing", header: undefined, want: null },
    { name: "a mirror's, to the nanosecond", header: "2026-10-12T17:40:05.123456789Z", want: { asOf: at("2026-10-12T17:40:05.123Z") } },
    { name: "a mirror's, to the second", header: "2026-10-12T17:40:05Z", want: { asOf: at("2026-10-12T17:40:05Z") } },
    { name: "a mirror that cannot say how old its copy is", header: "", want: { asOf: null } },
    { name: "a mirror saying something that is not a time", header: "yesterday", want: { asOf: null } },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(mirrorAsOf(answer(tt.header))).toEqual(tt.want);
    });
  }
});

describe("MirrorWatch", () => {
  it("is nothing until a mirror answers, and stays nothing at a venue", () => {
    const watch = new MirrorWatch(() => 1_000);
    expect(watch.current()).toBeNull();
    watch.heard(answer());
    expect(watch.current()).toBeNull();
  });

  it("keeps when the copy was written and when it was told", () => {
    let now = 5_000;
    const watch = new MirrorWatch(() => now);
    const told = vi.fn();
    watch.subscribe(told);

    watch.heard(answer("2026-10-12T17:40:00Z"));
    expect(watch.current()).toEqual({ asOf: at("2026-10-12T17:40:00Z"), heardAt: 5_000 });
    expect(told).toHaveBeenCalledTimes(1);

    now = 9_000;
    watch.heard(answer("2026-10-12T17:41:30Z"));
    expect(watch.current()).toEqual({ asOf: at("2026-10-12T17:41:30Z"), heardAt: 9_000 });
    expect(told).toHaveBeenCalledTimes(2);
  });

  it("never goes back: an answer from the copy before, landing late, moves only when it was heard", () => {
    let now = 1;
    const watch = new MirrorWatch(() => now);
    watch.heard(answer("2026-10-12T17:41:30Z"));
    now = 2;
    watch.heard(answer("2026-10-12T17:40:00Z"));
    expect(watch.current()).toEqual({ asOf: at("2026-10-12T17:41:30Z"), heardAt: 2 });
    now = 3;
    watch.heard(answer(""));
    expect(watch.current()).toEqual({ asOf: at("2026-10-12T17:41:30Z"), heardAt: 3 });
  });

  it("is still a mirror after an answer that carries no word of it", () => {
    const watch = new MirrorWatch(() => 7);
    watch.heard(answer(""));
    watch.heard(answer());
    expect(watch.current()).toEqual({ asOf: null, heardAt: 7 });
  });

  it("stops telling a listener that has left", () => {
    const watch = new MirrorWatch();
    const told = vi.fn();
    const leave = watch.subscribe(told);
    leave();
    watch.heard(answer(""));
    expect(told).not.toHaveBeenCalled();
  });
});

describe("venueOffline", () => {
  const asOf = at("2026-10-12T17:40:00Z");
  const tests: { name: string; mark: MirrorMark; want: boolean }[] = [
    { name: "a copy written a moment before it was read", mark: { asOf, heardAt: asOf + 5_000 }, want: false },
    { name: "exactly two minutes old", mark: { asOf, heardAt: asOf + venueOfflineAfterMs }, want: false },
    { name: "a millisecond over two minutes", mark: { asOf, heardAt: asOf + venueOfflineAfterMs + 1 }, want: true },
    { name: "last night's", mark: { asOf, heardAt: asOf + 9 * 3_600_000 }, want: true },
    { name: "a copy that cannot say", mark: { asOf: null, heardAt: asOf }, want: true },
    { name: "a browser whose clock is behind the venue's", mark: { asOf, heardAt: asOf - 60_000 }, want: false },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(venueOffline(tt.mark)).toBe(tt.want);
    });
  }
});

describe("mirrorText", () => {
  const now = at("2026-10-12T18:00:00Z"); // 23:30 in Kolkata

  it("says the time in the venue's zone, on the 24 hour clock", () => {
    expect(mirrorText({ asOf: at("2026-10-12T17:40:00Z"), heardAt: now }, now, "Asia/Kolkata")).toBe("Mirror, as of 23:10");
    expect(mirrorText({ asOf: at("2026-10-12T03:35:00Z"), heardAt: now }, now, "Asia/Kolkata")).toBe("Mirror, as of 09:05");
  });

  it("says the day when it is not today at the venue", () => {
    expect(mirrorText({ asOf: at("2026-10-11T17:40:00Z"), heardAt: now }, now, "Asia/Kolkata")).toBe("Mirror, as of 11 Oct, 23:10");
    // The same hour, read a day on: it is the venue's day that is meant.
    const next = at("2026-10-13T04:00:00Z");
    expect(mirrorText({ asOf: at("2026-10-12T17:40:00Z"), heardAt: next }, next, "Asia/Kolkata")).toBe("Mirror, as of 12 Oct, 23:10");
  });

  it("is midnight as 00, not 24", () => {
    const later = at("2026-10-12T19:00:00Z");
    expect(mirrorText({ asOf: at("2026-10-12T18:30:00Z"), heardAt: later }, later, "Asia/Kolkata")).toBe("Mirror, as of 00:00");
  });

  it("says only that it is a mirror when the copy cannot say how old it is", () => {
    expect(mirrorText({ asOf: null, heardAt: now }, now, "Asia/Kolkata")).toBe("Mirror");
  });

  it("falls back to the device's clock for no zone or one it does not know", () => {
    const mark = { asOf: at("2026-10-12T17:40:00Z"), heardAt: now };
    expect(mirrorText(mark, now, null)).toMatch(/^Mirror, as of (\d{1,2} \w{3}, )?\d{2}:\d{2}$/);
    expect(mirrorText(mark, now, "Mars/Olympus")).toMatch(/^Mirror, as of (\d{1,2} \w{3}, )?\d{2}:\d{2}$/);
  });
});

describe("askMirror", () => {
  it("asks who this device is, which every paired browser may", async () => {
    const get = vi.fn().mockResolvedValue({});
    await askMirror({ get });
    expect(get).toHaveBeenCalledWith("/api/v1/devices/me", { timeoutMs: 4_000 });
  });

  it("leaves a mirror that does not answer to the next time", async () => {
    const get = vi.fn().mockRejectedValue(new Error("gone"));
    await expect(askMirror({ get })).resolves.toBeUndefined();
  });
});
