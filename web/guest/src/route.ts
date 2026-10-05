// Where the guest is, read from the address. There is no router: the app has
// a table's code in its path and one of four screens in its hash.
//
// The path is /order/t/<code> on the venue's wifi and /t/<code> (or the same
// /order/t/<code>) through the relay, so the code is read from the end and
// the front is not looked at (ADR-206).

/** A table code: twelve of the alphabet internal/service/tablecodes draws from. */
const CODE = /^[2-9a-hjkmnp-z]{12}$/;

/** The table's code from a path, or null when the path does not end in one. */
export function tableCodeFrom(pathname: string): string | null {
  const segments = pathname.split("/").filter((s) => s !== "");
  const code = segments[segments.length - 1];
  const marker = segments[segments.length - 2];
  if (marker !== "t" || code === undefined || !CODE.test(code)) return null;
  return code;
}

export type View = "landing" | "menu" | "cart" | "order";

const hashes: Readonly<Record<View, string>> = { landing: "", menu: "#menu", cart: "#cart", order: "#order" };

/** The screen a hash names. Anything that is not the menu's, the cart's or the order's is the landing. */
export function viewFrom(hash: string): View {
  if (hash === hashes.menu) return "menu";
  if (hash === hashes.cart) return "cart";
  if (hash === hashes.order) return "order";
  return "landing";
}

/** The hash for a screen. The landing has none, so Back from the menu returns to it. */
export function hashFor(view: View): string {
  return hashes[view];
}
