import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createQueryClient } from "../../shared/query";
import { ErrorBoundary } from "../../shared/ui/ErrorBoundary";
import { App } from "./App";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// No realtime here: a guest's phone is not a paired device and holds no
// stream. What it waits on (the menu, the bill, an order, a payment) it asks
// for again on a timer.
const queryClient = createQueryClient();

createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
