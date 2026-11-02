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

- [x] **Task 4 - Paired by scanning.** Done 2026-11-02 in two parts (ADR-414,
  ADR-415), seen on emulators; a real tablet's camera is owed. First run is the app's page "Scan the
  pairing code". The camera reads the QR the Devices page draws (ADR-367:
  `http://<address>:<port>/<path>#code=XXXX-XXXX`) with a scanner that needs
  no Play services (ZXing). The app accepts only that shape: http or https,
  a private address or `mez.local`, a path the server's `appPaths` names
  (the till, `/kitchen`); anything else is refused with words. It keeps the
  address and path and opens the link; the web app pairs itself from the
  fragment, as in a browser. Typing the address by hand is the second way
  in. Unit tests on the link's checks. **Seen:** a till and a kitchen
  display paired on a tablet from a demo venue's Devices page. Split in two
  on 2026-11-02, the camera apart:

  - [x] **Task 4a - The address, checked, kept and typed.** Done 2026-11-02
    (ADR-414), seen on Android 13 and 17 emulators; an address is kept only
    once mez answers there (a review's find). The link's
    checks, a pure function with unit tests: http or https, a private IPv4
    address or `mez.local`, the till or `/kitchen` (the back office refused
    with its own words), an optional `#code=XXXX-XXXX` and nothing else.
    The app keeps the PC's address and path, never the code, and opens the
    link once. First run is the app's page "Pair this tablet", which until
    4b takes the address typed as the PC shows it; a refusal says why on
    the same page. Android 17's local network page comes after the address,
    before the PC is opened, the code kept in memory across it. "Can't
    reach the mez PC" gets "Pair again". A debug build no longer opens an
    address by itself (`-PmezUrl` still seeds one). **Seen:** on an
    emulator, a till and a kitchen display paired by typing their
    addresses, the refusals' words, and the kept address opened on the
    next start.

  - [x] **Task 4b - Scanned.** Done 2026-11-02 (ADR-415), seen on an Android
    13 emulator fed the QR as a picture; not yet through a real tablet's
    camera. The camera on the "Pair this tablet" page:
    CameraX and ZXing's core, no Play services; the camera permission asked
    with words, a refusal leaving typing; the app's page over the preview
    (its words and Cancel in HTML, the preview native); a scanned link
    through 4a's checks. **Seen:** a till and a kitchen display paired by
    scanning a demo venue's Devices page, on an emulator whose camera is
    fed the QR as an image (`-camera-back imagefile:`), and on a real
    tablet when one is to hand.

- [x] **Task 5 - The PC found again.** Done 2026-11-02 in three parts
  (ADR-416, ADR-417, ADR-418), seen on an emulator; a real tablet on a
  venue's wifi is owed. ADR first. When the kept address
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
  Split in two on 2026-11-02 (ADR-416), the finding apart: the emulator's
  network carries no multicast, so the carry is walked by hand first.

  - [x] **Task 5a - The pairing carried.** Done 2026-11-02 (ADR-416), seen
    on an Android 13 emulator: a till carried twice, as the same device,
    and another venue's mez given nothing; not yet on a real tablet, nor a
    kitchen display. Health says which venue
    it serves (its public id, `venue`), to the venue's network only, from
    what the server holds: no database. The app keeps the venue with the
    address once mez answers there. "Pair again" keeps where the pairing is
    held; the PC's new address typed or scanned, its mez saying the same
    venue, carries the pairing: the old address's `mez.` localStorage keys
    written at the new one by the app's own empty page at each, before the
    PC's page opens there. Tests in `internal/api`, the app (the carry's
    checks and scripts), and the web app (the keys' prefix). **Seen:** on an
    emulator, a paired till, "Pair again", the same PC at another address
    typed: the till at its sign-in screen, the same device, no code.

  - [x] **Task 5b - The PC proves itself.** Done 2026-11-02 (ADR-417),
    seen on an Android 13 emulator: an impostor refused, the real PC at a
    new address proven and carried to, a proof across a NAT refused; not
    yet on a real tablet. ADR-417, split from the
    finding on 2026-11-02: the venue's id is no proof, and a found address
    must not be handed the pairing on its word. Before any carry, the new
    address answers `GET /api/v1/devices/{id}/proof?nonce=&at=`: an HMAC
    over the nonce and the address asked, keyed by what the server stores
    of the device's token, made only when `at` is the address the request
    reached. The app reads the device's id and token at the old address,
    makes the same, and carries only on a match; otherwise the new address
    opens at the pairing screen. Tests in `internal/auth`, `internal/api`
    and the app (the message, the key, the answer read). **Seen:** on an
    emulator, a till carried to the PC's LAN address with the proof; a
    proof refused for another address; a mez that answers health with the
    venue's id but cannot prove given nothing.

  - [x] **Task 5c - The PC found by itself.** Done 2026-11-02 (ADR-418),
    seen on an Android 13 emulator through a relay to the real announcer,
    and the query run on the PC against it; not yet on a real tablet.
    ADR-416 and ADR-417. While
    the PC's page shows, a health check now and then; when the kept address
    does not answer (that, or "Can't reach the mez PC"), one legacy mDNS
    query for `mez.local` from an ordinary port, the kept port, each address
    asked its health, the kept venue's taken, then 5b's proof: 5a's carry,
    then the till. The packet's making and reading are pure functions with
    unit tests. **Seen:** on a real tablet on the venue's wifi (or the
    emulator with the answer fed, and the query checked from the PC against
    the real announcer), the PC given a new address, the till back by
    itself.

- [x] **Task 6 - Locked to mez.** Done 2026-11-02 in two parts (ADR-419,
  ADR-420). The app as the tablet's home app, so the
  home button and a reboot come back to it. Lock task mode when the app is
  the device owner (`adb shell dpm set-device-owner`, on a tablet with no
  accounts, written up for the human); screen pinning where it is not. The
  way out (change venue, Android's settings, leave the app) is behind a
  long press and a manager's PIN checked by the server, the way the till
  checks it; never a PIN the app keeps. **Seen:** a reboot lands in mez;
  home, recents and the notification shade do not leave it.
  Split in two on 2026-11-02 (ADR-419), the way out first, so that no
  tablet is ever locked without one.

  - [x] **Task 6a - The way out.** Done 2026-11-02 (ADR-419), seen on an
    Android 13 emulator (two fingers through its gRPC controller): the
    owner alone listed, a wrong PIN refused, her PIN opening Android's
    settings, the PC down saying so; a cashier refused in the API's tests,
    which the page never lists. ADR-419. Two fingers held still for
    three seconds anywhere: the page's touch cancelled, the device's token
    read from the till's page, and the app's page "Let this tablet out":
    who may (a role with `device.manage`, from
    `GET /api/v1/devices/me/way-out`), and their PIN, checked by the PC
    (`POST /api/v1/devices/me/way-out`, the till's `Authorise`, its
    lockout). Opened: Android's settings, pair again, back to mez. No PC,
    no PIN: the page says so. Tests in `internal/api` and the app (the
    hold, the answers read). **Seen:** on an emulator, the hold made, a
    cashier refused, a wrong PIN refused, a manager's PIN opening Android's
    settings.

  - [x] **Task 6b - Locked.** Done 2026-11-02 (ADR-420), seen on an
    Android 13 emulator: pinned when not the device owner; as the device
    owner, Home, Recents and the shade staying in mez, a reboot landing
    in the till locked, the Wi-Fi panel inside the lock, "Leave mez" to
    the tablet's launcher and Home back to the till locked, "Release it"
    undoing the kiosk. "Pair again" on "Can't reach the mez PC" keeps
    needing no PIN (ADR-420 says why). ADR-419. The app as the tablet's home app;
    lock task mode when the app is the device owner (`adb shell dpm
    set-device-owner`, on a tablet with no accounts, written up for the
    human), the keyguard off, the power menu kept; screen pinning where it
    is not; the way out's "Leave mez" lifting the lock; what a locked
    tablet whose PC does not answer offers (the Wi-Fi settings). Locked,
    the app's own ways round the PIN are closed: "Pair again" on "Can't
    reach the mez PC" and Open Settings on the permission pages go behind
    the way out (ADR-419's review). **Seen:** a reboot lands in mez; home,
    recents and the notification shade do not leave it; the way out
    leaves it.

- [x] **Task 7 - Installed from the venue's PC.** Done 2026-11-02 in three
  parts (ADR-421). ADR first, on how the
  APK reaches the PC: the release builds and signs it and the Go build
  embeds it under a build tag, or the installer carries it beside the
  binary. The server serves it at a fixed path with its version; the
  Devices page draws "Install the mez app" as a QR beside the pairing
  QR, and says plainly that Android will ask to allow installs from the
  browser. A build without the app says so and draws no QR. Tests in
  `internal/api`. **Seen:** a tablet that has never seen mez gets the app
  from the PC's page and pairs, with the internet unplugged.
  Split in three on 2026-11-02 (ADR-421): the PC's side first, then the
  release that builds the app, then the page and the walk.

  - [x] **Task 7a - The app served.** Done 2026-11-02 (ADR-421), seen
    with the app's debug build put in place: its version, the package
    (ranges, the ETag), a till's pairing code carrying its QR; and the
    same server built without it saying so. ADR-421. The package and its
    `app.json` embedded under the build tag `tabletapp`
    (`internal/device/tabletapp`); `GET /app/mez.apk` and its version at
    `GET /api/v1/app`, no token, at the venue only; a build without the
    app says so (its log, 404 `app_not_built`); the pairing code's places
    carry the app's address and QR for a till or a kitchen display, and
    the code says which app the build carries. `make build` and `make
    dist` embed it when it is there. Tests in `internal/api` and the
    package's own. **Seen:** a build with a package put in place serves
    it (ranges, the ETag), its version and the pairing code's app QR; one
    without says so.

  - [x] **Task 7b - The release builds and signs it.** Done 2026-11-02
    (ADR-421), with a throwaway key: signed, its code the build number,
    carried by `make dist` and served; the release build installed on the
    emulator and paired. The workflow as written, not run. ADR-421. A signed
    release build from Gradle with the key from the environment (never in
    the repository), the version name `git describe` and the code the
    build number (ADR-410), `app.json` written beside it into
    `internal/device/tabletapp/app/`; an Android job in `release.yml`
    before `binaries`, from the repository's secrets, the binaries and the
    Docker image embedding what it made, the package and its `app.json`
    written together; a release without the key carries no app and its
    notes say so. CI builds with the tag against a stand-in package, since
    no `make check` compiles `embed_app.go` (7a's review). `docs/releasing.md`: the key, the
    secrets. **Seen:** a release build signed with a throwaway key, its
    version code the build number, embedded in a `make dist` binary that
    serves it.

  - [x] **Task 7c - On the Devices page.** Done 2026-11-02 (ADR-421),
    seen in a browser at both sizes, with and without the app, and on
    an emulator that had never seen mez, its name lookups sent nowhere:
    the app downloaded from the PC, installed, and paired. ADR-421.
    Pairing a till or a
    kitchen display: "Install the mez app" with its QR beside the pairing
    QR, and plainly that Android will ask to allow installs from the
    browser; a build without the app says so and draws no QR. **Seen:** a
    tablet that has never seen mez gets the app from the PC's page and
    pairs, with the internet unplugged.

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
