// "/pass": the kitchen's board for someone signed in on a terminal (ADR-179).
// The flow map lands the kitchen role here. It is the wall display's board
// (D10), read through the same API with the person's own session, so a ticket
// they start carries their name. Moving a ticket needs kitchen.work (ADR-180):
// someone who only sees the kitchen gets the board without its buttons. The station is kept on the device, as the
// wall display keeps its own; here changing it needs no reload, because a
// terminal's stream is not narrowed to one station.

import { useState } from "react";
import { browserStorage } from "../../../shared/auth/storage";
import { useAuth, useAuthStore } from "../../../shared/auth/use-auth";
import { StationBoard } from "./KitchenApp";
import { readStation, stationKey } from "./kitchen-logic";

export function CookBoard() {
  const auth = useAuth();
  const store = useAuthStore();
  const [station, setStation] = useState(() => readStation(browserStorage("local")));
  if (auth.kind !== "signed_in") return null; // the Gate never lets this happen

  const choose = (code: string | null) => {
    try {
      const durable = browserStorage("local");
      if (code) durable?.setItem(stationKey, code);
      else durable?.removeItem(stationKey);
    } catch {
      // Storage blocked: the choice lasts until the page is reloaded.
    }
    setStation(code);
  };

  const signedIn = { name: auth.staff.name, onLock: () => void store.signOut(), canWork: auth.permissions.includes("kitchen.work") };
  return <StationBoard station={station} onStation={choose} signedIn={signedIn} />;
}
