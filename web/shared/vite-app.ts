// One Vite config for all three apps. An app differs from the others only in
// where the Go server mounts it and which port it uses in development; the
// rest must not drift, so it lives here.

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, type UserConfig } from "vite";

// The server `make run` starts. Every API call and the WebSocket stream go
// through the dev proxy to it, so dev and production use the same origin-
// relative paths and nothing in the apps knows which one it is in.
const serverOrigin = "http://localhost:7777";

type App = {
  // Where internal/api mounts the bundle, from web/web.go. Vite writes asset
  // URLs relative to it, so a mismatch serves index.html for every script.
  base: "/" | "/backoffice/" | "/order/";
  port: number;
};

export function appConfig({ base, port }: App): UserConfig {
  return defineConfig({
    base,
    plugins: [react(), tailwindcss()],
    build: {
      outDir: "dist",
      emptyOutDir: true,
      // Hashed files under assets/: internal/api/static.go caches exactly
      // that directory as immutable.
      assetsDir: "assets",
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
