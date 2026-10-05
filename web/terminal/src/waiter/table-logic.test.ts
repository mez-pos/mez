import { describe, expect, it } from "vitest";
import type { LineResponse, LiveTableResponse, OrderResponse, TicketResponse } from "../../../shared/api-types";
import { detailOrder, orderProgress, startedText } from "./table-logic";

const table = (extra: Partial<LiveTableResponse> = {}): LiveTableResponse => ({
  id: 1,
  area_id: 10,
  name: "T1",
  seats: 4,
  shape: "square",
  x: 0,
  y: 0,
  status: "ordered",
  covers: 2,
  orders: ["ord_A"],
  ...extra,
});

const line = (id: number, extra: Partial<LineResponse> = {}): LineResponse =>
  ({ id, station: "kitchen", status: "active", sent: false, course_fired: false, ...extra }) as LineResponse;
const kot = (lineIds: number[], extra: Partial<TicketResponse> = {}): TicketResponse =>
  ({ seq: 1, station: "kitchen", line_ids: lineIds, at: "2026-10-05T13:45:00Z", status: "sent", ...extra }) as TicketResponse;
const order = (lines: LineResponse[], tickets: TicketResponse[] = []): OrderResponse =>
  ({ opened_at: "2026-10-05T13:44:00Z", lines, tickets }) as unknown as OrderResponse;

describe("detailOrder", () => {
  it("is the one order at a table with guests", () => {
    expect(detailOrder(table())).toBe("ord_A");
    expect(detailOrder(table({ status: "seated" }))).toBe("ord_A");
    expect(detailOrder(table({ status: "settling" }))).toBe("ord_A");
  });
  it("is none for a table that is free or to clear: the floor seats and clears", () => {
    expect(detailOrder(table({ status: "free", orders: [] }))).toBeNull();
    expect(detailOrder(table({ status: "dirty", orders: [] }))).toBeNull();
  });
  it("is none for a table with several orders: the floor asks which", () => {
    expect(detailOrder(table({ orders: ["ord_A", "ord_B"] }))).toBeNull();
  });
});

describe("orderProgress", () => {
  const shape = (o: OrderResponse) => orderProgress(o).map((s) => [s.key, s.state, s.detail]);

  it("starts with nothing placed", () => {
    expect(shape(order([]))).toEqual([
      ["placed", "todo", "Nothing ordered yet"],
      ["sent", "todo", "Nothing sent yet"],
      ["preparing", "todo", "Nothing in preparation"],
      ["ready", "todo", "Nothing at the pass"],
    ]);
  });

  it("counts what is placed, and waits to be sent", () => {
    expect(shape(order([line(1), line(2)]))).toEqual([
      ["placed", "done", "2 items"],
      ["sent", "now", "2 items to send"],
      ["preparing", "todo", "Nothing in preparation"],
      ["ready", "todo", "Nothing at the pass"],
    ]);
  });

  it("is in preparation once sent, and ready as the kitchen bumps tickets", () => {
    const lines = [line(1, { sent: true }), line(2, { sent: true }), line(3, { sent: true })];
    expect(shape(order(lines, [kot([1, 2]), kot([3], { seq: 2, status: "ready", ready_at: "2026-10-05T13:55:00Z" })]))).toEqual([
      ["placed", "done", "3 items"],
      ["sent", "done", "3 items"],
      ["preparing", "now", "2 items"],
      ["ready", "done", "1 item at the pass"],
    ]);
  });

  it("is done preparing when everything sent is at the pass", () => {
    const o = order([line(1, { sent: true })], [kot([1], { status: "ready", ready_at: "2026-10-05T13:55:00Z" })]);
    expect(shape(o).slice(2)).toEqual([
      ["preparing", "done", "Nothing in preparation"],
      ["ready", "done", "1 item at the pass"],
    ]);
  });

  it("leaves voided lines out, and gives each step its time", () => {
    const o = order([line(1, { sent: true }), line(2, { status: "voided", sent: true })], [kot([1, 2])]);
    const steps = orderProgress(o);
    expect(steps.map((s) => s.detail)).toEqual(["1 item", "1 item", "1 item", "Nothing at the pass"]);
    expect(steps.map((s) => s.at)).toEqual(["2026-10-05T13:44:00Z", "2026-10-05T13:45:00Z", undefined, undefined]);
  });
});

describe("startedText", () => {
  const now = Date.parse("2026-10-05T14:56:00Z");
  it("says when the table sat down, in the venue's time, and for how long", () => {
    expect(startedText("2026-10-05T13:44:00Z", now, "Asia/Kolkata")).toBe("Started 7:14 PM · 1h 12m");
  });
  it("says nothing for a table with no time", () => {
    expect(startedText(undefined, now, "Asia/Kolkata")).toBeNull();
  });
});
