import { describe, expect, it } from "vitest";
import { ApiError, type ApiClient } from "../../api";
import type { AddressResponse } from "../../api-types";
import { codeError, makeCode, placesFor, secondDevice, secondDevices, serverAddresses, svgSrc, typed, untilWords } from "./ready-logic";

/** An address as the server answers it, with every app at it. */
function address(host: string, adapter: string, virtual = false): AddressResponse {
  const base = `http://${host}:7777`;
  return {
    adapter,
    virtual,
    apps: [
      { kind: "terminal", url: base, svg: `<svg>${host}</svg>` },
      { kind: "kitchen", url: `${base}/kitchen`, svg: `<svg>${host}/kitchen</svg>` },
      { kind: "back_office", url: `${base}/backoffice/`, svg: `<svg>${host}/backoffice/</svg>` },
    ],
  };
}

describe("secondDevices", () => {
  it("offers a till, a kitchen display and a back office, and never a guest", () => {
    expect(secondDevices.map((d) => d.kind)).toEqual(["terminal", "kitchen", "back_office"]);
  });

  it("falls back to a till for a kind it does not know", () => {
    expect(secondDevice("kitchen").label).toBe("Kitchen display");
    expect(secondDevice("guest").kind).toBe("terminal");
  });

  it("writes an address as it is typed", () => {
    expect(typed("http://192.168.1.10:7777/kitchen")).toBe("192.168.1.10:7777/kitchen");
  });
});

describe("placesFor", () => {
  const wifi = address("192.168.29.211", "Wi-Fi");
  const cable = address("192.168.1.40", "Ethernet");
  const wsl = address("172.25.224.1", "vEthernet (WSL)", true);
  const vbox = address("10.0.75.1", "VirtualBox Host-Only Network", true);

  it("shows the one real adapter's address large, and virtual ones small", () => {
    const { main, others } = placesFor([wifi, wsl, vbox], "kitchen");
    expect(main).toEqual([{ url: "http://192.168.29.211:7777/kitchen", svg: "<svg>192.168.29.211/kitchen</svg>", adapter: "Wi-Fi" }]);
    expect(others.map((p) => p.adapter)).toEqual(["vEthernet (WSL)", "VirtualBox Host-Only Network"]);
  });

  it("shows two real adapters as equals, in the server's order", () => {
    const { main, others } = placesFor([cable, wifi, wsl], "terminal");
    expect(main.map((p) => [p.adapter, p.url])).toEqual([
      ["Ethernet", "http://192.168.1.40:7777"],
      ["Wi-Fi", "http://192.168.29.211:7777"],
    ]);
    expect(others).toHaveLength(1);
  });

  it("shows the virtual adapters large when they are all there is", () => {
    const { main, others } = placesFor([wsl, vbox], "back_office");
    expect(main.map((p) => p.url)).toEqual(["http://172.25.224.1:7777/backoffice/", "http://10.0.75.1:7777/backoffice/"]);
    expect(others).toEqual([]);
  });

  it("has nothing to show with no addresses, or for a kind the server did not answer for", () => {
    expect(placesFor([], "terminal")).toEqual({ main: [], others: [] });
    expect(placesFor([wifi], "guest")).toEqual({ main: [], others: [] });
  });
});

describe("svgSrc", () => {
  it("is a data address an image can show, whatever is in the SVG", () => {
    const src = svgSrc('<svg viewBox="0 0 1 1"><path d="M0 0#"/></svg>');
    expect(src.startsWith("data:image/svg+xml;charset=utf-8,")).toBe(true);
    expect(src).not.toMatch(/[<>"#]/);
  });
});

describe("serverAddresses", () => {
  it("is the server's list, or none when it cannot be asked", async () => {
    const list = [address("192.168.1.10", "Wi-Fi")];
    const ok = { get: async () => ({ addresses: list }) } as unknown as ApiClient;
    expect(await serverAddresses(ok)).toEqual(list);
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
