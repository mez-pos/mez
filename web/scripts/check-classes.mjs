// Fails the build on a class that renders nothing (TECH-21, ADR-405).
//
// Tailwind writes CSS only for the classes it knows; a class it does not
// know, `bg-white` where the theme has no white, or `text-heading` where the
// colour is `fg-heading`, is kept in the markup and does nothing, and no step
// of the build says so. This reads every string in the apps' sources, takes
// the ones that are lists of classes (most of their words are classes the
// built CSS has), and names every word in them the CSS does not have.
//
// Run after the build: it reads terminal/dist, backoffice/dist and
// guest/dist. A word that is a class on purpose but never styled (a hook
// for a test or a selector) goes in `meant` below, with why.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const apps = ["terminal", "backoffice", "guest"];
const sourceDirs = ["terminal/src", "backoffice/src", "guest/src", "shared"];

// Classes that style nothing by themselves, on purpose.
const meant = new Set([
  // Tailwind's markers: what group-* and peer-* variants look up to.
  "group",
  "peer",
]);

function files(dir, keep) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (name === "node_modules" || name === "dist") return [];
    return statSync(path).isDirectory() ? files(path, keep) : keep(path) ? [path] : [];
  });
}

// Every class a built stylesheet has a selector for, as it is written in
// markup: `.hover\:bg-x:hover` is `hover:bg-x`.
function classesIn(css) {
  const out = new Set();
  // A class starts with a letter, a dash, an underscore or an escape: a dot
  // before a digit is a decimal, "0.6s", not a class.
  for (const m of css.matchAll(/(?<![\w\\])\.((?:\\[0-9a-fA-F]{1,6} ?|\\.|[A-Za-z_-])(?:\\[0-9a-fA-F]{1,6} ?|\\.|[\w-])*)/g)) {
    const name = m[1].replace(/\\([0-9a-fA-F]{1,6}) ?|\\(.)/g, (_, hex, ch) => (hex ? String.fromCodePoint(parseInt(hex, 16)) : ch));
    out.add(name);
  }
  return out;
}

const known = new Set(meant);
for (const app of apps) {
  const dist = join(root, app, "dist");
  for (const path of files(dist, (p) => p.endsWith(".css"))) {
    for (const c of classesIn(readFileSync(path, "utf8"))) known.add(c);
  }
}
if (known.size < 500) {
  console.error(`  classes  only ${known.size} classes in the built CSS: build the apps first`);
  process.exit(1);
}

// A word that could be a class: lower case, no spaces, the characters
// utilities and their variants use.
const classLike = /^-?[a-z0-9*@[(!][\w:./\-[\]()%#,&>*='"!@~+]*$/;
// Tailwind's classes are lower case, but for what is in brackets: "12H5M11"
// in an icon's path is not one.
const isClassLike = (w) => classLike.test(w) && !/[A-Z]/.test(w.replace(/\[[^\]]*\]/g, ""));

const dead = [];
for (const dir of sourceDirs) {
  for (const path of files(join(root, dir), (p) => /\.(ts|tsx)$/.test(p) && !/\.test\.tsx?$/.test(p))) {
    const text = readFileSync(path, "utf8");
    // Every string literal: "...", '...' and the literal runs of a `...`.
    for (const m of text.matchAll(/"((?:[^"\\\n]|\\.)*)"|'((?:[^'\\\n]|\\.)*)'|`((?:[^`\\]|\\.)*)`/g)) {
      const body = (m[1] ?? m[2] ?? m[3] ?? "").replace(/\$\{[^}]*\}/g, " ");
      const words = body.split(/\s+/).filter(Boolean);
      if (words.length < 2) continue;
      const candidates = words.filter(isClassLike);
      const hits = candidates.filter((w) => known.has(w));
      // A list of classes: nearly every word a class, most of them ones the CSS has.
      if (candidates.length < words.length * 0.8 || hits.length < Math.max(2, candidates.length * 0.6)) continue;
      const line = text.slice(0, m.index).split("\n").length;
      for (const w of candidates) {
        if (!known.has(w)) dead.push(`${relative(root, path).replaceAll("\\", "/")}:${line}  ${w}`);
      }
    }
  }
}

if (dead.length > 0) {
  console.error(`  classes  ${dead.length} that render nothing:`);
  for (const d of dead) console.error(`    ${d}`);
  process.exit(1);
}
console.log(`  classes  every class in the sources renders (${known.size} in the built CSS)`);
