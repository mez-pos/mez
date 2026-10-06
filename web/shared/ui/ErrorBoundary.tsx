// The last stop for an error a screen did not catch: instead of a blank
// page, a plain card that says so and offers a reload. A reload is the right
// answer on a till: the server holds every order, so nothing sent is lost.
//
// Imported by path, not through ./index, so the guest bundle takes only this
// and the button.

import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "./Button";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  failed: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("A screen failed to draw", error, info.componentStack);
  }

  override render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="flex min-h-dvh items-center justify-center bg-canvas p-6 text-fg">
        <section role="alert" className="flex w-full max-w-md flex-col gap-3 rounded-card border border-line bg-raised p-6">
          <h1 className="font-serif text-xl">Something went wrong on this screen</h1>
          <p className="text-base text-fg-2">Reload to carry on. Everything the server has recorded is kept.</p>
          <Button variant="primary" onClick={() => window.location.reload()} className="mt-2 self-start">
            Reload
          </Button>
        </section>
      </main>
    );
  }
}
