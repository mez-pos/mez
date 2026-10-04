import { describe, expect, it } from "vitest";
import { route } from "./routes";

// The server gives /order/ to the guest app and /backoffice/ to the back office
// (web/web.go). A terminal route there is served the wrong app on a reload, the
// one thing a link-by-link walk through the screens never shows.
const mounts = ["/order", "/backoffice"];

const everyRoute = (): string[] => [...Object.values(route).flatMap((r) => (typeof r === "string" ? [r] : [r("ord_ABC1234" as never)]))];

describe("route", () => {
  it("keeps every terminal path out of the other apps' mounts", () => {
    for (const path of everyRoute()) {
      for (const mount of mounts) {
        expect(path === mount || path.startsWith(`${mount}/`), `${path} is under ${mount}`).toBe(false);
      }
    }
  });

  it("builds a path for an order or a Z-report from its id", () => {
    expect(route.bill("ord_ABC")).toBe("/orders/ord_ABC");
    expect(route.pay("ord_ABC")).toBe("/pay/ord_ABC");
    expect(route.zReport(12)).toBe("/shift/z/12");
  });

  it("escapes an id that would change the path", () => {
    expect(route.pay("a/b")).toBe("/pay/a%2Fb");
  });
});
