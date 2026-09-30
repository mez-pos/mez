// The time, re-read every `every` ms, for screens whose ages or states
// must move on their own: a ticket's minutes, a table's time seated, a
// dish that is back on sale when the trading day turns.

import { useEffect, useState } from "react";

export function useClock(every: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), every);
    return () => clearInterval(id);
  }, [every]);
  return now;
}
