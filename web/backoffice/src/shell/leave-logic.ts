// The words and the bookkeeping of the back office's leave guard
// (leave-guard.tsx), apart from React so they are tested directly.

/** The drafts open now: a key per form that holds one, and what it is, for the question. */
export type Drafts = ReadonlyMap<symbol, string>;

export const noDrafts: Drafts = new Map();

/**
 * The drafts once a form says it holds one (what) or no longer does (null).
 * The same map when nothing changed, so a form that says so again re-renders
 * nothing.
 */
export function holdDraft(drafts: Drafts, key: symbol, what: string | null): Drafts {
  if (what === null) {
    if (!drafts.has(key)) return drafts;
    const next = new Map(drafts);
    next.delete(key);
    return next;
  }
  if (drafts.get(key) === what) return drafts;
  return new Map(drafts).set(key, what);
}

/** "Your changes to the venue's details are not saved.", naming each draft once, in the order they began. */
export function leaveText(drafts: Drafts): string {
  const whats = [...new Set(drafts.values())];
  if (whats.length === 0) return "Your changes are not saved.";
  const list = whats.length === 1 ? whats[0] : `${whats.slice(0, -1).join(", ")} and ${whats[whats.length - 1]}`;
  return `Your changes to ${list} are not saved.`;
}

/** A click that opens the link in another tab or window: it leaves nothing here, so nothing is asked. */
export function opensElsewhere(e: { metaKey: boolean; ctrlKey: boolean; shiftKey: boolean; button: number }): boolean {
  return e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0;
}
