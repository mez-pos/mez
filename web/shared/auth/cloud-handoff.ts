// A session mez Cloud hands the back office (ADR-355). The cloud signs the
// owner in, at the venue through the relay or at the venue's mirror, and is
// answered a token that stands for both the device and the session
// (ADR-324). It opens the back office at the venue's own hostname with that
// token in the address's fragment, which no server is sent:
//
//   https://corner-cafe.mezpos.com/backoffice/#cloud=<token>&back=<url>
//
// `back` is the account area's page for the venue, where the owner goes to
// open the back office again once this session ends. It is taken only on
// the venues' own domain, the page's hostname less its first label (a
// venue is one label under it, ADR-203), so a link cannot send the owner
// anywhere else; and it is shown only for a session the server confirmed.

export interface CloudHandoff {
  readonly token: string;
  readonly back: string | null;
}

/** A session token as mez makes them: a short prefix and 256 bits. */
const tokenShape = /^[a-z]{3,5}_[A-Za-z0-9_-]{43}$/;

const loopbackAddresses = new Set(["localhost", "127.0.0.1", "[::1]"]);

/**
 * A name that is this machine: a loopback address, or localhost or any name
 * under it, which a browser never asks anyone to resolve (RFC 6761). A
 * developer's venues are at names under it (mez Cloud's walk, ADR-355).
 */
function loopback(hostname: string): boolean {
  const h = hostname.toLowerCase().replace(/\.$/, "");
  return loopbackAddresses.has(h) || h.endsWith(".localhost");
}

/** Where this page is, as much of it as the checks need. */
export interface Here {
  readonly protocol: string;
  readonly hostname: string;
}

/** The handoff in an address's fragment, or null when there is none or it is not one. */
export function readHandoff(hash: string, here: Here): CloudHandoff | null {
  const params = new URLSearchParams(hash.replace(/^#/, ""));
  const token = params.get("cloud");
  if (!token || !tokenShape.test(token)) return null;
  return { token, back: safeBack(params.get("back"), here) };
}

/**
 * `back` if it may be followed from here: https on the venues' domain or a
 * name under it, never an address by number, or http between two loopback
 * names on a developer's machine.
 */
export function safeBack(raw: string | null, here: Here): string | null {
  if (!raw) return null;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return null;
  }
  if (url.username || url.password) return null;
  if (loopback(here.hostname)) {
    return (url.protocol === "http:" || url.protocol === "https:") && loopback(url.hostname) ? url.href : null;
  }
  if (url.protocol !== "https:" || here.protocol !== "https:" || numeric(url.hostname)) return null;
  const domain = venuesDomain(here.hostname);
  if (domain === null) return null;
  return url.hostname === domain || url.hostname.endsWith(`.${domain}`) ? url.href : null;
}

/** The venues' domain: this page's hostname less its first label, if two are left. */
function venuesDomain(hostname: string): string | null {
  if (numeric(hostname)) return null;
  const labels = hostname.toLowerCase().replace(/\.$/, "").split(".");
  return labels.length >= 3 ? labels.slice(1).join(".") : null;
}

/** An address by number: IPv4 as the URL parser writes it, or IPv6 in brackets. */
function numeric(hostname: string): boolean {
  return hostname.startsWith("[") || /^[0-9.]+$/.test(hostname);
}
