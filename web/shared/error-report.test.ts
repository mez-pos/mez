import { afterEach, describe, expect, it, vi } from "vitest";
import type { ClientErrorRequest } from "./api-types";
import { fallbackWords, MAX_REPORTS, ReportGate, reportOf, reportRenderError, setErrorReporter } from "./error-report";

describe("reportOf", () => {
  it("is the error's message and stack, React's component stack, and where", () => {
    const error = new TypeError("Cannot read properties of undefined (reading 'qty')");
    const r = reportOf("terminal", "rail", error, "\n    at OrderRail\n    at OrderScreen", "/order");
    expect(r).toMatchObject({
      app: "terminal",
      where: "rail",
      path: "/order",
      message: error.message,
      component_stack: "\n    at OrderRail\n    at OrderScreen",
    });
    expect(r.stack).toContain("TypeError");
  });

  it("takes something thrown that is not an Error", () => {
    expect(reportOf("backoffice", "route", "a string", null, "/menu")).toMatchObject({ message: "a string", stack: "", component_stack: "" });
  });

  it("keeps the path and never what follows it: a pairing code or a handed-over token", () => {
    expect(reportOf("terminal", "route", new Error("x"), "", "/pay/ord_1?x=1#code=7K3M").path).toBe("/pay/ord_1");
    expect(reportOf("backoffice", "root", new Error("x"), "", "/backoffice#cloud=secret").path).toBe("/backoffice");
  });

  it("cuts what the server would cut", () => {
    const r = reportOf("terminal", "route", new Error("m".repeat(900)), "c".repeat(9000), "/" + "p".repeat(400));
    expect([r.message.length, r.component_stack.length, r.path.length]).toEqual([500, 4000, 200]);
  });
});

describe("ReportGate", () => {
  const report = (message: string): Pick<ClientErrorRequest, "where" | "path" | "message"> => ({ where: "route", path: "/order", message });

  it("lets each different failure through once", () => {
    const gate = new ReportGate();
    expect(gate.allow(report("a"))).toBe(true);
    expect(gate.allow(report("a"))).toBe(false);
    expect(gate.allow(report("b"))).toBe(true);
    expect(gate.allow({ ...report("a"), where: "rail" })).toBe(true);
  });

  it("lets no more than MAX_REPORTS through in all", () => {
    const gate = new ReportGate();
    const sent = Array.from({ length: MAX_REPORTS + 5 }, (_, i) => gate.allow(report(`e${i}`))).filter(Boolean);
    expect(sent).toHaveLength(MAX_REPORTS);
  });
});

describe("reportRenderError", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("sends what a boundary caught, as the app that set the reporter, once", () => {
    vi.stubGlobal("window", { location: { pathname: "/order" } });
    const send = vi.fn((_: ClientErrorRequest) => Promise.resolve());
    setErrorReporter("kitchen", send);
    reportRenderError("route", new Error("boom"), "\n    at Board");
    reportRenderError("route", new Error("boom"), "\n    at Board");
    expect(send).toHaveBeenCalledTimes(1);
    expect(send.mock.calls[0]?.[0]).toMatchObject({ app: "kitchen", where: "route", path: "/order", message: "boom" });
  });

  it("never throws: a send that fails or throws is dropped", async () => {
    vi.stubGlobal("window", { location: { pathname: "/menu" } });
    setErrorReporter("backoffice", () => Promise.reject(new Error("offline")));
    expect(() => reportRenderError("modal", new Error("one"), "")).not.toThrow();
    setErrorReporter("backoffice", () => {
      throw new Error("no client");
    });
    expect(() => reportRenderError("modal", new Error("two"), "")).not.toThrow();
    await Promise.resolve();
  });
});

describe("fallbackWords", () => {
  it("says what failed and what is kept, for each size", () => {
    expect(fallbackWords("page")).toEqual({
      title: "Something went wrong on this screen",
      line: "Reload to carry on. Everything the server has recorded is kept.",
    });
    expect(fallbackWords("region").title).toBe("This part of the screen failed to draw");
    expect(fallbackWords("modal")).toEqual({
      title: "This window failed to draw",
      line: "Try again, or close it and open it again. Anything already saved is kept.",
    });
  });

  it("takes a place's own line", () => {
    expect(fallbackWords("region", "Any payment already taken is recorded.").line).toBe("Any payment already taken is recorded.");
  });
});
