// The back office's routes, under /backoffice. Sign-in is the same staff PIN
// as the terminal, on a device paired as a back office (ADR-056).

import { Link, Redirect, Route, Router, Switch, useLocation } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { SignedInShell } from "../../shared/screens/SignedInShell";
import { Splash } from "../../shared/screens/Splash";
import { FloorScreen } from "./floor/FloorScreen";
import { PrintersScreen } from "./printers/PrintersScreen";
import { ZReportsScreen } from "./reports/ZReportsScreen";
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
          <Route path="/printers">
            <SignedInShell area="Printers" exitLabel="Sign out">
              <PrintersScreen />
            </SignedInShell>
          </Route>
          <Route path="/reports/:number">
            {(params) => (
              <SignedInShell area="Z-report" exitLabel="Sign out">
                <ReportRoute number={Number(params.number)} />
              </SignedInShell>
            )}
          </Route>
          <Route path="/shifts/:id/tips">
            {(params) => (
              <SignedInShell area="Tips" exitLabel="Sign out">
                <TipsRoute id={Number(params.id)} />
              </SignedInShell>
            )}
          </Route>
          <Route path="/reports">
            <SignedInShell area="Z-reports" exitLabel="Sign out">
              <ZReportsScreen />
            </SignedInShell>
          </Route>
          <Route path="/floor">
            <SignedInShell area="Floor plan" exitLabel="Sign out">
              <FloorScreen />
            </SignedInShell>
          </Route>
          <Route path="/">
            <SignedInShell area="Back office" exitLabel="Sign out">
              <h1 className="font-serif text-total">Back office</h1>
              <nav aria-label="Settings" className="mt-6 flex flex-wrap gap-tap-gap">
                {[
                  { href: "/floor", label: "Floor plan" },
                  { href: "/printers", label: "Printers" },
                  { href: "/reports", label: "Z-reports" },
                ].map((l) => (
                  <Link
                    key={l.href}
                    href={l.href}
                    className="inline-flex min-h-tap items-center rounded border border-line-strong bg-raised px-4 text-base font-semibold"
                  >
                    {l.label}
                  </Link>
                ))}
              </nav>
              <p className="mt-4 max-w-prose text-base text-fg-2">Menus and staff arrive in phase 10.</p>
            </SignedInShell>
          </Route>
          <Route>
            <Redirect to="/" replace />
          </Route>
        </Switch>
      </Gate>
    </Router>
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
    />
  );
}

/** A shift's tips by the id in its path. */
function TipsRoute({ id }: { id: number }) {
  if (!Number.isSafeInteger(id) || id < 1) return <Redirect to="/reports" replace />;
  return <TipsScreen shiftId={id} onBack={() => window.history.back()} />;
}
