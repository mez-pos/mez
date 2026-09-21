// Calls fn once calls have stopped for `ms`, with the last arguments.
// flush() runs a pending call now; cancel() drops it.

export interface Debounced<A extends unknown[]> {
  call(...args: A): void;
  flush(): void;
  cancel(): void;
}

export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number): Debounced<A> {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let pending: A | null = null;

  const run = () => {
    timer = null;
    const args = pending;
    pending = null;
    if (args) fn(...args);
  };

  return {
    call(...args) {
      pending = args;
      if (timer !== null) clearTimeout(timer);
      timer = setTimeout(run, ms);
    },
    flush() {
      if (timer !== null) clearTimeout(timer);
      run();
    },
    cancel() {
      if (timer !== null) clearTimeout(timer);
      timer = null;
      pending = null;
    },
  };
}
