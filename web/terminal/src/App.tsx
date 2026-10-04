// The terminal's routes (ADR-148). Every path goes through the Gate, which
// sends an unpaired device to /pair and a locked one to /lock
// (shared/auth/route.ts). Signed in, "/" lands a person on their own work by
// role, and every other screen is inside the shell, with the ticket, the floor
// and the shift above it, and behind the shift gate: no shift, no screens.

import { Redirect, Route, Switch } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { Splash } from "../../shared/screens/Splash";
import { TablesScreen } from "./floor/TablesScreen";
import { OrderScreen } from "./register/OrderScreen";
import { OrdersPage, Landing, LeaveForKitchen, PayPage, ShiftClosePage, ShiftPage, ZReportPage } from "./shell/pages";
import { route } from "./shell/routes";
import { ShiftGate } from "./shell/ShiftGate";
import { TerminalProvider } from "./shell/terminal-context";
import { TerminalShell } from "./shell/TerminalShell";

export function App() {
  return (
    <Gate splash={<Splash />}>
      <Switch>
        <Route path="/pair">
          <PairScreen app="terminal" />
        </Route>
        <Route path="/lock">
          <LockScreen />
        </Route>
        <Route path={route.kitchen}>
          <LeaveForKitchen />
        </Route>
        <Route path="/">
          <Landing />
        </Route>
        <Route>
          <TerminalProvider>
            <TerminalShell>
              <ShiftGate>
                <Screens />
              </ShiftGate>
            </TerminalShell>
          </TerminalProvider>
        </Route>
      </Switch>
    </Gate>
  );
}

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
      {/* A waiter's own tables arrive in Task 11; until then they are the floor. */}
      <Route path={route.waiter}>
        <Redirect to={route.tables} replace />
      </Route>
      {/* One route for the list and the order picked on it, so the tab and the search stay as rows are picked. */}
      <Route path={`${route.orders}/:id?`}>{(params) => <OrdersPage orderId={params.id ?? null} />}</Route>
      <Route path="/pay/:id">{(params) => <PayPage key={params.id} orderId={params.id} />}</Route>
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
