import { QueryClientProvider } from "@tanstack/react-query";
import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { createAppAuth } from "../../shared/auth/app-auth";
import { AuthProvider } from "../../shared/auth/use-auth";
import { createQueryClient } from "../../shared/query";
import { ToastProvider } from "../../shared/ui";
import { RealtimeProvider } from "../../shared/use-realtime";
import { App } from "./App";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// The component gallery, in development only. import.meta.env.DEV is false in
// a production build, so this branch and the lazy import both disappear.
const Gallery = import.meta.env.DEV && window.location.pathname === "/gallery" ? lazy(() => import("./gallery/Gallery")) : null;

const queryClient = createQueryClient();
const { store, realtime } = createAppAuth("terminal");

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RealtimeProvider connection={realtime}>
        <AuthProvider store={store} realtime={realtime}>
          <ToastProvider>
            {Gallery ? (
              <Suspense>
                <Gallery />
              </Suspense>
            ) : (
              <App />
            )}
          </ToastProvider>
        </AuthProvider>
      </RealtimeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
