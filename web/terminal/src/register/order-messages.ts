// What the register says when the server refuses an order command. The API's
// `message` is for developers ("db: in transaction: projecting ..."); its
// `code` is the contract, so the till's words come from the code.

import { ApiError } from "../../../shared/api";
import type { ErrorCode } from "../../../shared/api-types";

const words: Partial<Record<ErrorCode, string>> = {
  forbidden: "You don't have permission to do that. Ask a manager.",
  order_not_found: "This ticket isn't on the server any more.",
  order_closed: "This ticket is closed.",
  line_not_found: "That line isn't on this ticket.",
  line_not_active: "That line has been voided or comped.",
  modifiers_mismatch: "This line was changed on another till. Close this and look again.",
  qty_mismatch: "This line was changed on another till. Look again and retry.",
  line_in_kitchen: "This has gone to the kitchen. Void it and add it again.",
  venue_not_configured: "This venue's tax settings aren't finished. Ask a manager.",
  rate_limited: "Too many requests. Wait a moment and retry.",
  internal: "The server hit a problem. Try again.",
};

/** One line for the till: what went wrong, never the server's internals. */
export function failureOf(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.unreachable) return "Not sent: the server can't be reached.";
    const said = (words as Partial<Record<string, string>>)[err.code];
    if (said) return said;
    return "The server refused this. Check the ticket and try again.";
  }
  return "Not sent: something went wrong.";
}
