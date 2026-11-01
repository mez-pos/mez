import { QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { LucideProvider } from "lucide-react";
import { createRoot } from "react-dom/client";
import { createAppAuth } from "../../shared/auth/app-auth";
import { browserStorage } from "../../shared/auth/storage";
import { AuthProvider } from "../../shared/auth/use-auth";
import { setErrorReporter } from "../../shared/error-report";
import { createQueryClient } from "../../shared/query";
import { ErrorBoundary, ToastProvider } from "../../shared/ui";
import { RealtimeProvider } from "../../shared/use-realtime";
import { Splash } from "../../shared/screens/Splash";
import { readPrefs, surfaceOf } from "./kitchen/board-prefs";
import { readStation, stationKey } from "./kitchen/kitchen-logic";
import { lazyScreen } from "./shell/lazy-screen";
import "./index.css";
import { stroke } from "../../shared/ui/stroke";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// The component gallery, in development only. import.meta.env.DEV is false in
// a production build, so this branch and the lazy import both disappear.
const Gallery = import.meta.env.DEV && window.location.pathname === "/gallery" ? lazy(() => import("./gallery/Gallery")) : null;

// The till and the kitchen display are fetched apart (TECH-25, ADR-408): a
// device parses only the one it is, and the kitchen's screen never parses the
// till's.
const App = lazyScreen("App", () => import("./App").then((m) => m.App));
const KitchenApp = lazyScreen("KitchenApp", () => import("./kitchen/KitchenApp").then((m) => m.KitchenApp));

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
// A screen that fails to draw is told to the server's log, as this device (TECH-8, ADR-402).
// Not before it is paired: the server would refuse it, and the refusal would
// read as this device being forgotten.
setErrorReporter(kitchen ? "kitchen" : "terminal", (report) =>
  store.credentials.deviceToken() ? store.api.post("/api/v1/client-errors", report) : Promise.resolve(),
);

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
            <LucideProvider strokeWidth={stroke.base}>
              <ToastProvider placement="top">
                {/* While the device's own app is fetched: the splash the Gate shows next, so the boot reads as one screen. */}
                <Suspense fallback={<Splash />}>
                  <Screen />
                </Suspense>
              </ToastProvider>
            </LucideProvider>
          </AuthProvider>
        </RealtimeProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
