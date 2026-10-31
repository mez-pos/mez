// The waiter's top bar on a table's own screen (T03): Back, to the tables, and
// nothing else. The table's name and what can be done are the screen's.

import { ArrowLeftIcon } from "lucide-react";
import { Link } from "wouter";
import { route } from "./routes";
import { buttonClass } from "../../../shared/ui/Button";

export function WaiterBack() {
  return (
    <Link href={route.waiter} className={buttonClass()}>
      <ArrowLeftIcon aria-hidden="true" size={19} />
      Back
    </Link>
  );
}
