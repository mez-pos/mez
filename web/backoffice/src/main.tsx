import { QueryClientProvider } from "@tanstack/react-query";
import { LucideProvider } from "lucide-react";
import { StrictMode } from "react";
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

const queryClient = createQueryClient();
const { store, realtime } = createAppAuth("backoffice");

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RealtimeProvider connection={realtime}>
        <AuthProvider store={store} realtime={realtime}>
          {/* The design's line icons: 1.75px strokes (tokens.md, Icons). */}
          <LucideProvider strokeWidth={1.75}>
            <ToastProvider>
              <App />
            </ToastProvider>
          </LucideProvider>
        </AuthProvider>
      </RealtimeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
