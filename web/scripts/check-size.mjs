// Fails the build when the guest bundle is over budget. See CLAUDE.md and
// web/README.md: the guest app loads on a stranger's phone on 3G.
//
// Counted: every .js, .css and .html file in guest/dist, gzipped at level 9,
// which is what a browser must fetch before the first screen can render.
// ADR-063.
//
// Counted apart, against their own budget: the guest's fonts, as they are
// sent. A woff2 is compressed already, so its size on disk is what crosses
// the network. A face is fetched only when the page sets it, but any of them
// may be, so every one in guest/dist counts (GST-11, TECH-18, ADR-397).
// Images are listed and not counted: a dish's photo is the venue's.
//
// It also fails if React's own runtime is in the guest bundle. Guest renders
// through preact/compat (ADR-064), and a stray import that bypasses the alias
// would put 40 KB back without anyone noticing until the budget broke.
//
// The terminal and back office have no budget; their sizes are printed so a
// jump is visible in the build log. The terminal is fetched in parts (TECH-25,
// ADR-408), so it is printed by device: what the kitchen display and the till
// each parse before their first screen, and what the till fetches once idle.
// It fails if the parts are gone, or if the kitchen display's reach the till's.
// Which source file imports which is held by terminal/src/bundle-split.test.ts.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { basename, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const GUEST_BUDGET_BYTES = 60 * 1024;
const GUEST_FONT_BUDGET_BYTES = 72 * 1024;
const COUNTED = /\.(js|css|html)$/;
const FONT = /\.woff2?$/;
// A string only React's runtime contains; Preact has no equivalent.
const REACT_MARKER = "__SECRET_INTERNALS_DO_NOT_USE";
const root = fileURLToPath(new URL("..", import.meta.url));

function files(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    return statSync(path).isDirectory() ? files(path) : [path];
  });
}

function measure(app) {
  const dist = join(root, app, "dist");
  let all;
  try {
    all = files(dist);
  } catch {
    throw new Error(`${app}/dist not found: run \`npm run build\` first`);
  }
  const counted = all
    .filter((f) => COUNTED.test(f))
    .map((f) => {
      const body = readFileSync(f);
      return { file: relative(dist, f), gz: gzipSync(body, { level: 9 }).length, react: body.includes(REACT_MARKER) };
    });
  const sized = (f) => ({ file: relative(dist, f), raw: statSync(f).size });
  const fonts = all.filter((f) => FONT.test(f)).map(sized);
  const other = all.filter((f) => !COUNTED.test(f) && !FONT.test(f)).map(sized);
  return {
    counted,
    fonts,
    other,
    total: counted.reduce((sum, f) => sum + f.gz, 0),
    fontTotal: fonts.reduce((sum, f) => sum + f.raw, 0),
  };
}

const kb = (bytes) => `${(bytes / 1024).toFixed(1)} KB`;

// A built file's own imports, parsed with it: `from"./x.js"` and
// `import"./x.js"`. Not `import("./x.js")`, which is fetched apart.
const STATIC_IMPORT = /(?:from|import)\s*"\.\/([^"]+\.js)"/g;

/** What the terminal's devices parse, by the built files' own imports. Returns the failures. */
function terminalParts(counted) {
  const assets = join(root, "terminal", "dist", "assets");
  // By file name: `relative` writes assets\x.js on Windows and assets/x.js elsewhere.
  const gz = new Map(counted.map((f) => [basename(f.file), f.gz]));
  const html = readFileSync(join(root, "terminal", "dist", "index.html"), "utf8");
  const entry = html.match(/src="\/assets\/([^"]+\.js)"/)?.[1];
  // A part is named for the file it starts from, then its hash: App-BD1cYF7F.js.
  const chunk = (name) => [...gz.keys()].find((f) => f.startsWith(`${name}-`) && /^[\w-]+\.js$/.test(f));
  const till = chunk("App");
  const kitchen = chunk("KitchenApp");
  if (!entry || !till || !kitchen) {
    return [
      "the terminal's parts are not where they were: no entry in index.html, or no App or KitchenApp file; the till and the kitchen display must each be fetched apart",
    ];
  }
  const parsed = (...starts) => {
    const seen = new Set(starts);
    const queue = [...starts];
    for (let file = queue.shift(); file; file = queue.shift()) {
      for (const [, dep] of readFileSync(join(assets, file), "utf8").matchAll(STATIC_IMPORT)) {
        if (!seen.has(dep)) {
          seen.add(dep);
          queue.push(dep);
        }
      }
    }
    return seen;
  };
  const sum = (files) => [...files].reduce((total, f) => total + (gz.get(f) ?? 0), 0);
  const kitchenFiles = parsed(entry, kitchen);
  const tillFiles = parsed(entry, till);
  // Everything the till does not boot with, the kitchen's own file among it: the cook's board on a till is drawn by it.
  const later = [...gz.keys()].filter((f) => f.endsWith(".js") && !tillFiles.has(f));
  // Script only: the stylesheet and the page are the same for both, and in the total.
  console.log(`          ${kb(sum(kitchenFiles)).padStart(9)}  script the kitchen display parses before its first screen`);
  console.log(`          ${kb(sum(tillFiles)).padStart(9)}  script the till parses before its first screen`);
  console.log(`          ${kb(sum(later)).padStart(9)}  script the till fetches once it is idle`);
  const failures = [];
  if (kitchenFiles.has(till)) failures.push(`the kitchen display parses the till: ${kitchen} reaches ${till}`);
  if (later.length === 0) failures.push("the till boots with every screen again: pay, the shift's and the waiter's must be fetched once it is idle");
  return failures;
}

let failed = false;
for (const app of ["terminal", "backoffice", "guest"]) {
  const { counted, fonts, other, total, fontTotal } = measure(app);
  if (app !== "guest") {
    console.log(`  size  ${app.padEnd(10)} ${kb(total)} gzipped (no budget)`);
    if (app === "terminal") {
      for (const failure of terminalParts(counted)) {
        console.error(`  size  FAIL: ${failure}`);
        failed = true;
      }
    }
    continue;
  }
  const headroom = GUEST_BUDGET_BYTES - total;
  console.log(`  size  guest      ${kb(total)} gzipped of ${kb(GUEST_BUDGET_BYTES)} (${kb(headroom)} left)`);
  for (const f of counted) console.log(`          ${kb(f.gz).padStart(9)}  ${f.file}`);
  const fontHeadroom = GUEST_FONT_BUDGET_BYTES - fontTotal;
  console.log(`  size  guest fonts ${kb(fontTotal)} of ${kb(GUEST_FONT_BUDGET_BYTES)} (${kb(fontHeadroom)} left)`);
  for (const f of fonts) console.log(`          ${kb(f.raw).padStart(9)}  ${f.file}`);
  for (const f of other) console.log(`          ${kb(f.raw).padStart(9)}  ${f.file} (not counted)`);
  for (const f of counted.filter((c) => c.react)) {
    console.error(`  size  FAIL: guest ${f.file} contains React's runtime; guest must render through preact/compat`);
    failed = true;
  }
  if (headroom < 0) {
    console.error(`  size  FAIL: guest bundle is ${kb(-headroom)} over its ${kb(GUEST_BUDGET_BYTES)} gzipped budget`);
    failed = true;
  }
  if (fontHeadroom < 0) {
    console.error(
      `  size  FAIL: guest fonts are ${kb(-fontHeadroom)} over their ${kb(GUEST_FONT_BUDGET_BYTES)} budget; a face for the guest is cut in fonts/guest.css`,
    );
    failed = true;
  }
}
process.exit(failed ? 1 : 0);
