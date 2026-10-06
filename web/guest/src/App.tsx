// The guest's app: a table's code in the path, and the landing, the menu or
// the cart, the order placed, the bill or its receipt in the hash (route.ts).

import { useEffect, useState } from "react";
import { Bill } from "./Bill";
import { Cart } from "./Cart";
import { Landing } from "./Landing";
import { Menu } from "./Menu";
import { Order, OrderStrip } from "./Order";
import { problemOf } from "./menu-logic";
import { Problem } from "./Problem";
import { Receipt } from "./Receipt";
import { hashFor, tableCodeFrom, viewFrom, type View } from "./route";
import { useCart } from "./use-cart";
import { hasStartedCharge, hasStartedRoundCharge } from "./use-charge";
import { useGuestMenu, useTable } from "./use-guest";
import { useOrder } from "./use-order";
import { keptBill } from "./use-receipt";

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
  const cart = useCart(code, menu.data);
  const order = useOrder(code, cart);
  // Back from a payment gateway's page, the address is the table's alone:
  // a guest who went to pay comes back to what they were paying for, the
  // order or the bill, and not the landing.
  useEffect(() => {
    if (viewFrom(window.location.hash) !== "landing") return;
    if (order.placed && hasStartedRoundCharge(code, order.placed.round.id)) go("order");
    else if (hasStartedCharge(code)) go("bill");
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once, on arriving
  }, []);

  if (table.data === undefined) {
    if (table.isPending) return <Waiting line="Finding your table" />;
    const kind = problemOf(table.error);
    return <Problem kind={kind} onRetry={kind === "unknown_code" ? undefined : () => void table.refetch()} />;
  }
  if (view === "landing") return <Landing table={table.data} onStart={() => go("menu")} />;
  // The order placed is the server's answer, kept on the phone: it shows
  // with or without the menu.
  if (view === "order" && order.placed) {
    return (
      <Order
        key={order.placed.round.id}
        code={code}
        table={table.data}
        placed={order.placed}
        onRefresh={order.refresh}
        onMenu={() => go("menu")}
        onBill={() => go("bill")}
      />
    );
  }
  if (view === "bill") return <Bill code={code} table={table.data} onBack={() => go("menu")} onReceipt={() => go("receipt")} />;
  if (view === "receipt") return <Receipt code={code} table={table.data} bill={keptBill(code)?.id ?? null} onBack={() => go("bill")} />;

  if (menu.data === undefined) {
    if (menu.isPending) return <Waiting line="Loading the menu" />;
    const kind = problemOf(menu.error);
    return <Problem kind={kind} onRetry={kind === "unknown_code" ? undefined : () => void menu.refetch()} />;
  }
  const strip = order.placed && <OrderStrip placed={order.placed} venue={table.data.venue.name} onOpen={() => go("order")} />;
  if (view === "cart") {
    return (
      <>
        {strip}
        <Cart cart={cart} order={order} onBack={() => go("menu")} onPlaced={() => go("order")} onBill={() => go("bill")} />
      </>
    );
  }
  return (
    <>
      {strip}
      <Menu table={table.data} menu={menu.data} cart={cart} onViewCart={() => go("cart")} />
    </>
  );
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
