import { QueryClientProvider } from "@tanstack/react-query";
import { LucideProvider } from "lucide-react";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createAppAuth } from "../../shared/auth/app-auth";
import { AuthProvider } from "../../shared/auth/use-auth";
import { createQueryClient } from "../../shared/query";
import { ErrorBoundary, ToastProvider } from "../../shared/ui";
import { MirrorProvider } from "../../shared/use-mirror";
import { RealtimeProvider } from "../../shared/use-realtime";
import { App } from "./App";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

const queryClient = createQueryClient();
const { store, realtime, mirror } = createAppAuth("backoffice");

createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <RealtimeProvider connection={realtime}>
          <AuthProvider store={store} realtime={realtime}>
            <MirrorProvider watch={mirror}>
              {/* The design's line icons: 1.75px strokes (tokens.md, Icons). */}
              <LucideProvider strokeWidth={1.75}>
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
