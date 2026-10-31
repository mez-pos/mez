// The kitchen display's app (docs/design/screens/kitchen-display.md). It
// pairs as a kitchen device and asks for no PIN (ADR-090): unpaired, it
// shows the pairing screen; paired, the station chosen on this device, or
// the choice. It runs from the terminal's bundle at /kitchen (ADR-091).

import type { StationResponse } from "../../../shared/api-types";
import { ArrowLeftIcon, LockIcon } from "lucide-react";
import { useLayoutEffect } from "react";
import { browserStorage } from "../../../shared/auth/storage";
import { useAuth } from "../../../shared/auth/use-auth";
import { NoVenueScreen } from "../../../shared/screens/NoVenueScreen";
import { PairScreen } from "../../../shared/screens/PairScreen";
import { Splash } from "../../../shared/screens/Splash";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { useMenu } from "../../../shared/use-menu";
import { readPrefs, surfaceOf } from "./board-prefs";
import type { SignedIn } from "./BoardFoot";
import { KitchenBoard } from "./KitchenBoard";

interface KitchenAppProps {
  /** The station this device shows, or null until one is chosen. */
  station: string | null;
  /** Keeps a station on the device; null clears it. The page reloads, to listen to it alone. */
  onStation: (code: string | null) => void;
}

export function KitchenApp({ station, onStation }: KitchenAppProps) {
  const status = useAuth();
  if (status.kind === "checking") return <Splash />;
  if (status.kind === "unpaired") return <PairScreen app="kitchen" />;
  if (status.kind === "no_venue") return <NoVenueScreen />;
  return <StationBoard station={station} onStation={onStation} />;
}

interface StationBoardProps extends KitchenAppProps {
  /** Who is signed in, on a terminal: the wall display has nobody. */
  signedIn?: SignedIn | undefined;
}

/** The board of the station chosen, or the choice of one. */
export function StationBoard({ station, onStation, signedIn }: StationBoardProps) {
  const menu = useMenu();
  const stations = menu.data?.stations ?? [];
  const chosen = stations.find((s) => s.code === station);
  // The choice of station is on the board's surface, so Change station at
  // night on /pass does not flash paper (KDS-18); the board sets it while it
  // is up. Leaving the kitchen gives the page back to paper: on a terminal
  // the next screen is the sign-in, which is never dark.
  useLayoutEffect(() => {
    if (!chosen) document.documentElement.dataset.surface = surfaceOf(readPrefs(browserStorage("local")));
  }, [chosen]);
  useLayoutEffect(
    () => () => {
      document.documentElement.dataset.surface = "paper";
    },
    [],
  );
  if (chosen) {
    return (
      <KitchenBoard
        station={chosen}
        onChangeStation={() => {
          rememberStation(chosen.code);
          onStation(null);
        }}
        signedIn={signedIn}
      />
    );
  }
  const previous = stations.find((s) => s.code === lastStation());

  return (
    <main className="flex min-h-screen flex-col gap-8 bg-canvas px-8 py-10 text-fg">
      <h1 className="font-serif text-display">Which station is this?</h1>
      {previous && (
        <button
          type="button"
          onClick={() => onStation(previous.code)}
          className="inline-flex min-h-tap w-fit items-center gap-2 rounded-pill border border-line-strong bg-raised px-5 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
        >
          <ArrowLeftIcon aria-hidden="true" size={18} />
          Back to {previous.name}
        </button>
      )}
      {menu.isPending && <p className="text-kitchen text-fg-muted">Loading the stations</p>}
      {menu.isError && !menu.data && (
        <EmptyState icon={<InboxIcon />} line="Can't reach the mez server." action={{ label: "Try again", onClick: () => void menu.refetch() }} />
      )}
      {menu.data && stations.length === 0 && (
        <p className="text-kitchen text-fg-muted">This venue has no stations yet. Add them in the back office, under Menu.</p>
      )}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-4">
        {stations.map((s: StationResponse) => (
          <button
            key={s.code}
            type="button"
            onClick={() => onStation(s.code)}
            className="min-h-24 rounded-card border-2 border-line-strong bg-raised px-6 text-left font-serif text-display shadow-card outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-4"
          >
            {s.name}
          </button>
        ))}
      </div>
      {signedIn && (
        // A cook who is signed in and cannot choose (no stations, the server
        // not reached) must still be able to lock the screen.
        <div className="mt-auto flex justify-end">
          <button
            type="button"
            onClick={signedIn.onLock}
            className="inline-flex min-h-tap items-center gap-2 rounded-pill border border-line-strong bg-raised px-5 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
          >
            <LockIcon aria-hidden="true" size={18} />
            Lock
          </button>
        </div>
      )}
    </main>
  );
}

// The station a display left by "Change station", for the picker to offer a
// way back: the page reloads between the two, so it is kept for the tab.
const lastStationKey = "mez.kitchen.last_station";

function rememberStation(code: string): void {
  try {
    sessionStorage.setItem(lastStationKey, code);
  } catch {
    // refused storage: no way back is offered
  }
}

function lastStation(): string | null {
  try {
    return sessionStorage.getItem(lastStationKey);
  } catch {
    return null;
  }
}
