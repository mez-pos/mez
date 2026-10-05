// What the register says when the server refuses an order command. The API's
// `message` is for developers ("db: in transaction: projecting ..."); its
// `code` is the contract, so the till's words come from the code.

import { ApiError } from "../../../shared/api";
import type { ErrorCode } from "../../../shared/api-types";

const words: Partial<Record<ErrorCode, string>> = {
  forbidden: "You don't have permission to do that. Ask a manager.",
  no_open_shift: "The shift isn't open. Someone who can open it needs to count the drawer first.",
  shift_open: "The shift is already open.",
  shift_closed: "This shift is already closed.",
  shift_not_found: "That shift isn't on the server.",
  invalid_count: "The notes and coins don't add up. Count them again.",
  already_counted: "The drawer has already been counted.",
  not_counted: "Count the drawer first.",
  drawer_counted: "The drawer is counted: nothing more goes in or out.",
  variance_unexplained: "Say why the drawer is over or short before closing.",
  already_explained: "A reason has already been given.",
  order_not_found: "This order isn't on the server any more.",
  order_closed: "This order is closed.",
  line_not_found: "That line isn't on this order.",
  line_not_active: "That line has been voided or comped.",
  modifiers_mismatch: "This line was changed on another screen. Close this and look again.",
  qty_mismatch: "This line was changed on another screen. Look again and retry.",
  line_in_kitchen: "This has gone to the kitchen. Void it and add it again.",
  needs_approval: "This needs a manager now. Use Void.",
  order_held: "This order is saved. Resume it to carry on.",
  order_not_held: "This order isn't saved.",
  venue_not_configured: "This venue's tax settings aren't finished. Ask a manager.",
  over_tender: "That's more than is owed now. Look at the amount due again.",
  order_invoiced: "This bill is invoiced. Close it and start a new order.",
  invoice_not_found: "That invoice isn't on this order.",
  print_job_not_found: "That print job isn't on this venue's queue.",
  print_job_state: "That job has moved on. Look at the queue again.",
  no_printer: "There is still no printer set up for this. Ask a manager to set one up.",
  not_invoiced: "This bill has no invoice yet. Void or comp the item instead.",
  over_credit: "That's more than is left to credit on this bill.",
  over_refund: "That payment has less left to refund than this. Choose another payment, or refund less.",
  refunds_mismatch: "The refunds must add up to the credit exactly.",
  split_invalid: "That split doesn't work. Check the number of payers.",
  split_over_allocated: "Those amounts come to more than is owed.",
  split_unassigned: "Every item needs someone to pay for it.",
  split_mismatch: "The split doesn't add up to the bill. Split it again.",
  split_after_payment: "Money has been taken, so split what's left evenly or by amount.",
  nothing_to_send: "Nothing is waiting to go to the kitchen or the bar.",
  course_already_fired: "That course has already been fired.",
  table_not_found: "That table isn't on the floor any more. Look at the floor again.",
  table_occupied: "Guests are still at this table: settle or move their order first.",
  table_not_empty: "Something is on this table's order: void it or take the payment first.",
  request_not_found: "That request isn't there any more. Look at the requests again.",
  request_decided: "Someone has already answered this request.",
  request_lapsed: "This request is from an earlier shift. Decline it.",
  table_not_ordering: "This table's ticket is on hold or already invoiced. Resume it, or decline the request.",
  rate_limited: "Too many requests. Wait a moment and retry.",
  internal: "The server hit a problem. Try again.",
};

/** One line for the till: what went wrong, never the server's internals. */
export function failureOf(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.unreachable) return "Not sent: the server can't be reached.";
    const said = (words as Partial<Record<string, string>>)[err.code];
    if (said) return said;
    return "The server refused this. Check the order and try again.";
  }
  return "Not sent: something went wrong.";
}

/**
 * The words for a failure that may already be in words: a server refusal by
 * its code, or the words the order queue chose (it rejects with an Error in
 * those words).
 */
export function wordsOf(err: unknown): string {
  if (err instanceof ApiError || !(err instanceof Error)) return failureOf(err);
  return err.message;
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
