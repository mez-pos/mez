// Where the guest is, read from the address. There is no router: the app has
// a table's code in its path and one of two screens in its hash.
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

export type View = "landing" | "menu";

const MENU_HASH = "#menu";

/** The screen a hash names. Anything but the menu's is the landing. */
export function viewFrom(hash: string): View {
  return hash === MENU_HASH ? "menu" : "landing";
}

/** The hash for a screen. The landing has none, so Back from the menu returns to it. */
export function hashFor(view: View): string {
  return view === "menu" ? MENU_HASH : "";
}
