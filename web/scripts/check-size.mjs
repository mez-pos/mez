// Fails the build when the guest bundle is over budget. See CLAUDE.md and
// web/README.md: the guest app loads on a stranger's phone on 3G.
//
// Counted: every .js, .css and .html file in guest/dist, gzipped at level 9,
// which is what a browser must fetch before the first screen can render.
// Not counted: fonts and images. They load in parallel, a face is fetched
// only if the page uses it, and `font-display: swap` means text renders
// without them. They are listed so nobody forgets they exist. ADR-063.
//
// The terminal and back office have no budget; their sizes are printed so a
// jump is visible in the build log.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const GUEST_BUDGET_BYTES = 60 * 1024;
const COUNTED = /\.(js|css|html)$/;
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
    .map((f) => ({ file: relative(dist, f), gz: gzipSync(readFileSync(f), { level: 9 }).length }));
  const other = all
    .filter((f) => !COUNTED.test(f))
    .map((f) => ({ file: relative(dist, f), raw: statSync(f).size }));
  return { counted, other, total: counted.reduce((sum, f) => sum + f.gz, 0) };
}

const kb = (bytes) => `${(bytes / 1024).toFixed(1)} KB`;

let failed = false;
for (const app of ["terminal", "backoffice", "guest"]) {
  const { counted, other, total } = measure(app);
  if (app !== "guest") {
    console.log(`  size  ${app.padEnd(10)} ${kb(total)} gzipped (no budget)`);
    continue;
  }
  const headroom = GUEST_BUDGET_BYTES - total;
  console.log(`  size  guest      ${kb(total)} gzipped of ${kb(GUEST_BUDGET_BYTES)} (${kb(headroom)} left)`);
  for (const f of counted) console.log(`          ${kb(f.gz).padStart(9)}  ${f.file}`);
  for (const f of other) console.log(`          ${kb(f.raw).padStart(9)}  ${f.file} (not counted)`);
  if (headroom < 0) {
    console.error(`  size  FAIL: guest bundle is ${kb(-headroom)} over its ${kb(GUEST_BUDGET_BYTES)} gzipped budget`);
    failed = true;
  }
}
process.exit(failed ? 1 : 0);
