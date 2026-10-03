// One Vite config for all three apps. An app differs from the others only in
// where the Go server mounts it, which port it uses in development, and which
// runtime renders it; the rest must not drift, so it lives here.

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv, type PluginOption, type UserConfig } from "vite";

// The server `make run` starts. Every API call and the WebSocket stream go
// through the dev proxy to it, so dev and production use the same origin-
// relative paths and nothing in the apps knows which one it is in.
// MEZ_DEV_SERVER points the proxy at a server on another port, for a dev
// venue running beside a real one.
const serverOrigin = loadEnv("development", ".", "MEZ_").MEZ_DEV_SERVER ?? "http://localhost:7777";

// Guest renders through preact/compat: the same React source and the same
// React types, about 40 KB less on a 3G phone. Only the bundler knows.
// ADR-064. The order matters: the longest specifier must match first.
const preactAliases = [
  { find: /^react-dom\/client$/, replacement: "preact/compat/client" },
  { find: /^react-dom\/test-utils$/, replacement: "preact/test-utils" },
  { find: /^react-dom$/, replacement: "preact/compat" },
  { find: /^react\/jsx-runtime$/, replacement: "preact/jsx-runtime" },
  { find: /^react\/jsx-dev-runtime$/, replacement: "preact/jsx-dev-runtime" },
  { find: /^react$/, replacement: "preact/compat" },
];

type App = {
  // Where internal/api mounts the bundle, from web/web.go. Vite writes asset
  // URLs relative to it, so a mismatch serves index.html for every script.
  base: "/" | "/backoffice/" | "/order/";
  port: number;
  runtime?: "react" | "preact";
};

export function appConfig({ base, port, runtime = "react" }: App): UserConfig {
  const preact = runtime === "preact";
  // plugin-react injects React Refresh, which Preact cannot run, so guest
  // gets Vite's own JSX transform and a full reload on edit in development.
  const plugins: PluginOption[] = preact ? [tailwindcss()] : [react(), tailwindcss()];

  return defineConfig({
    base,
    plugins,
    resolve: preact ? { alias: preactAliases } : {},
    oxc: preact ? { jsx: { runtime: "automatic", importSource: "react" } } : {},
    build: {
      outDir: "dist",
      emptyOutDir: true,
      // Hashed files under assets/: internal/api/static.go caches exactly
      // that directory as immutable.
      assetsDir: "assets",
      rolldownOptions: {
        // React Query marks files "use client" for server-component
        // frameworks. A single-page app has no server components, so the
        // directive means nothing here, and the warnings would bury real ones.
        onwarn(warning, warn) {
          if (warning.code === "MODULE_LEVEL_DIRECTIVE" && warning.message.includes('"use client"')) return;
          warn(warning);
        },
      },
    },
    server: {
      port,
      strictPort: true,
      proxy: {
        "/api": { target: serverOrigin, changeOrigin: false, ws: true },
      },
    },
  });
}
