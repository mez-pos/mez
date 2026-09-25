// The back office's routes, under /backoffice. Sign-in is the same staff PIN
// as the terminal, on a device paired as a back office (ADR-056).

import { Link, Redirect, Route, Router, Switch } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { SignedInShell } from "../../shared/screens/SignedInShell";
import { Splash } from "../../shared/screens/Splash";
import { PrintersScreen } from "./printers/PrintersScreen";

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
          <Route path="/">
            <SignedInShell area="Back office" exitLabel="Sign out">
              <h1 className="font-serif text-total">Back office</h1>
              <nav aria-label="Settings" className="mt-6">
                <Link
                  href="/printers"
                  className="inline-flex min-h-tap items-center rounded border border-line-strong bg-raised px-4 text-base font-semibold"
                >
                  Printers
                </Link>
              </nav>
              <p className="mt-4 max-w-prose text-base text-fg-2">Menus, staff and reports arrive in phase 10.</p>
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
