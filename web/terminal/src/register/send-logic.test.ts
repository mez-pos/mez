import { describe, expect, it } from "vitest";
import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { courseGroups, readyLines, sendLabel, sendPlan, sendPreview } from "./send-logic";

const line = (id: number, course: number, extra: Partial<LineResponse> = {}): LineResponse =>
  ({ id, course, station: "kitchen", status: "active", sent: false, course_fired: false, ...extra }) as LineResponse;

const order = (lines: LineResponse[]): OrderResponse => ({ lines }) as unknown as OrderResponse;

describe("sendPlan", () => {
  it("fires the first course while nothing has gone", () => {
    const plan = sendPlan(order([line(2, 1), line(3, 2), line(4, 1, { station: "bar" })]));
    expect(plan).toEqual({ waiting: 2, firing: 1, next: null });
    expect(sendLabel(plan)).toBe("Send to Kitchen");
  });

  it("sends what was added to a fired course, and offers the next course", () => {
    const plan = sendPlan(order([line(2, 1, { sent: true, course_fired: true }), line(3, 2), line(4, 1, { course_fired: true }), line(5, 3), line(6, 2)]));
    expect(plan).toEqual({ waiting: 1, firing: null, next: { course: 2, lines: 2 } });
  });

  it("sends lines in no course at once", () => {
    expect(sendPlan(order([line(2, 0), line(3, 2)]))).toEqual({ waiting: 2, firing: 2, next: null });
  });

  it("leaves out voided, sent and stationless lines", () => {
    const plan = sendPlan(order([line(2, 1, { status: "voided" }), line(3, 1, { sent: true, course_fired: true }), line(4, 1, { station: "" })]));
    expect(plan).toEqual({ waiting: 0, firing: null, next: null });
    expect(sendLabel(plan)).toBe("Sent");
  });

  it("is nothing without an order", () => {
    expect(sendPlan(undefined)).toEqual({ waiting: 0, firing: null, next: null });
  });
});

describe("readyLines", () => {
  it("is the lines on tickets the kitchen has bumped", () => {
    const o = {
      lines: [],
      tickets: [
        { seq: 3, station: "bar", line_ids: [2], at: "", status: "ready" },
        { seq: 5, station: "kitchen", line_ids: [4], at: "", status: "fired" },
      ],
    } as unknown as OrderResponse;
    expect([...readyLines(o)]).toEqual([2]);
    expect(readyLines(undefined).size).toBe(0);
  });
});

describe("sendPreview", () => {
  const ids = (lines: readonly LineResponse[]) => lines.map((l) => l.id);

  it("lists what one Send would send, in the order's order, and counts what stays", () => {
    const preview = sendPreview(order([line(2, 2), line(3, 1), line(4, 0), line(5, 3), line(6, 1)]));
    expect(ids(preview.going)).toEqual([3, 4, 6]);
    expect(preview).toMatchObject({ gone: 0, held: 2 });
  });

  it("sends only what was added to a course already fired, once one has gone", () => {
    const preview = sendPreview(order([line(2, 1, { sent: true, course_fired: true }), line(3, 1, { course_fired: true }), line(4, 2), line(5, 0)]));
    expect(ids(preview.going)).toEqual([3, 5]);
    expect(preview).toMatchObject({ gone: 1, held: 1 });
  });

  it("does not count a voided line as with the kitchen, or a stationless one as going", () => {
    const preview = sendPreview(order([line(2, 1, { sent: true, status: "voided" }), line(3, 1, { station: "" }), line(4, 1)]));
    expect(ids(preview.going)).toEqual([4]);
    expect(preview).toMatchObject({ gone: 0, held: 0 });
  });

  it("agrees with the plan about how many go", () => {
    const orders = [
      order([line(2, 1), line(3, 2), line(4, 1, { station: "bar" })]),
      order([line(2, 1, { sent: true, course_fired: true }), line(3, 2), line(4, 1, { course_fired: true }), line(5, 3)]),
      order([line(2, 0), line(3, 2)]),
      order([]),
    ];
    for (const o of orders) expect(sendPreview(o).going).toHaveLength(sendPlan(o).waiting);
    expect(sendPreview(undefined)).toEqual({ going: [], gone: 0, held: 0 });
  });
});

describe("courseGroups", () => {
  const name = (course: number | undefined) => (course ? `Course ${course}` : null);
  const shape = (o: OrderResponse) => courseGroups(o, name).map((g) => [g.name, g.state, g.canFire, g.lines.map((l) => l.id)]);

  it("groups the lines by course in course order, lines with no course first", () => {
    expect(shape(order([line(2, 2), line(3, 1), line(4, 0), line(5, 1)]))).toEqual([
      ["No course", "waiting", false, [4]],
      ["Course 1", "waiting", false, [3, 5]],
      ["Course 2", "held", false, [2]],
    ]);
  });

  it("says a course is with the kitchen once all of it has gone, and offers the next one to fire", () => {
    const o = order([line(2, 1, { sent: true, course_fired: true }), line(3, 2), line(4, 3)]);
    expect(shape(o)).toEqual([
      ["Course 1", "sent", false, [2]],
      ["Course 2", "held", true, [3]],
      ["Course 3", "held", false, [4]],
    ]);
  });

  it("says a fired course is waiting again when something is added to it", () => {
    const o = order([line(2, 1, { sent: true, course_fired: true }), line(3, 1, { course_fired: true })]);
    expect(shape(o)).toEqual([["Course 1", "waiting", false, [2, 3]]]);
  });

  it("leaves out voided lines, and a course with nothing left in it", () => {
    expect(shape(order([line(2, 1, { status: "voided" }), line(3, 2)]))).toEqual([["Course 2", "waiting", false, [3]]]);
    expect(courseGroups(undefined, name)).toEqual([]);
  });

  it("reads a line with nowhere to go as with the kitchen: there is nothing to send", () => {
    expect(shape(order([line(2, 0, { station: "" })]))).toEqual([["No course", "sent", false, [2]]]);
  });
});
