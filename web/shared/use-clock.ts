// The time, re-read every `every` ms, for screens whose ages or states
// must move on their own: a ticket's minutes, a table's time seated, a
// dish that is back on sale when the trading day turns, a countdown.
//
// `ticking` false stops it, for a countdown that is only sometimes shown;
// it reads the time again the moment it starts once more.

import { useEffect, useState } from "react";

export function useClock(every: number, ticking = true): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!ticking) return;
    const tick = () => setNow(Date.now());
    // Stopped a while, the time kept is old: read it now, not a tick from now.
    const first = setTimeout(tick, 0);
    const id = setInterval(tick, every);
    return () => {
      clearTimeout(first);
      clearInterval(id);
    };
  }, [every, ticking]);
  return now;
}
