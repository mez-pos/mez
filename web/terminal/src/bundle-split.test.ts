// The terminal is fetched in parts (TECH-25, ADR-408), and the parts hold
// only while no file imports, by a plain import, what is meant to be fetched
// apart. Nothing in the build says so when one does: the bundler quietly puts
// the screen back into the file every device parses. So this reads the
// imports themselves.

import { describe, expect, it } from "vitest";

/** A path joined and tidied, "a/b" with "../c" being "a/c". Written here: the app's own types have no Node. */
function join(dir: string, specifier: string): string {
  const parts: string[] = [];
  for (const part of [...dir.split("/"), ...specifier.split("/")]) {
    if (part === "" || part === ".") continue;
    if (part === ".." && parts.length > 0 && parts[parts.length - 1] !== "..") parts.pop();
    else parts.push(part);
  }
  return parts.join("/");
}

/** The folder a file is in. */
const folderOf = (file: string): string => file.slice(0, Math.max(0, file.lastIndexOf("/")));

// Every source file of the terminal and the shared kit, as text, by its path from terminal/src.
const sources = new Map(
  Object.entries(
    import.meta.glob(["./**/*.{ts,tsx}", "../../shared/**/*.{ts,tsx}", "!**/*.test.*"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >,
  ).map(([file, text]) => [join("", file), text]),
);

/** A relative specifier, as the file it names. */
function resolve(from: string, specifier: string): string | null {
  const base = join(folderOf(from), specifier);
  const found = [base, `${base}.ts`, `${base}.tsx`, `${base}/index.ts`, `${base}/index.tsx`].find((f) => sources.has(f));
  return found ?? null;
}

/** An import or export statement that names a file of ours: `import x from`, `export x from`, `import "x"`. */
const STATEMENT = /^(?:import|export)\s+(type\s+)?([\w\s{},*$]*?)\s*from\s+"(\.[^"]+)"|^import\s+"(\.[^"]+)"/gm;
/** Every file of ours a source names, however it is written: what STATEMENT must account for. */
const ANY_NAMED = /\bfrom\s+["'](\.[^"']+)["']|^\s*import\s+["'](\.[^"']+)["']/gm;

/**
 * The files a file imports when it runs. Not `import("x")`, which is fetched
 * apart, nor a type, which is gone once compiled.
 */
function staticImports(file: string): string[] {
  const text = sources.get(file) ?? "";
  return [...text.matchAll(STATEMENT)].flatMap(([, typeOnly, names = "", from, bare]) => {
    const specifier = from ?? bare;
    if (!specifier || typeOnly) return [];
    // `import { type A, type B } from` is a type too.
    const braces = names.match(/^\{([^}]*)\}$/);
    const allTypes = braces?.[1]?.split(",").every((name) => name.trim() === "" || name.trim().startsWith("type "));
    if (allTypes) return [];
    const target = resolve(file, specifier);
    return target ? [target] : [];
  });
}

/** The files a file's own imports, and theirs, bring in with it. */
function graph(entry: string): Set<string> {
  const seen = new Set([entry]);
  const queue = [entry];
  for (let file = queue.shift(); file; file = queue.shift()) {
    for (const next of staticImports(file)) {
      if (!seen.has(next)) {
        seen.add(next);
        queue.push(next);
      }
    }
  }
  return seen;
}

/** What a file fetches apart: its `import("./x")`s. */
function fetchedApart(file: string): string[] {
  const text = sources.get(file) ?? "";
  return [...text.matchAll(/import\("(\.[^"]+)"\)/g)].flatMap(([, specifier]) => {
    const target = specifier ? resolve(file, specifier) : null;
    return target ? [target] : [];
  });
}

describe("the terminal in parts (TECH-25, ADR-408)", () => {
  it("boots without the till or the kitchen display: each device fetches its own", () => {
    const apart = fetchedApart("main.tsx");
    expect(apart).toEqual(expect.arrayContaining(["App.tsx", "kitchen/KitchenApp.tsx"]));
    const boot = graph("main.tsx");
    expect(apart.filter((file) => boot.has(file))).toEqual([]);
  });

  it("boots the till without the screens it fetches while idle", () => {
    const apart = fetchedApart("App.tsx");
    const routes = [
      "shell/PayPage.tsx",
      "shell/ShiftPages.tsx",
      "waiter/WaiterHome.tsx",
      "waiter/ShiftSummary.tsx",
      "waiter/TableDetail.tsx",
      "kitchen/CookBoard.tsx",
    ];
    expect(apart).toEqual(expect.arrayContaining(routes));
    const till = graph("App.tsx");
    expect(apart.filter((file) => till.has(file))).toEqual([]);
  });

  it("draws the kitchen display without any of the till's screens", () => {
    // The kitchen shares the till's rules (a ticket's title, a line's key,
    // a tender's failure), which are .ts; a component of the till's is not.
    const tillScreens = [...graph("kitchen/KitchenApp.tsx")].filter(
      (file) => file.endsWith(".tsx") && !file.startsWith("kitchen/") && !file.startsWith("../../shared/"),
    );
    expect(tillScreens).toEqual([]);
    expect(graph("kitchen/KitchenApp.tsx").has("App.tsx")).toBe(false);
  });

  it("misses no import: each is read, and each names a file", () => {
    // A statement the walk cannot read (a comment inside its braces, single
    // quotes) would be skipped, and the checks above would pass without it.
    const unread = [...sources].flatMap(([file, text]) => {
      const missed = [...text.matchAll(ANY_NAMED)].length - [...text.matchAll(STATEMENT)].length;
      return missed === 0 ? [] : [`${file}: ${missed} not read`];
    });
    expect(unread).toEqual([]);
    const unnamed = [...sources].flatMap(([file, text]) =>
      [...text.matchAll(ANY_NAMED)]
        .map(([, from, bare]) => from ?? bare ?? "")
        .filter((specifier) => !specifier.endsWith(".css") && resolve(file, specifier) === null)
        .map((specifier) => `${file}: ${specifier}`),
    );
    expect(unnamed).toEqual([]);
  });

  it("reads imports as the bundler does", () => {
    // The walk itself: a type import is not followed, a plain one is.
    expect(staticImports("kitchen/KitchenApp.tsx")).not.toContain("kitchen/BoardFoot.tsx");
    expect(staticImports("kitchen/KitchenApp.tsx")).toContain("kitchen/KitchenBoard.tsx");
    expect(graph("App.tsx").has("register/OrderScreen.tsx")).toBe(true);
  });
});
