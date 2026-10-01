// A shift's closing stock count on the till (ADR-114): the list while it is
// owed, what it found once it is in. The count is taken once per shift, so
// a retry after a lost answer is told it is already counted: that is the
// count having gone in, and the sheet is read again to show it.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "../../../shared/api";
import type { ClosingCountRequest, ClosingStockCount, ClosingStockSheet } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { failureOf } from "../register/order-messages";

export function useClosingStock(shiftId: number) {
  const store = useAuthStore();
  const queryClient = useQueryClient();
  const key = ["closing-stock", shiftId] as const;
  const path = `/api/v1/shift/${shiftId}/stock-count`;
  const sheet = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => store.api.get<ClosingStockSheet>(path, { signal }),
  });

  const record = async (counts: ClosingStockCount[]): Promise<ClosingStockSheet> => {
    const body: ClosingCountRequest = { counts };
    try {
      const res = await store.api.post<ClosingStockSheet>(path, body);
      queryClient.setQueryData(key, res);
      return res;
    } catch (err) {
      if (err instanceof ApiError && err.code === "stock_already_counted") {
        const res = await store.api.get<ClosingStockSheet>(path);
        queryClient.setQueryData(key, res);
        return res;
      }
      void queryClient.invalidateQueries({ queryKey: key });
      throw new Error(failureOf(err), { cause: err });
    }
  };

  return { sheet, record };
}
