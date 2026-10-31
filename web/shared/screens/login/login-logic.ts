// The sign-in screen's profile tiles (D01): as many to a page as one row holds,
// and "More" for the rest. Paging is a function of the list, the page and the
// row's width, so it is tested without rendering anything.

/**
 * How many profiles show at once where the row cannot say: before it is
 * measured, or too narrow for a profile, Back and More side by side (a phone
 * held upright). The design draws four and a More; there the row wraps.
 */
export const defaultPerPage = 4;

/** How many slots `slot` px wide, `gap` px apart, fit across `width` px. */
export function slotsAcross(width: number, slot: number, gap: number): number {
  if (!(width > 0) || !(slot > 0)) return 0;
  const between = gap > 0 ? gap : 0;
  return Math.floor((width + between) / (slot + between));
}

/**
 * How many profiles a page shows, so that they and the page's Back and More
 * share one row of `slots` (ADR-255: no fixed count). Everyone, when everyone
 * fits. Over two pages a page has More or Back, never both, so one slot is
 * kept for it; over more than two, a middle page has both, so two are kept.
 */
export function perPage(slots: number, count: number): number {
  if (slots < 3) return defaultPerPage;
  if (count <= slots) return slots;
  const besideOne = slots - 1;
  return Math.ceil(count / besideOne) <= 2 ? besideOne : slots - 2;
}

export interface TilePage<T> {
  /** The profiles to show, in the list's order. */
  shown: readonly T[];
  /** Whether More is offered: there is more than a page, and it would not only repeat Back. */
  hasMore: boolean;
  /** Which page (from 0) More goes to: the next, or round to the first. */
  next: number;
  /** Which page Back goes to: the one before, or null on the first. */
  prev: number | null;
  /** The page shown, kept in range when the list shrank under it. */
  page: number;
  /** How many pages there are. */
  pages: number;
}

export function tilePage<T>(people: readonly T[], page: number, size = defaultPerPage): TilePage<T> {
  const pages = Math.max(1, Math.ceil(people.length / size));
  const at = Math.min(Math.max(0, Math.trunc(page) || 0), pages - 1);
  const next = (at + 1) % pages;
  const prev = at > 0 ? at - 1 : null;
  return {
    shown: people.slice(at * size, at * size + size),
    // On the second of two pages, More would go round to the page Back goes to.
    hasMore: pages > 1 && next !== prev,
    next,
    prev,
    page: at,
    pages,
  };
}

/** The page a person is on, so whoever is chosen stays in view when the page size changes. */
export function pageOf<T extends { id: string }>(people: readonly T[], id: string, size = defaultPerPage): number {
  const i = people.findIndex((p) => p.id === id);
  return i < 0 ? 0 : Math.floor(i / size);
}
