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
  needs_approval: "This needs a manager now. Use Void.",
  order_held: "This ticket is on hold. Resume it to carry on.",
  order_not_held: "This ticket isn't on hold.",
  venue_not_configured: "This venue's tax settings aren't finished. Ask a manager.",
  over_tender: "That's more than is owed now. Look at the amount due again.",
  order_invoiced: "This bill is invoiced. Close it and start a new ticket.",
  invoice_not_found: "That invoice isn't on this ticket.",
  print_job_not_found: "That print job isn't on this venue's queue.",
  print_job_state: "That job has moved on. Look at the queue again.",
  no_printer: "There is still no printer set up for this. Ask a manager to set one up.",
  not_invoiced: "This bill has no invoice yet. Void or comp the item instead.",
  over_credit: "That's more than is left to credit on this bill.",
  over_refund: "That payment has less left to refund than this. Choose another payment, or refund less.",
  refunds_mismatch:"The refunds must add up to the credit exactly.",
  split_invalid: "That split doesn't work. Check the number of payers.",
  split_over_allocated: "Those amounts come to more than is owed.",
  split_unassigned: "Every item needs someone to pay for it.",
  split_mismatch: "The split doesn't add up to the bill. Split it again.",
  split_after_payment: "Money has been taken, so split what's left evenly or by amount.",
  table_not_found: "That table isn't on the floor any more. Look at the floor again.",
  table_occupied: "Guests are still at this table: settle or move their ticket first.",
  table_not_empty: "Something is on this table's ticket: void it or take the payment first.",
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

/**
 * Words for a refused manager approval. `err` is what TicketQueue.run
 * rejected with; the server's error is its cause. `who` is the name chosen.
 */
export function approvalFailure(err: unknown, who: string): string {
  const cause = err instanceof Error && err.cause instanceof ApiError ? err.cause : err;
  if (cause instanceof ApiError) {
    if (cause.code === "invalid_pin") return "That PIN isn't right. Try again.";
    if (cause.code === "forbidden") return `${who} can't approve this. Choose a manager.`;
    if (cause.code === "locked_out") return "Too many wrong PINs. Try again in a few minutes.";
    return failureOf(cause);
  }
  return err instanceof Error ? err.message : failureOf(err);
}
