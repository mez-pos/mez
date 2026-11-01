// The terminal's routes (ADR-148). Every path but the setup wizard's goes
// through the Gate, which sends an unpaired device to /pair and a locked one
// to /lock (shared/auth/route.ts). The wizard is outside it (ADR-267): it
// starts on a device that is not paired and ends on one that is signed in, and
// the Gate would move it on at each change. Signed in, "/" lands a person on their own work by
// role, and every other screen is inside the shell, with the ticket, the floor
// and the shift above it, and behind the shift gate: no shift, no screens.

import { Suspense, useEffect } from "react";
import { Redirect, Route, Switch } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { RouteBoundary } from "../../shared/RouteBoundary";
import { ErrorBoundary } from "../../shared/ui/ErrorBoundary";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { setupPath } from "../../shared/screens/setup/setup-path";
import { SetupWizard } from "../../shared/screens/setup/SetupWizard";
import { Splash } from "../../shared/screens/Splash";
import { TablesScreen } from "./floor/TablesScreen";
import { OrderScreen } from "./register/OrderScreen";
import { lazyScreen, warm } from "./shell/lazy-screen";
import { OrdersPage, Landing } from "./shell/pages";
import { route } from "./shell/routes";
import { ShiftGate } from "./shell/ShiftGate";
import { TerminalProvider } from "./shell/terminal-context";
import { TerminalShell } from "./shell/TerminalShell";

// What the till boots with is the order screen, the floor and the day's
// orders, the screens of service. The rest is fetched while the till is idle,
// once it has drawn (TECH-25, ADR-408): paying, the shift's open and close,
// the waiter's screens and the cook's board.
const PayPage = lazyScreen("PayPage", () => import("./shell/PayPage").then((m) => m.PayPage));
const ShiftPage = lazyScreen("ShiftPage", () => import("./shell/ShiftPages").then((m) => m.ShiftPage));
const ShiftClosePage = lazyScreen("ShiftClosePage", () => import("./shell/ShiftPages").then((m) => m.ShiftClosePage));
const ZReportPage = lazyScreen("ZReportPage", () => import("./shell/ShiftPages").then((m) => m.ZReportPage));
const WaiterHome = lazyScreen("WaiterHome", () => import("./waiter/WaiterHome").then((m) => m.WaiterHome));
const ShiftSummary = lazyScreen("ShiftSummary", () => import("./waiter/ShiftSummary").then((m) => m.ShiftSummary));
const TableDetail = lazyScreen("TableDetail", () => import("./waiter/TableDetail").then((m) => m.TableDetail));
const CookBoard = lazyScreen("CookBoard", () => import("./kitchen/CookBoard").then((m) => m.CookBoard));
const later = [PayPage, ShiftPage, ShiftClosePage, ZReportPage, WaiterHome, ShiftSummary, TableDetail, CookBoard];

export function App() {
  useEffect(() => warm(later), []);
  return (
    <Switch>
      <Route path={setupPath}>
        <SetupWizard />
      </Route>
      <Route>
        <Gated />
      </Route>
    </Switch>
  );
}

/** Everything behind the guard. */
function Gated() {
  return (
    <Gate splash={<Splash />}>
      {/* A screen with no shell around it fails as a page; one inside the shell, below, as a region (TECH-8). */}
      <RouteBoundary size="page">
        {/* Only the cook's board is fetched at this level; a blink on a LAN, and it has no shell to keep. */}
        <Suspense fallback={null}>
          <Switch>
            <Route path="/pair">
              <PairScreen app="terminal" />
            </Route>
            <Route path="/lock">
              <LockScreen />
            </Route>
            {/* The kitchen's board has its own header and foot (D10): no shell, and no shift to wait for. */}
            <Route path={route.pass}>
              <CookBoard />
            </Route>
            <Route path="/">
              <Landing />
            </Route>
            <Route>
              <TerminalProvider>
                <TerminalShell>
                  <ShiftGate>
                    <RouteBoundary>
                      {/* A screen still being fetched leaves the shell's header and way out standing. */}
                      <Suspense fallback={null}>
                        <Screens />
                      </Suspense>
                    </RouteBoundary>
                  </ShiftGate>
                </TerminalShell>
              </TerminalProvider>
            </Route>
          </Switch>
        </Suspense>
      </RouteBoundary>
    </Gate>
  );
}

/** What the pay screen's fallback says: a payment taken before it failed is not lost (TECH-8). */
const payLine = "Any payment already taken is recorded, and the order shows it. Try again, or reload.";

/** The screens inside the shell. */
function Screens() {
  return (
    <Switch>
      <Route path={route.order}>
        <OrderScreen key="order" />
      </Route>
      <Route path={route.tables}>
        <TablesScreen />
      </Route>
      <Route path={route.waiter}>
        <WaiterHome />
      </Route>
      <Route path={route.waiterShift}>
        <ShiftSummary />
      </Route>
      <Route path="/waiter/table/:id">
        {(params) =>
          /^[1-9]\d{0,9}$/.test(params.id ?? "") ? <TableDetail key={params.id} tableId={Number(params.id)} /> : <Redirect to={route.waiter} replace />
        }
      </Route>
      {/* One route for the list and the order picked on it, so the tab and the search stay as rows are picked. */}
      <Route path={`${route.orders}/:id?`}>{(params) => <OrdersPage orderId={params.id ?? null} />}</Route>
      <Route path="/pay/:id">
        {(params) => (
          <ErrorBoundary where="pay" size="region" resetKey={params.id} line={payLine}>
            <PayPage key={params.id} orderId={params.id} />
          </ErrorBoundary>
        )}
      </Route>
      <Route path={route.shift}>
        <ShiftPage />
      </Route>
      <Route path={route.shiftClose}>
        <ShiftClosePage />
      </Route>
      <Route path="/shift/z/:n">
        {(params) => (/^[1-9]\d{0,9}$/.test(params.n ?? "") ? <ZReportPage number={Number(params.n)} /> : <Redirect to={route.shift} replace />)}
      </Route>
      {/* Anywhere else is landed again, from here: going through "/" would leave the provider and lose the ticket in hand. */}
      <Route>
        <Landing />
      </Route>
    </Switch>
  );
}
