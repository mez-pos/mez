import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { devCredentials } from "../../shared/credentials";
import { createQueryClient } from "../../shared/query";
import { createRealtime, RealtimeProvider } from "../../shared/use-realtime";
import { App } from "./App";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

const queryClient = createQueryClient();
const realtime = createRealtime(devCredentials);

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RealtimeProvider connection={realtime}>
        <App />
      </RealtimeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
