import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { debounce } from "./debounce";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("debounce", () => {
  it("calls once, with the last arguments, after the quiet period", () => {
    const fn = vi.fn();
    const d = debounce(fn, 200);
    d.call("f");
    vi.advanceTimersByTime(100);
    d.call("fl");
    vi.advanceTimersByTime(100);
    d.call("fla");
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith("fla");
  });

  it("flush runs the pending call at once, and only once", () => {
    const fn = vi.fn();
    const d = debounce(fn, 200);
    d.call("latte");
    d.flush();
    vi.advanceTimersByTime(500);
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith("latte");
  });

  it("flush with nothing pending does nothing", () => {
    const fn = vi.fn();
    debounce(fn, 200).flush();
    expect(fn).not.toHaveBeenCalled();
  });

  it("cancel drops the pending call", () => {
    const fn = vi.fn();
    const d = debounce(fn, 200);
    d.call("x");
    d.cancel();
    vi.advanceTimersByTime(500);
    expect(fn).not.toHaveBeenCalled();
  });
});
