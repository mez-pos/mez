// The terminal's routes. Every path goes through the Gate, which sends an
// unpaired device to /pair and a locked one to /lock (shared/auth/route.ts).

import { Redirect, Route, Switch } from "wouter";
import { Gate } from "../../shared/auth/Gate";
import { LockScreen } from "../../shared/screens/LockScreen";
import { PairScreen } from "../../shared/screens/PairScreen";
import { SignedInShell } from "../../shared/screens/SignedInShell";
import { Splash } from "../../shared/screens/Splash";

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
        <Route path="/">
          <SignedInShell area="Terminal" exitLabel="Lock">
            <h1 className="font-serif text-total">Register</h1>
            <p className="mt-4 max-w-prose text-base text-fg-2">The register arrives in phase 06.</p>
          </SignedInShell>
        </Route>
        <Route>
          <Redirect to="/" replace />
        </Route>
      </Switch>
    </Gate>
  );
}
