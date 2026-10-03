import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { ReceiptResponse } from "../../../shared/api-types";
import {
  PAPERS,
  linesOf,
  receiptChanged,
  receiptFormFrom,
  receiptPreviewKey,
  receiptProblem,
  receiptProblemOf,
  toPreviewInput,
  toReceiptInput,
  type ReceiptForm,
} from "./receipt-logic";

const saved: ReceiptResponse = {
  header: ["Unit 4, MG Road", "Tel 080 1234 5678"],
  footer: ["Thank you!"],
  hide_barcode: false,
  hide_order_ref: true,
  hide_modifiers: false,
  max_lines: 4,
  max_line_chars: 48,
};
const limits = { maxLines: 4, maxChars: 48 };
const form = (patch: Partial<ReceiptForm> = {}): ReceiptForm => ({ ...receiptFormFrom(saved), ...patch });

describe("lines of text", () => {
  it("are trimmed, with the blank ones dropped, whatever the line ends", () => {
    expect(linesOf("")).toEqual([]);
    expect(linesOf("  \n\n ")).toEqual([]);
    expect(linesOf("one")).toEqual(["one"]);
    expect(linesOf("  one \r\n\r\n two\nthree  ")).toEqual(["one", "two", "three"]);
  });
});

describe("the receipt form", () => {
  it("is what the server holds: lines as text, and what to print rather than what to hide", () => {
    expect(receiptFormFrom(saved)).toEqual({
      header: "Unit 4, MG Road\nTel 080 1234 5678",
      footer: "Thank you!",
      barcode: true,
      orderRef: false,
      modifiers: true,
    });
  });
  it("sends lines and the switches as the server keeps them", () => {
    expect(toReceiptInput(form({ header: "  a \n\n b ", footer: "", barcode: false, orderRef: true, modifiers: false }))).toEqual({
      header: ["a", "b"],
      footer: [],
      hide_barcode: true,
      hide_order_ref: false,
      hide_modifiers: true,
    });
  });
  it("round-trips what the server sent", () => {
    const { max_lines: _l, max_line_chars: _c, ...sent } = saved;
    expect(toReceiptInput(receiptFormFrom(saved))).toEqual(sent);
  });
  it("is asked a preview at a paper width", () => {
    expect(toPreviewInput(form(), 58)).toMatchObject({ paper_mm: 58, header: ["Unit 4, MG Road", "Tel 080 1234 5678"], hide_order_ref: true });
    expect(PAPERS.map((p) => p.value)).toEqual([80, 58]);
  });
  it("is changed by a real difference, not by blank lines or spaces", () => {
    const base = form();
    expect(receiptChanged(base, base)).toBe(false);
    expect(receiptChanged(form({ header: "\n  Unit 4, MG Road  \n\nTel 080 1234 5678\n" }), base)).toBe(false);
    expect(receiptChanged(form({ footer: "Thanks" }), base)).toBe(true);
    expect(receiptChanged(form({ barcode: false }), base)).toBe(true);
    expect(receiptChanged(form({ orderRef: true }), base)).toBe(true);
    expect(receiptChanged(form({ modifiers: false }), base)).toBe(true);
  });
  it("keys a preview by the draft and the paper", () => {
    const base = form();
    expect(receiptPreviewKey(base, 80)).toBe(receiptPreviewKey({ ...base }, 80));
    expect(receiptPreviewKey(base, 80)).not.toBe(receiptPreviewKey(base, 58));
    expect(receiptPreviewKey(base, 80)).not.toBe(receiptPreviewKey(form({ footer: "x" }), 80));
    expect(receiptPreviewKey(base, 80)).toBe(receiptPreviewKey(form({ footer: "  Thank you!  \n\n" }), 80));
  });
});

describe("what is refused before asking the server", () => {
  it("lets a good form through, and an empty one", () => {
    expect(receiptProblem(form(), limits)).toBeNull();
    expect(receiptProblem(form({ header: "", footer: "" }), limits)).toBeNull();
    expect(receiptProblem(form({ header: "a\nb\nc\nd" }), limits)).toBeNull();
  });
  it("refuses more lines than fit, not counting blank ones", () => {
    expect(receiptProblem(form({ header: "1\n2\n3\n4\n5" }), limits)).toEqual({ field: "header", message: "Use at most 4 lines." });
    expect(receiptProblem(form({ footer: "1\n2\n3\n4\n5" }), limits)?.field).toBe("footer");
    expect(receiptProblem(form({ header: "1\n\n2\n\n3\n\n4\n\n" }), limits)).toBeNull();
  });
  it("refuses a line too long, naming it, counting characters", () => {
    expect(receiptProblem(form({ footer: `ok\n${"a".repeat(49)}` }), limits)).toEqual({ field: "footer", message: "Keep line 2 to 48 characters." });
    expect(receiptProblem(form({ footer: "a".repeat(48) }), limits)).toBeNull();
    expect(receiptProblem(form({ footer: "\u{1F600}".repeat(48) }), limits)).toBeNull(); // 48 characters, not 96 units
  });
  it("reads the header before the footer", () => {
    expect(receiptProblem(form({ header: "1\n2\n3\n4\n5", footer: "1\n2\n3\n4\n5" }), limits)?.field).toBe("header");
  });
});

describe("a failed request", () => {
  it("puts the server's words under the header or the footer", () => {
    expect(receiptProblemOf(new ApiError("invalid_setting", "Line 1 has a character the printer cannot print.", 422, "header"))).toEqual({
      field: "header",
      message: "Line 1 has a character the printer cannot print.",
    });
    expect(receiptProblemOf(new ApiError("invalid_setting", "x", 422, "footer")).field).toBe("footer");
    expect(receiptProblemOf(new ApiError("invalid_setting", "x", 422, "paper_mm")).field).toBeNull();
  });
});
