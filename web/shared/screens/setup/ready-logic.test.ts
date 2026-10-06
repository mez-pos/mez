import { describe, expect, it } from "vitest";
import { ApiError, type ApiClient } from "../../api";
import { codeError, makeCode, openAt, secondDevice, secondDevices, serverAddresses, typed, untilWords } from "./ready-logic";

describe("secondDevices", () => {
  it("offers a till, a kitchen display and a back office, and never a guest", () => {
    expect(secondDevices.map((d) => d.kind)).toEqual(["terminal", "kitchen", "back_office"]);
  });

  it("falls back to a till for a kind it does not know", () => {
    expect(secondDevice("kitchen").label).toBe("Kitchen display");
    expect(secondDevice("guest").kind).toBe("terminal");
  });

  it.each([
    ["terminal", "http://192.168.1.10:7777"],
    ["kitchen", "http://192.168.1.10:7777/kitchen"],
    ["back_office", "http://192.168.1.10:7777/backoffice/"],
  ])("a %s opens %s", (kind, want) => {
    expect(openAt("http://192.168.1.10:7777", secondDevice(kind))).toBe(want);
    expect(openAt("http://192.168.1.10:7777/", secondDevice(kind))).toBe(want);
  });

  it("writes an address as it is typed", () => {
    expect(typed("http://192.168.1.10:7777/kitchen")).toBe("192.168.1.10:7777/kitchen");
  });
});

describe("serverAddresses", () => {
  it("is the server's list, or none when it cannot be asked", async () => {
    const ok = { get: async () => ({ urls: ["http://192.168.1.10:7777"] }) } as unknown as ApiClient;
    expect(await serverAddresses(ok)).toEqual(["http://192.168.1.10:7777"]);
    const down = { get: async () => Promise.reject(new ApiError("network", "unreachable")) } as unknown as ApiClient;
    expect(await serverAddresses(down)).toEqual([]);
  });
});

describe("makeCode", () => {
  it("asks for a code for that kind of device, by its starting name", async () => {
    const asked: unknown[] = [];
    const api = {
      post: async (path: string, body: unknown) => {
        asked.push(path, body);
        return { code: "ABCD-2345", expires_at: "2026-10-07T04:40:00Z" };
      },
    } as unknown as ApiClient;
    expect((await makeCode(api, secondDevice("kitchen"))).code).toBe("ABCD-2345");
    expect(asked).toEqual(["/api/v1/devices/codes", { name: "Kitchen", kind: "kitchen" }]);
  });
});

describe("untilWords", () => {
  it("is a time of day, or ten minutes when the server's time cannot be read", () => {
    expect(untilWords("2026-10-07T04:40:00Z", "en-GB")).toMatch(/^until \d{1,2}:\d{2}$/);
    expect(untilWords("soon")).toBe("for 10 minutes");
  });
});

describe("codeError", () => {
  it.each<[unknown, string]>([
    [new ApiError("network", "unreachable"), "localhost:7777"],
    [new ApiError("forbidden", "no", 403, "device.manage"), "under Devices"],
    [new ApiError("invalid_request", "a name of 1 to 40 characters is needed", 400, "name"), "a name of 1 to 40"],
    [new TypeError("x"), "Try again"],
  ])("%s", (err, says) => {
    expect(codeError(err, "localhost:7777")).toContain(says);
  });
});
