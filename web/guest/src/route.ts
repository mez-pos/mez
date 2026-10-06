// Where the guest is, read from the address. There is no router: the app has
// a code in its path and one of six screens in its hash.
//
// The path is /order/t/<code> on the venue's wifi and /t/<code> (or the same
// /order/t/<code>) through the relay, so the code is read from the end and
// the front is not looked at (ADR-206). A venue's pickup code, for ordering
// to collect with no table, has "p" where a table's has "t" (ADR-247).

/** A table code: twelve of the alphabet internal/service/tablecodes draws from. */
const CODE = /^[2-9a-hjkmnp-z]{12}$/;

/** The code a path ends in, and which kind the path says it is. */
export interface Way {
  readonly code: string;
  /** The venue's pickup code: ordering to collect, with no table. */
  readonly collect: boolean;
}

/** The code from a path, or null when the path does not end in one. */
export function wayFrom(pathname: string): Way | null {
  const segments = pathname.split("/").filter((s) => s !== "");
  const code = segments[segments.length - 1];
  const marker = segments[segments.length - 2];
  if ((marker !== "t" && marker !== "p") || code === undefined || !CODE.test(code)) return null;
  return { code, collect: marker === "p" };
}

/** The table's code from a path, or null when the path does not end in one. */
export function tableCodeFrom(pathname: string): string | null {
  const way = wayFrom(pathname);
  return way === null || way.collect ? null : way.code;
}

export type View = "landing" | "menu" | "cart" | "order" | "bill" | "receipt";

const hashes: Readonly<Record<View, string>> = { landing: "", menu: "#menu", cart: "#cart", order: "#order", bill: "#bill", receipt: "#receipt" };

/** The screen a hash names. Anything that is not one of the other five's is the landing. */
export function viewFrom(hash: string): View {
  if (hash === hashes.menu) return "menu";
  if (hash === hashes.cart) return "cart";
  if (hash === hashes.order) return "order";
  if (hash === hashes.bill) return "bill";
  if (hash === hashes.receipt) return "receipt";
  return "landing";
}

/** The hash for a screen. The landing has none, so Back from the menu returns to it. */
export function hashFor(view: View): string {
  return hashes[view];
}
