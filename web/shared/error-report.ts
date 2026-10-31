// A screen that failed to draw, caught by the error boundary around the part
// that failed and told to the server's log (TECH-8, ADR-402). No React here,
// so all of it is tested directly.
//
// An app says once, at its start, how to send a report (setErrorReporter);
// every boundary in it then reports what it catches. The guest's app sets
// none: it has no device token to send one with, and its boundary only
// offers a reload.

import type { ClientErrorRequest } from "./api-types";

/** The bundles that report, as the server knows them. */
export type ReportApp = "terminal" | "kitchen" | "backoffice";

/** Which boundary caught it: the whole app, a route, the order rail, the pay screen, or a window. */
export type BoundaryPlace = "root" | "route" | "rail" | "pay" | "modal";

/** The most reports one page load sends: a screen that fails on every Try again is told a few times, not without end. */
export const MAX_REPORTS = 10;

// The server keeps no more than these; sending more is only bytes.
const MAX_PATH = 200;
const MAX_TEXT = 500;
const MAX_STACK = 4000;

const cut = (s: string, n: number): string => (s.length > n ? s.slice(0, n) : s);

/**
 * The report for one error, from what React hands a boundary. The address is
 * its path alone: a query or a fragment can carry a pairing code or a token
 * handed over by mez Cloud, and neither belongs in a log.
 */
export function reportOf(app: ReportApp, where: BoundaryPlace, error: unknown, componentStack: string | null | undefined, path: string): ClientErrorRequest {
  const e = error instanceof Error ? error : null;
  return {
    app,
    where,
    path: cut(path.split(/[?#]/)[0] ?? "", MAX_PATH),
    message: cut(e ? e.message : String(error), MAX_TEXT),
    stack: cut(e?.stack ?? "", MAX_STACK),
    component_stack: cut(componentStack ?? "", MAX_STACK),
  };
}

/** Lets each different failure through once, and no more than MAX_REPORTS in all. */
export class ReportGate {
  private readonly sent = new Set<string>();

  allow(report: Pick<ClientErrorRequest, "where" | "path" | "message">): boolean {
    const key = `${report.where}\u0000${report.path}\u0000${report.message}`;
    if (this.sent.has(key) || this.sent.size >= MAX_REPORTS) return false;
    this.sent.add(key);
    return true;
  }
}

type Send = (report: ClientErrorRequest) => Promise<unknown>;

let reporter: { app: ReportApp; send: Send; gate: ReportGate } | null = null;

/** Where an app's boundaries send what they catch: its API client, with the device's token. */
export function setErrorReporter(app: ReportApp, send: Send): void {
  reporter = { app, send, gate: new ReportGate() };
}

/**
 * Tells the server, if the app said how. It never throws and never waits: a
 * report that cannot be sent, the server out of reach or the device not yet
 * paired, is dropped, and the screen's own fallback is what the person sees.
 */
export function reportRenderError(where: BoundaryPlace, error: unknown, componentStack: string | null | undefined): void {
  if (!reporter) return;
  try {
    const report = reportOf(reporter.app, where, error, componentStack, window.location.pathname);
    if (!reporter.gate.allow(report)) return;
    reporter.send(report).catch(() => undefined);
  } catch {
    // Reporting is never a second failure.
  }
}

/** How much of the screen a boundary's fallback takes. */
export type FallbackSize = "page" | "region" | "modal";

/** What a fallback says. `line` is a place's own words: the pay screen says what is kept. */
export function fallbackWords(size: FallbackSize, line?: string): { title: string; line: string } {
  switch (size) {
    case "page":
      return { title: "Something went wrong on this screen", line: line ?? "Reload to carry on. Everything the server has recorded is kept." };
    case "region":
      return {
        title: "This part of the screen failed to draw",
        line: line ?? "Try again, or reload if it fails again. Everything the server has recorded is kept.",
      };
    case "modal":
      return { title: "This window failed to draw", line: line ?? "Try again, or close it and open it again. Anything already saved is kept." };
  }
}
