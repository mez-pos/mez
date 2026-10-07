import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import type { OrderResponse } from "../../../shared/api-types";
import { newer, openTicketsKey, orderQueryKey, resetOrders } from "./use-ticket";

const order = (id: string, lastSeq: number, status: OrderResponse["status"]) => ({ id, last_seq: lastSeq, status }) as OrderResponse;

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

describe("resetOrders", () => {
  it("forgets every cached order, and nothing else, so the next read is taken as it comes", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(orderQueryKey("ord_1"), order("ord_1", 9, "open"));
    queryClient.setQueryData(orderQueryKey("ord_2"), order("ord_2", 4, "held"));
    queryClient.setQueryData(openTicketsKey, { orders: [] });

    await resetOrders(queryClient);

    expect(queryClient.getQueryData(orderQueryKey("ord_1"))).toBeUndefined();
    expect(queryClient.getQueryData(orderQueryKey("ord_2"))).toBeUndefined();
    expect(queryClient.getQueryData(openTicketsKey)).toEqual({ orders: [] });
    // A restored backup is behind what the till saw: its copy now wins.
    const restored = order("ord_1", 2, "open");
    expect(newer(queryClient.getQueryData<OrderResponse>(orderQueryKey("ord_1")), restored)).toBe(restored);
  });
});
