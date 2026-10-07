import { describe, expect, it } from "vitest";
import { lockScroll } from "./scroll-lock";

const page = (overflow = "") => ({ style: { overflow } });

describe("lockScroll", () => {
  it("hides overflow while held and puts back what was there", () => {
    const root = page("auto");
    const release = lockScroll(root);
    expect(root.style.overflow).toBe("hidden");
    release();
    expect(root.style.overflow).toBe("auto");
  });

  it("scrolls again only when the last of two dialogs lets go, in either order", () => {
    const root = page("");
    const first = lockScroll(root);
    const second = lockScroll(root);
    first(); // the one opened first closes first: the sheet above it is still open
    expect(root.style.overflow).toBe("hidden");
    second();
    expect(root.style.overflow).toBe("");

    const a = lockScroll(root);
    const b = lockScroll(root);
    b();
    expect(root.style.overflow).toBe("hidden");
    a();
    expect(root.style.overflow).toBe("");
  });

  it("counts a release once, however often it is called", () => {
    const root = page("");
    const first = lockScroll(root);
    const second = lockScroll(root);
    first();
    first();
    expect(root.style.overflow).toBe("hidden");
    second();
    expect(root.style.overflow).toBe("");
  });
});
