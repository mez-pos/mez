// The arithmetic behind <Qty>'s buttons, apart from the component so the
// edges are tested without rendering.

export interface QtyBounds {
  readonly min: number;
  readonly max: number;
}

/** A default a line can take: at least one, and more than any real round. */
export const defaultQtyBounds: QtyBounds = { min: 1, max: 999 };

/** The next quantity after a step, held inside the bounds. */
export function stepQty(value: number, step: 1 | -1, bounds: QtyBounds = defaultQtyBounds): number {
  return Math.min(bounds.max, Math.max(bounds.min, Math.trunc(value) + step));
}

export function canStep(value: number, step: 1 | -1, bounds: QtyBounds = defaultQtyBounds): boolean {
  return stepQty(value, step, bounds) !== Math.trunc(value);
}
