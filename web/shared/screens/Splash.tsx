// Shown while the stored device is checked with the server, so no screen
// renders against a guess. On a LAN this is a blink.

import { Meta } from "../ui";

export function Splash() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-2 bg-canvas text-fg">
      <p className="font-serif text-total">mez</p>
      <Meta items={["Checking this device"]} />
    </main>
  );
}
