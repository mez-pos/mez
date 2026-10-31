// Where an error a screen did not catch stops (TECH-8, ADR-402). Around the
// whole app it is the last stop: a plain card and a reload. Around a route,
// the order rail, the pay screen or a window's body, it stops there, so the
// rest of the screen, the header and the way out still work, and it offers
// Try again before a reload. Either way it tells the server's log
// (error-report.ts), so the failure reaches whoever looks after mez instead
// of a browser console nobody at the venue opens.
//
// A reload is always safe on a till: the server holds every order, so
// nothing sent is lost.
//
// Imported by path, not through ./index, so the guest bundle takes only this
// and the button.

import { Component, type ErrorInfo, type ReactNode } from "react";
import { fallbackWords, reportRenderError, type BoundaryPlace, type FallbackSize } from "../error-report";
import { Button } from "./Button";
import { cx } from "./cx";

interface ErrorBoundaryProps {
  children: ReactNode;
  /** Which boundary this is, in the report. */
  where?: BoundaryPlace;
  /** How much of the screen the fallback takes: the page (around the app), a region, or a window's body. */
  size?: FallbackSize;
  /** When it changes, a failed boundary draws its children again: the route, so moving on clears it. */
  resetKey?: string;
  /** The place's own line, in place of the size's: the pay screen says what is kept. */
  line?: string;
  /** A window's Close, offered beside Try again. */
  onClose?: () => void;
  /** Done before Try again draws again: reading the data afresh (use-fresh-retry.ts). A failure here still draws again. */
  onRetry?: () => Promise<unknown>;
  /** The fallback's own frame, where it stands in for something with one: the order rail's column. */
  frame?: string;
}

interface ErrorBoundaryState {
  failed: boolean;
  /** Try again is reading the data afresh. */
  retrying?: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("A screen failed to draw", error, info.componentStack);
    reportRenderError(this.props.where ?? "root", error, info.componentStack);
  }

  override componentDidUpdate(prev: ErrorBoundaryProps) {
    if (this.state.failed && prev.resetKey !== this.props.resetKey) this.setState({ failed: false });
  }

  private readonly again = async () => {
    this.setState({ retrying: true });
    try {
      await this.props.onRetry?.();
    } catch {
      // Drawn again all the same: the fallback comes back if it still fails.
    }
    this.setState({ failed: false, retrying: false });
  };

  override render() {
    if (!this.state.failed) return this.props.children;
    const size = this.props.size ?? "page";
    const words = fallbackWords(size, this.props.line);
    const reload = () => window.location.reload();

    if (size === "page") {
      return (
        <main className="flex min-h-dvh items-center justify-center bg-canvas p-6 text-fg">
          <section role="alert" className="flex w-full max-w-md flex-col gap-3 rounded-card border border-line bg-raised p-6">
            <h1 className="font-serif text-xl">{words.title}</h1>
            <p className="text-base text-fg-2">{words.line}</p>
            <Button variant="primary" onClick={reload} className="mt-2 self-start">
              Reload
            </Button>
          </section>
        </main>
      );
    }
    const frame = this.props.frame ?? (size === "region" ? "m-6 max-w-md self-start rounded-card border border-line bg-raised p-6" : "px-6 py-4");
    // Inside a window, under the window's own title.
    const Heading = size === "modal" ? "h3" : "h2";
    return (
      <section role="alert" className={cx("flex flex-col gap-3 text-fg", frame)}>
        <Heading className="font-serif text-xl">{words.title}</Heading>
        <p className="text-base text-fg-2">{words.line}</p>
        <div className="mt-2 flex flex-wrap gap-tap-gap">
          <Button variant="primary" onClick={() => void this.again()} disabled={this.state.retrying}>
            {this.state.retrying ? "Trying again" : "Try again"}
          </Button>
          {size === "modal" && this.props.onClose ? <Button onClick={this.props.onClose}>Close</Button> : <Button onClick={reload}>Reload</Button>}
        </div>
      </section>
    );
  }
}
