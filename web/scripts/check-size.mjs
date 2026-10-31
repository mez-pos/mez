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
// jump is visible in the build log.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
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

let failed = false;
for (const app of ["terminal", "backoffice", "guest"]) {
  const { counted, fonts, other, total, fontTotal } = measure(app);
  if (app !== "guest") {
    console.log(`  size  ${app.padEnd(10)} ${kb(total)} gzipped (no budget)`);
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
