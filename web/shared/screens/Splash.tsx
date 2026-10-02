// Shown while the stored device is checked with the server, so no screen
// renders against a guess. On a LAN this is a blink.

import { Logo, Meta } from "../ui";

export function Splash() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 bg-canvas text-fg">
      <Logo className="h-14" />
      <Meta items={["Checking this device"]} />
    </main>
  );
}
