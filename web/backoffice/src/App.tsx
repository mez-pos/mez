// The back office's routes, under /backoffice. Sign-in is the same staff PIN
// as the terminal, on a device paired as a back office (ADR-056).

import { Redirect, Route, Router, Switch, useLocation } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { Splash } from "../../shared/screens/Splash";
import { DevicesScreen } from "./devices/DevicesScreen";
import { FloorScreen } from "./floor/FloorScreen";
import { MenuScreen } from "./menu/MenuScreen";
import { PrintersScreen } from "./printers/PrintersScreen";
import { ZReportsScreen } from "./reports/ZReportsScreen";
import { ExportsPanel } from "./reports/ExportsPanel";
import { BackOfficeShell } from "./shell/BackOfficeShell";
import { Dashboard } from "./dashboard/Dashboard";
import { SectionGate } from "./shell/SectionGate";
import { AuditScreen } from "./audit/AuditScreen";
import { PeopleScreen } from "./people/PeopleScreen";
import { ItemsScreen } from "./sales/ItemsScreen";
import { ReportRangeProvider } from "./sales/report-range";
import { SalesScreen } from "./sales/SalesScreen";
import { SettingsScreen } from "./settings/SettingsScreen";
import { StockScreen } from "./stock/StockScreen";
import { TipsScreen } from "./tips/TipsScreen";
import { Button } from "../../shared/ui";
import { ZReportScreen } from "../../shared/zreport/ZReportScreen";

export function App() {
  return (
    // Matches the mount in web/web.go and the Vite base.
    <Router base="/backoffice">
      <Gate splash={<Splash />}>
        <Switch>
          <Route path="/pair">
            <PairScreen app="backoffice" />
          </Route>
          <Route path="/lock">
            <LockScreen />
          </Route>
          <Route>
            <BackOfficeShell>
              {/* Above the routes, so the days picked on one report are the days on the next. */}
              <ReportRangeProvider>
                <SignedInRoutes />
              </ReportRangeProvider>
            </BackOfficeShell>
          </Route>
        </Switch>
      </Gate>
    </Router>
  );
}

/** Every screen inside the shell, each behind the permission its section needs. */
function SignedInRoutes() {
  return (
    <Switch>
      <Route path="/menu">
        <SectionGate id="menu">
          <MenuScreen />
        </SectionGate>
      </Route>
      <Route path="/audit">
        <SectionGate id="audit">
          <AuditScreen />
        </SectionGate>
      </Route>
      <Route path="/sales/items">
        <SectionGate id="sales">
          <ItemsScreen />
        </SectionGate>
      </Route>
      <Route path="/sales">
        <SectionGate id="sales">
          <SalesScreen />
        </SectionGate>
      </Route>
      <Route path="/people">
        <SectionGate id="people">
          <PeopleScreen />
        </SectionGate>
      </Route>
      <Route path="/stock">
        <SectionGate id="stock">
          <StockScreen />
        </SectionGate>
      </Route>
      <Route path="/settings">
        <Redirect to="/settings/venue" replace />
      </Route>
      <Route path="/settings/:section">
        {(params) => (
          <SectionGate id="settings">
            <SettingsScreen path={`/settings/${params.section}`} />
          </SectionGate>
        )}
      </Route>
      <Route path="/devices">
        <SectionGate id="devices">
          <DevicesScreen />
        </SectionGate>
      </Route>
      <Route path="/printers">
        <SectionGate id="printers">
          <PrintersScreen />
        </SectionGate>
      </Route>
      <Route path="/reports/:number">
        {(params) => (
          <SectionGate id="zreports">
            <ReportRoute number={Number(params.number)} />
          </SectionGate>
        )}
      </Route>
      <Route path="/shifts/:id/tips">
        {(params) => (
          <SectionGate id="zreports">
            <TipsRoute id={Number(params.id)} />
          </SectionGate>
        )}
      </Route>
      <Route path="/reports">
        <SectionGate id="zreports">
          <ZReportsScreen />
        </SectionGate>
      </Route>
      <Route path="/floor">
        <SectionGate id="floor">
          <FloorScreen />
        </SectionGate>
      </Route>
      <Route path="/">
        <Dashboard />
      </Route>
      <Route>
        <Redirect to="/" replace />
      </Route>
    </Switch>
  );
}

/** A report by the number in its path; a path that is not one is sent back to the list. */
function ReportRoute({ number }: { number: number }) {
  const [, navigate] = useLocation();
  if (!Number.isSafeInteger(number) || number < 1) return <Redirect to="/reports" replace />;
  return (
    <ZReportScreen
      number={number}
      backLabel="All reports"
      onBack={() => navigate("/reports")}
      extraActions={(z) => <Button onClick={() => navigate(`/shifts/${z.shift_id}/tips`)}>Tips</Button>}
      footer={(z) => <ExportsPanel shiftId={z.shift_id} />}
    />
  );
}

/** A shift's tips by the id in its path. */
function TipsRoute({ id }: { id: number }) {
  if (!Number.isSafeInteger(id) || id < 1) return <Redirect to="/reports" replace />;
  return <TipsScreen shiftId={id} onBack={() => window.history.back()} />;
}
