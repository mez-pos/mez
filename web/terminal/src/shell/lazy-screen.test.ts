import { createElement, Suspense } from "react";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { lazyScreen, retryAfterMs } from "./lazy-screen";

/** A stand-in for a screen: draws its name. */
const Hello = ({ name }: { name: string }) => createElement("p", null, `Hello ${name}`);

/** A load that counts its calls and fails the first `failures` of them. */
function loader(failures = 0) {
  const calls = { count: 0 };
  const load = () => {
    calls.count += 1;
    return calls.count <= failures ? Promise.reject(new Error("chunk missing")) : Promise.resolve(Hello);
  };
  return { calls, load };
}

/** What the screen draws in one pass, with "waiting" where it would suspend. Suspense's own markers are left out. */
function draw(Screen: ReturnType<typeof lazyScreen<{ name: string }>>) {
  const html = renderToString(createElement(Suspense, { fallback: "waiting" }, createElement(Screen, { name: "till" })));
  return html.replace(/<!--.*?-->/g, "");
}

describe("lazyScreen", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("draws a preloaded screen in the first pass, with no fallback", async () => {
    const { load } = loader();
    const Screen = lazyScreen("Hello", load);
    await Screen.preload();
    expect(draw(Screen)).toBe("<p>Hello till</p>");
  });

  it("shows the fallback while a screen nobody preloaded is fetched", () => {
    const { load } = loader();
    const html = draw(lazyScreen("Hello", load));
    expect(html).toContain("waiting");
    expect(html).not.toContain("Hello till");
  });

  it("fetches once however often it is asked", async () => {
    const { calls, load } = loader();
    const Screen = lazyScreen("Hello", load);
    await Promise.all([Screen.preload(), Screen.preload()]);
    await Screen.preload();
    expect(calls.count).toBe(1);
  });

  it("forgets a failed fetch, so the next ask fetches again", async () => {
    const { calls, load } = loader(1);
    const Screen = lazyScreen("Hello", load);
    await expect(Screen.preload()).rejects.toThrow("chunk missing");
    await Screen.preload();
    expect(calls.count).toBe(2);
    expect(draw(Screen)).toBe("<p>Hello till</p>");
  });

  it("a warm that failed is not held against the screen: opening it fetches again", async () => {
    const { calls, load } = loader(1);
    const Screen = lazyScreen("Hello", load);
    await Screen.preload().catch(() => undefined);
    draw(Screen);
    await Screen.preload();
    expect(calls.count).toBe(2);
    expect(draw(Screen)).toBe("<p>Hello till</p>");
  });

  it("stays failed once it failed as it was drawn, for the boundary to show, with no loop of fetches", async () => {
    const { calls, load } = loader(5);
    const Screen = lazyScreen("Hello", load);
    draw(Screen);
    await new Promise((settled) => setTimeout(settled, 0));
    draw(Screen);
    draw(Screen);
    expect(calls.count).toBe(1);
  });

  it("fetches again once the error has had its moment: Try again, or coming back to the route, is a new fetch", async () => {
    vi.useFakeTimers();
    const { calls, load } = loader(1);
    const Screen = lazyScreen("Hello", load);
    draw(Screen);
    await vi.advanceTimersByTimeAsync(0);
    draw(Screen);
    expect(calls.count).toBe(1);
    await vi.advanceTimersByTimeAsync(retryAfterMs);
    draw(Screen);
    await vi.advanceTimersByTimeAsync(0);
    expect(calls.count).toBe(2);
    expect(draw(Screen)).toBe("<p>Hello till</p>");
  });

  it("carries the screen's name, for the error report's component stack", () => {
    expect(lazyScreen("PayPage", loader().load).displayName).toBe("PayPage");
  });
});
