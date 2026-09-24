import { describe, expect, it } from "vitest";
import type { OrderResponse } from "../../../shared/api-types";
import { newer } from "./use-ticket";

const order = (id: string, lastSeq: number, status: OrderResponse["status"]) =>
  ({ id, last_seq: lastSeq, status }) as OrderResponse;

describe("newer", () => {
  it("keeps the later copy, whichever arrives last", () => {
    const resumed = order("ord_1", 4, "open");
    const staleRead = order("ord_1", 3, "held");
    expect(newer(resumed, staleRead)).toBe(resumed);
    expect(newer(staleRead, resumed)).toBe(resumed);
  });

  it("takes the incoming copy when there is nothing newer", () => {
    const a = order("ord_1", 4, "open");
    expect(newer(undefined, a)).toBe(a);
    expect(newer(order("ord_1", 4, "open"), a)).toBe(a);
    expect(newer(order("ord_2", 9, "open"), a)).toBe(a);
  });
});
