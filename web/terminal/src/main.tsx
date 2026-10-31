import { QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { LucideProvider } from "lucide-react";
import { createRoot } from "react-dom/client";
import { createAppAuth } from "../../shared/auth/app-auth";
import { browserStorage } from "../../shared/auth/storage";
import { AuthProvider } from "../../shared/auth/use-auth";
import { createQueryClient } from "../../shared/query";
import { ErrorBoundary, ToastProvider } from "../../shared/ui";
import { RealtimeProvider } from "../../shared/use-realtime";
import { App } from "./App";
import { KitchenApp } from "./kitchen/KitchenApp";
import { readPrefs, surfaceOf } from "./kitchen/board-prefs";
import { readStation, stationKey } from "./kitchen/kitchen-logic";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// The component gallery, in development only. import.meta.env.DEV is false in
// a production build, so this branch and the lazy import both disappear.
const Gallery = import.meta.env.DEV && window.location.pathname === "/gallery" ? lazy(() => import("./gallery/Gallery")) : null;

const queryClient = createQueryClient();

// The kitchen display is served from this bundle at /kitchen (ADR-091): its
// own device, paired as kitchen, listening to its station alone.
const kitchen = window.location.pathname === "/kitchen" || window.location.pathname.startsWith("/kitchen/");
// The kitchen's board opens on espresso night (D11), the ink surface tokens.md
// gives the kitchen; paper is a choice kept on the device (ADR-381, over
// ADR-177). Set here, before anything is drawn, so a dark pass is never lit
// by a flash.
const durable = browserStorage("local");
if (kitchen) document.documentElement.dataset.surface = surfaceOf(readPrefs(durable));
const station = kitchen ? readStation(durable) : null;
const { store, realtime } = kitchen ? createAppAuth("kitchen", station ? [`station:${station}`] : []) : createAppAuth("terminal");

/** Keeps the display's station and starts again, subscribed to it alone. */
function chooseStation(code: string | null) {
  try {
    if (code) durable?.setItem(stationKey, code);
    else durable?.removeItem(stationKey);
  } catch {
    // Storage blocked: the choice lasts until the page is reloaded.
  }
  window.location.reload();
}

function Screen() {
  if (kitchen) return <KitchenApp station={station} onStation={chooseStation} />;
  if (Gallery) {
    return (
      <Suspense>
        <Gallery />
      </Suspense>
    );
  }
  return <App />;
}

createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <RealtimeProvider connection={realtime}>
          <AuthProvider store={store} realtime={realtime}>
            {/* The design's line icons: 1.75px strokes (tokens.md, Icons). */}
            <LucideProvider strokeWidth={1.75}>
              <ToastProvider placement="top">
                <Screen />
              </ToastProvider>
            </LucideProvider>
          </AuthProvider>
        </RealtimeProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
