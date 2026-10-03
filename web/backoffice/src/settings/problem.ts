// A failed settings request as words, and the field of the form to put them
// under. Every section reads a refusal the same way; each names its own fields.

import { ApiError } from "../../../shared/api";

export interface Problem<F extends string = string> {
  /** The field it is about, or null for the section's footer. */
  field: F | null;
  message: string;
}

/**
 * `fields` are the form's own: a refusal naming any other field (the server
 * names its keys, not our inputs) goes to the footer rather than nowhere.
 */
export function problemWith<F extends string>(err: unknown, fields: readonly F[]): Problem<F> {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the mez server. Check this computer's connection." };
    if (err.code === "forbidden") {
      return { field: null, message: "Only someone who can change the venue's settings can do this." };
    }
    const field = (fields as readonly string[]).includes(err.field ?? "") ? (err.field as F) : null;
    return { field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}
