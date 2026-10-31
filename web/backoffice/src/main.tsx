import { QueryClientProvider } from "@tanstack/react-query";
import { LucideProvider } from "lucide-react";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createAppAuth } from "../../shared/auth/app-auth";
import { AuthProvider } from "../../shared/auth/use-auth";
import { setErrorReporter } from "../../shared/error-report";
import { createQueryClient } from "../../shared/query";
import { ErrorBoundary, ToastProvider } from "../../shared/ui";
import { MirrorProvider } from "../../shared/use-mirror";
import { RealtimeProvider } from "../../shared/use-realtime";
import { App } from "./App";
import "./index.css";
import { stroke } from "../../shared/ui/stroke";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

const queryClient = createQueryClient();
const { store, realtime, mirror } = createAppAuth("backoffice");
// A screen that fails to draw is told to the server's log, as this device (TECH-8, ADR-402).
// Not before it is paired: the server would refuse it, and the refusal would
// read as this device being forgotten.
setErrorReporter("backoffice", (report) => (store.credentials.deviceToken() ? store.api.post("/api/v1/client-errors", report) : Promise.resolve()));

createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <RealtimeProvider connection={realtime}>
          <AuthProvider store={store} realtime={realtime}>
            <MirrorProvider watch={mirror}>
              {/* The design's line icons: 1.75px strokes (tokens.md, Icons). */}
              <LucideProvider strokeWidth={stroke.base}>
                <ToastProvider>
                  <App />
                </ToastProvider>
              </LucideProvider>
            </MirrorProvider>
          </AuthProvider>
        </RealtimeProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
