// The terminal's routes (ADR-148). Every path but the setup wizard's goes
// through the Gate, which sends an unpaired device to /pair and a locked one
// to /lock (shared/auth/route.ts). The wizard is outside it (ADR-267): it
// starts on a device that is not paired and ends on one that is signed in, and
// the Gate would move it on at each change. Signed in, "/" lands a person on their own work by
// role, and every other screen is inside the shell, with the ticket, the floor
// and the shift above it, and behind the shift gate: no shift, no screens.

import { Redirect, Route, Switch } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { setupPath } from "../../shared/screens/setup/setup-path";
import { SetupWizard } from "../../shared/screens/setup/SetupWizard";
import { Splash } from "../../shared/screens/Splash";
import { TablesScreen } from "./floor/TablesScreen";
import { CookBoard } from "./kitchen/CookBoard";
import { OrderScreen } from "./register/OrderScreen";
import { OrdersPage, Landing, PayPage, ShiftClosePage, ShiftPage, ZReportPage } from "./shell/pages";
import { route } from "./shell/routes";
import { ShiftGate } from "./shell/ShiftGate";
import { TerminalProvider } from "./shell/terminal-context";
import { TerminalShell } from "./shell/TerminalShell";
import { ShiftSummary } from "./waiter/ShiftSummary";
import { TableDetail } from "./waiter/TableDetail";
import { WaiterHome } from "./waiter/WaiterHome";

export function App() {
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
