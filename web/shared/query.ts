// React Query, set up the same way in every app.

import { QueryClient, type QueryKey } from "@tanstack/react-query";
import { ApiError } from "./api";

// Only a server that was not reached is worth asking again. A 4xx is an
// answer; asking twice gets the same answer twice, slower.
const maxRetries = 2;

export function shouldRetry(failureCount: number, error: unknown): boolean {
  return failureCount < maxRetries && error instanceof ApiError && error.unreachable;
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // React Query pauses when navigator.onLine is false. That flag is the
        // browser's guess about the internet, and the server is on the local
        // network, often the same machine: a till with no internet can still
        // reach it. Always try, and let the request say whether it worked.
        networkMode: "always",
        retry: shouldRetry,
        // Freshness comes from the realtime stream, not from refocusing a
        // window that on a till is never unfocused.
        refetchOnWindowFocus: false,
        staleTime: 30_000,
      },
      mutations: {
        networkMode: "always",
        // Never retried automatically: a charge must be retried on purpose,
        // with the same idempotency key, by the screen that knows it.
        retry: false,
      },
    },
  });
}

/**
 * Puts a command's answer in the cache in place of whatever a read still in
 * flight would bring. The read left before the command, so what it brings
 * is older; cancelling it first means it is dropped when it lands, rather
 * than put back over the answer.
 */
export async function replaceQuery<T>(queryClient: QueryClient, queryKey: QueryKey, next: T): Promise<void> {
  await queryClient.cancelQueries({ queryKey });
  queryClient.setQueryData(queryKey, next);
}
