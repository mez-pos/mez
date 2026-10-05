// The kitchen display's app (docs/design/screens/kitchen-display.md). It
// pairs as a kitchen device and asks for no PIN (ADR-090): unpaired, it
// shows the pairing screen; paired, the station chosen on this device, or
// the choice. It runs from the terminal's bundle at /kitchen (ADR-091).

import type { StationResponse } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { PairScreen } from "../../../shared/screens/PairScreen";
import { Splash } from "../../../shared/screens/Splash";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { useMenu } from "../../../shared/use-menu";
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
  if (chosen) return <KitchenBoard station={chosen} onChangeStation={() => onStation(null)} signedIn={signedIn} />;

  return (
    <main className="flex min-h-screen flex-col gap-8 bg-canvas px-8 py-10 text-fg">
      <h1 className="font-serif text-display">Which station is this?</h1>
      {menu.isPending && <p className="text-kitchen text-fg-muted">Loading the stations</p>}
      {menu.isError && !menu.data && (
        <EmptyState icon={<InboxIcon />} line="Can't reach the mez server." action={{ label: "Try again", onClick: () => void menu.refetch() }} />
      )}
      {menu.data && stations.length === 0 && <p className="text-kitchen text-fg-muted">This venue has no stations yet. Add them with the menu.</p>}
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
    </main>
  );
}
