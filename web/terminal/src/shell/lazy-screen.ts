// A screen whose code is fetched when it is first wanted, not when the
// terminal boots (TECH-25, ADR-408). The kitchen display and the till each
// parse only their own screens, and the till fetches the rest while it is
// idle (`warm`), so a screen opened during service is already in memory and
// draws at once, as it did when the terminal was one file. Code in memory
// also cannot go missing: after mez is upgraded, the old build's files are
// gone from the server, and a till left open would ask for them.
//
// Why not React.lazy alone: a lazy component suspends on its first draw even
// when its code is already here, which would flash the fallback over the pay
// screen; here a loaded screen draws directly.
//
// A fetch that fails while the screen is being drawn is held as failed for a
// moment (`retryAfterMs`), then let go. Held, the route boundary shows the
// error: React draws again as soon as the fetch fails, and a fresh lazy in
// that draw would fetch again, and again, and the error would never be
// shown, only an empty screen (seen on the walk, ADR-408). Let go after, so
// the boundary's Try again, or coming back to the route, is a new fetch;
// after an upgrade only Reload helps, as only a reload knows the new files'
// names. A failed warm is not held at all: the screen has not been drawn.

import { createElement, lazy, type FunctionComponent } from "react";

/** Anything that can be fetched ahead of its first draw. */
export interface Preloadable {
  /** Fetches the code, once; a warm that failed is tried again by the next call. */
  preload(): Promise<unknown>;
}

export type LazyScreen<P extends object> = FunctionComponent<P> & Preloadable;

export function lazyScreen<P extends object>(name: string, load: () => Promise<FunctionComponent<P>>): LazyScreen<P> {
  let loaded: FunctionComponent<P> | null = null;
  let pending: Promise<FunctionComponent<P>> | null = null;

  const preload = (): Promise<FunctionComponent<P>> => {
    pending ??= load().then(
      (screen) => (loaded = screen),
      (err: unknown) => {
        pending = null;
        throw err;
      },
    );
    return pending;
  };

  // Drawn only until the code is here. React.lazy's type adds ref handling
  // for a component that takes one; a screen takes none, and the lazy draws
  // it with the same props. Replaced a moment after it fails, not at once.
  const fresh = () =>
    lazy(() =>
      preload().then(
        (screen) => ({ default: screen }),
        (err: unknown) => {
          setTimeout(() => {
            cold = fresh();
          }, retryAfterMs);
          throw err;
        },
      ),
    ) as unknown as FunctionComponent<P>;
  let cold = fresh();

  const Screen = (props: P) => createElement(loaded ?? cold, props);
  Screen.displayName = name;
  return Object.assign(Screen, { preload });
}

/**
 * Fetches the screens once the browser is idle, so the screen that is up
 * draws first. A fetch that fails here is not an error yet: the screen fetches
 * again when it is opened, and its route boundary says so if that fails too.
 * Returns the cancel, for an effect's cleanup.
 */
export function warm(screens: readonly Preloadable[]): () => void {
  const run = () => {
    for (const screen of screens) screen.preload().catch(() => undefined);
  };
  if (typeof requestIdleCallback === "function") {
    const id = requestIdleCallback(run, { timeout: warmWithinMs });
    return () => cancelIdleCallback(id);
  }
  const id = setTimeout(run, warmFallbackMs);
  return () => clearTimeout(id);
}

/**
 * How long a fetch that failed as its screen was drawn stays failed: long
 * enough for React's own redraw to show the error, short of anyone's Try again.
 */
export const retryAfterMs = 1000;

/** The longest the warm waits for an idle moment: a till busy from the start still gets its screens. */
const warmWithinMs = 2000;
/** Where there is no idle callback (older Safari): soon after the first screen is drawn. */
const warmFallbackMs = 300;
