# Phase 18 - The mez app for Android tablets

## Goal
A waiter's tablet and a kitchen display are an app, not a browser tab. The
tablet is installed from the venue's own PC, paired by scanning the QR the
Devices page already draws, and from then on it is mez: full screen, the
screen never sleeps in service, the kitchen chime rings without a tap, a
reboot comes back to mez, staff cannot wander into other apps, and a new
address from the router does not cost a pairing. The screens are the ones
the PC serves today; the app is a thin native shell around them (ADR-409).

## Depends on
ADR-338 (the device token lives in the origin's localStorage), ADR-367 (the
pairing QR carries the code in the fragment), ADR-379 (`mez.local`),
ADR-257 (the LAN is plain HTTP). Phase 12's installer and release, which
carry the app to the venue.

**The app's id is `com.mezpos.meztablet`**, the human's, chosen when the
project was made (2026-11-01). It is the app's identity for good: Google's
developer verification registers it, and a tablet treats another id as a
different app (uninstall, pair again). It does not change.

**The human's, before the app goes to the venue** (Task 2 went ahead on
minSdk 29, as made):
- **The tablets.** On each of the venue's tablets: the make and model,
  Settings > About tablet > Android version, and Settings > Apps > (show
  system apps) Android System WebView's version, and whether it has the Play
  Store. These set `minSdk` (29, Android 10, as made) and the oldest WebView
  the app accepts.

**Walking the app on the developer's PC** (2026-11-02): the Android 17
emulator image aborts in its graphics driver whenever the screen is read
back (ADR-412), so `adb screencap` fails and Android's Settings app crashes
there. The AVD `mez_tablet_33` (Pixel Tablet, Android 13) walks cleanly;
`mez_tablet_37` shows the local network permission, photographed from
Windows. A real tablet is the better check of both.

**The human's, before Task 7:** the release signing key, made once
(`keytool`), kept out of the repository and backed up where the venue's
backups go. Lose it and every installed tablet must uninstall the app and
pair again. Then, before Google's verification reaches India (2027 at the
earliest): the developer registration ($25), naming the app's id and this
key.

## Where the code lives
`android-app/`: Kotlin, Gradle, the Android SDK; nothing else in the
repository builds it. The server's side (serving the app, its version, the
manifest for browsers) is `internal/api`, `web/` and `internal/device`.
Words in `docs/hardware.md`, `docs/deploy.md` and `docs/releasing.md`.

## What stays true
- **The screens are the server's.** The app draws no screen of mez's. Its
  own pages (first run, "Can't reach the mez PC", the way out) are HTML in
  its assets, in the tokens of `docs/design/tokens.md`, and Kotlin is
  plumbing.
- **Offline is the default.** No Google account, no Play services, no
  internet: not to install, not to pair, not in service. The venue's wifi
  and the PC are all the app needs.
- **`make check` and the Go build never need Java, Gradle or the Android
  SDK**, and `CGO_ENABLED=0` cross-compiles as before. The app has its own
  `make android` and its own CI job.
- **The browser stays a way in.** A tablet or phone in a browser works
  exactly as now. The web app never requires the app; where it asks the app
  for something, it does without when the app is not there.
- **The app keeps no PIN and no staff session.** Signing in is the web app's,
  checked by the server, as now. What the app keeps is where the PC is and,
  per Task 5's ADR, what carries the pairing across an address change.
- Every task is seen on a real Android tablet, or an emulator where the task
  says so, before it is ticked.

## Tasks

- [x] **Task 1 - ADR-409 and this phase.** Done 2026-11-01. Native Kotlin,
  the screens the server's, `make check` untouched, the browser kept; React
  Native, Flutter, Capacitor, a TWA, every HTTPS-on-the-LAN route and a
  kiosk browser as the product are rejected there, with the research that
  rejected them.

- [x] **Task 2 - The shell builds, as mez.** Done 2026-11-02 (ADR-412), on
  minSdk 29 as made, before the tablets' versions were read. Seen on an
  Android 13 tablet emulator (the order sent) and an Android 17 one (the
  permission's every path); not yet on a real tablet. The project had no activity
  yet (made that way): the app's one activity, holding one `WebView`, in
  the package `com.mezpos.meztablet`. The label "mez" (`app_name`,
  `rootProject.name`), the mez mark as its icon (from `web/`'s, as
  adaptive icon layers). The template's AppCompat and Material stay only if
  something native needs them (the app's own pages are HTML, so likely
  not; `ComponentActivity` is enough). Release builds with R8 on. Cleartext allowed by a network security config (the LAN is plain
  HTTP, ADR-257). Android 17's local network permission, which the app
  needs because it targets SDK 37, asked for on first run with words that
  say why. The WebView opens an address set in a debug build. `make
  android` (assemble debug, unit tests, lint) and a CI job beside the Go
  one. **Seen:** the debug APK on an emulator showing a demo venue's till
  at `10.0.2.2:<port>`, signed in, an order sent.

- [x] **Task 3 - A tablet that stays a till.** Done 2026-11-02 (ADR-413).
  Seen on an Android 13 tablet emulator, the chime included (no tap, after a
  reload: in WebView the `AudioContext` runs whatever the media setting, so
  no fix was owed); not yet on a real tablet. Full screen (system bars
  hidden, back by a swipe); `FLAG_KEEP_SCREEN_ON`; back goes back in the
  WebView and never leaves the app from the first screen. WebView settings
  chosen and written down: JavaScript and DOM storage on, media without a
  gesture, text zoom fixed at 100 so the tablet's font size cannot break a
  layout, no pinch zoom, no file or content access. The renderer dying
  (`onRenderProcessGone`) brings the WebView back instead of the app
  crashing. An address that does not answer shows the app's own "Can't
  reach the mez PC" page, which retries by itself and has Try again.
  **Seen on a tablet:** the kitchen chime rings after a reload with no tap
  (`kitchen/use-chime.ts`); if `AudioContext` still starts suspended, the
  fix is found and written in an ADR.

- [ ] **Task 4 - Paired by scanning.** First run is the app's page "Scan
  the pairing code". The camera reads the QR the Devices page draws
  (ADR-367: `http://<address>:<port>/<path>#code=XXXX-XXXX`) with a
  scanner that needs no Play services (ZXing). The app accepts only that
  shape: http or https, a private address or `mez.local`, a path the
  server's `appPaths` names (the till, `/kitchen`); anything else is
  refused with words. It keeps the address and path and opens the link; the
  web app pairs itself from the fragment, as in a browser. Typing the
  address by hand is the second way in. Unit tests on the link's checks.
  **Seen:** a till and a kitchen display paired on a tablet from a demo
  venue's Devices page.

- [ ] **Task 5 - The PC found again.** ADR first. When the kept address
  stops answering, the app looks for the PC (`mez.local` by a one-shot
  multicast DNS query, which the announcer answers, with a
  `WifiManager.MulticastLock`; or a DNS-SD service the server would add to
  `internal/device/announce`) and goes back to it. The pairing must
  survive: the token is in the origin's localStorage (ADR-338), and a new
  address is a new origin. The ADR chooses between handing the token to the
  new origin before the page's scripts run (`addDocumentStartJavaScript`),
  a bridge the web app asks at boot, or one origin that never changes; and
  says what the web app's storage keys then promise (`web/shared/auth/
  storage.ts`, `mez.<app>.device`). Tests on both sides. **Seen:** a paired
  tablet, the PC given a new address, the till back without a pairing.

- [ ] **Task 6 - Locked to mez.** The app as the tablet's home app, so the
  home button and a reboot come back to it. Lock task mode when the app is
  the device owner (`adb shell dpm set-device-owner`, on a tablet with no
  accounts, written up for the human); screen pinning where it is not. The
  way out (change venue, Android's settings, leave the app) is behind a
  long press and a manager's PIN checked by the server, the way the till
  checks it; never a PIN the app keeps. **Seen:** a reboot lands in mez;
  home, recents and the notification shade do not leave it.

- [ ] **Task 7 - Installed from the venue's PC.** ADR first, on how the
  APK reaches the PC: the release builds and signs it and the Go build
  embeds it under a build tag, or the installer carries it beside the
  binary. The server serves it at a fixed path with its version; the
  Devices page draws "Install the mez app" as a QR beside the pairing
  QR, and says plainly that Android will ask to allow installs from the
  browser. A build without the app says so and draws no QR. Tests in
  `internal/api`. **Seen:** a tablet that has never seen mez gets the app
  from the PC's page and pairs, with the internet unplugged.

- [ ] **Task 8 - Updates.** The app reads the version the PC serves and,
  when it is newer, offers the update at a quiet moment (never over an open
  order: the web app tells it when it is idle, or the app asks); the person
  confirms Android's install. As device owner, the app installs it itself
  between services. **Seen:** an older app on a tablet takes the PC's newer
  one.

- [ ] **Task 9 - The browser path, for the iPad.** A web manifest and icons
  for the till and the kitchen display, `apple-touch-icon` and the
  standalone meta, the theme colour from the tokens. Nothing changes for the
  app. **The human's:** on an iPad over plain HTTP, Add to Home Screen opens
  full screen; with Guided Access and Auto-Lock at Never it stays there
  (iPadOS 26).

- [ ] **Task 10 - The words.** `hardware.md`'s tablets: the app (install,
  pair, device owner for a kiosk, the way out), the iPad's way, and the
  stopgap for a tablet that cannot take the app yet (a kiosk browser such
  as FreeKiosk or Fully Kiosk pointed at the PC's address, not `mez.local`:
  Chrome's "Always use secure connections" spares private addresses).
  `releasing.md`: the signing key, where it is kept, and building the app
  for a release. `deploy.md`: a line on tablets.

- [ ] **Task 11 - A spike: the PC's own certificate.** The LAN's plain HTTP
  carries PINs and tokens in clear (ADR-257). On a tablet: mez serves HTTPS
  on a second port with a certificate it makes at setup; the pairing QR
  carries its SHA-256; the app accepts that certificate and no other in
  `onReceivedSslError`. Find whether the page, `fetch` and the WebSocket all
  go through it, and how a new certificate reaches a paired tablet. An ADR
  either way; the build, if it holds, is a later phase.

## Done when
- On the venue's own Android tablets: the app is installed from the PC's
  Devices page and paired by scanning, with the internet unplugged.
- A service is run on it: an order from the waiter's tablet, the kitchen
  chime with nobody tapping, the screen awake throughout.
- A tablet restarted comes back to mez; home and recents do not leave it.
- The PC given a new address: every tablet is back on its till without a
  pairing.
- A tablet in a browser works as before, and `make check` passes on a
  machine with no Java.
