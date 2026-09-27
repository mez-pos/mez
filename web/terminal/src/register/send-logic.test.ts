import { describe, expect, it } from "vitest";
import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { sendLabel, sendPlan } from "./send-logic";

const line = (id: number, course: number, extra: Partial<LineResponse> = {}): LineResponse =>
  ({ id, course, station: "kitchen", status: "active", sent: false, course_fired: false, ...extra }) as LineResponse;

const order = (lines: LineResponse[]): OrderResponse => ({ lines }) as unknown as OrderResponse;

describe("sendPlan", () => {
  it("fires the first course while nothing has gone", () => {
    const plan = sendPlan(order([line(2, 1), line(3, 2), line(4, 1, { station: "bar" })]));
    expect(plan).toEqual({ waiting: 2, firing: 1, next: null });
    expect(sendLabel(plan)).toBe("Send 2");
  });

  it("sends what was added to a fired course, and offers the next course", () => {
    const plan = sendPlan(
      order([
        line(2, 1, { sent: true, course_fired: true }),
        line(3, 2),
        line(4, 1, { course_fired: true }),
        line(5, 3),
        line(6, 2),
      ]),
    );
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
