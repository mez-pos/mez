import { defineConfig } from "vitest/config";

// Logic only (docs/03-conventions.md): no DOM, no screen snapshots.
export default defineConfig({
  test: {
    environment: "node",
    include: ["shared/**/*.test.ts", "*/src/**/*.test.ts"],
  },
});
