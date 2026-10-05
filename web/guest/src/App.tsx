// The guest's app: a table's code in the path, and the landing or the menu
// in the hash (route.ts). The cart and the rest arrive with Task 3c on.

import { useEffect, useState } from "react";
import { Landing } from "./Landing";
import { Menu } from "./Menu";
import { problemOf } from "./menu-logic";
import { Problem } from "./Problem";
import { hashFor, tableCodeFrom, viewFrom, type View } from "./route";
import { useGuestMenu, useTable } from "./use-guest";

export function App() {
  const code = tableCodeFrom(window.location.pathname);
  // A path with no code in it was not written by a table's sticker.
  if (code === null) return <Problem kind="unknown_code" />;
  return <Table code={code} />;
}

function Table({ code }: { code: string }) {
  const [view, go] = useView();
  const table = useTable(code);
  // Asked for as soon as the table is known, so the menu is there by the
  // time the guest has read the landing and tapped.
  const menu = useGuestMenu(code, table.data !== undefined);

  if (table.data === undefined) {
    if (table.isPending) return <Waiting line="Finding your table" />;
    const kind = problemOf(table.error);
    return <Problem kind={kind} onRetry={kind === "unknown_code" ? undefined : () => void table.refetch()} />;
  }
  if (view === "landing") return <Landing table={table.data} onStart={() => go("menu")} />;

  if (menu.data === undefined) {
    if (menu.isPending) return <Waiting line="Loading the menu" />;
    const kind = problemOf(menu.error);
    return <Problem kind={kind} onRetry={kind === "unknown_code" ? undefined : () => void menu.refetch()} />;
  }
  return <Menu table={table.data} menu={menu.data} />;
}

function Waiting({ line }: { line: string }) {
  return (
    <main className="flex min-h-dvh items-center justify-center bg-canvas p-6 text-fg-muted">
      <p role="status" className="text-base">
        {line}
      </p>
    </main>
  );
}

/**
 * The screen, kept in the address's hash, so the phone's Back button goes
 * from the menu to the landing and a reload stays where it was.
 */
function useView(): [View, (view: View) => void] {
  const [view, setView] = useState<View>(() => viewFrom(window.location.hash));
  useEffect(() => {
    const read = () => setView(viewFrom(window.location.hash));
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, []);
  return [
    view,
    (next) => {
      window.location.hash = hashFor(next);
      window.scrollTo(0, 0);
    },
  ];
}
