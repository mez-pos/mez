// The sign-in screen's profile tiles (D01): four to a page, and "More" for the
// rest. Paging is a function of the list and the page, so it is tested without
// rendering anything.

/** How many profiles show at once. The design draws four and a More. */
export const tilesPerPage = 4;

export interface TilePage<T> {
  /** The profiles to show, in the list's order. */
  shown: readonly T[];
  /** Whether there are more than a page, so More is offered. */
  hasMore: boolean;
  /** Which page (from 0) More goes to: the next, or round to the first. */
  next: number;
  /** The page shown, kept in range when the list shrank under it. */
  page: number;
  /** How many pages there are. */
  pages: number;
}

export function tilePage<T>(people: readonly T[], page: number, size = tilesPerPage): TilePage<T> {
  const pages = Math.max(1, Math.ceil(people.length / size));
  const at = Math.min(Math.max(0, Math.trunc(page) || 0), pages - 1);
  return {
    shown: people.slice(at * size, at * size + size),
    hasMore: pages > 1,
    next: (at + 1) % pages,
    page: at,
    pages,
  };
}

/** The page a person is on, so choosing someone by keyboard or by default shows them. */
export function pageOf<T extends { id: string }>(people: readonly T[], id: string, size = tilesPerPage): number {
  const i = people.findIndex((p) => p.id === id);
  return i < 0 ? 0 : Math.floor(i / size);
}
