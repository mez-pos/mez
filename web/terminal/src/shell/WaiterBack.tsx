// The waiter's top bar on a table's own screen (T03): Back, to the tables, and
// nothing else. The table's name and what can be done are the screen's.

import { ArrowLeftIcon } from "lucide-react";
import { Link } from "wouter";
import { route } from "./routes";

export function WaiterBack() {
  return (
    <Link
      href={route.waiter}
      className="inline-flex min-h-tap items-center gap-2 rounded-pill border border-line-strong px-5 text-base font-semibold outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3"
    >
      <ArrowLeftIcon aria-hidden="true" size={19} />
      Back
    </Link>
  );
}
