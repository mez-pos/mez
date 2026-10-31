// A route's error boundary (TECH-8, ADR-402): a screen that fails to draw
// leaves the rest of the app standing, and going to another screen clears
// it. Outside the kit (shared/ui) because it reads the router.

import type { ReactNode } from "react";
import { useLocation } from "wouter";
import type { FallbackSize } from "./error-report";
import { ErrorBoundary } from "./ui/ErrorBoundary";
import { useFreshRetry } from "./use-fresh-retry";

interface RouteBoundaryProps {
  children: ReactNode;
  /** A region inside a shell, whose header and way out still work; the page where there is no shell. */
  size?: Exclude<FallbackSize, "modal">;
}

export function RouteBoundary({ children, size = "region" }: RouteBoundaryProps) {
  const [location] = useLocation();
  const retry = useFreshRetry();
  return (
    <ErrorBoundary where="route" size={size} resetKey={location} onRetry={retry}>
      {children}
    </ErrorBoundary>
  );
}
