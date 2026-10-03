import { defineConfig } from "vitest/config";

// Dates render in the browser's own zone, which at a venue is the venue's.
// Tests pin one so that "40 hours ago" is the same calendar day on a laptop
// in India and on a CI runner in UTC. India's +05:30 is the zone they were
// written in, and a half-hour offset catches more slips than UTC would.
// Set here, before the workers start, so each one inherits it.
process.env.TZ = "Asia/Kolkata";

// Logic only (docs/03-conventions.md): no DOM, no screen snapshots.
export default defineConfig({
  test: {
    environment: "node",
    include: ["shared/**/*.test.ts", "*/src/**/*.test.ts"],
  },
});
