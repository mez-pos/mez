# Phase 12 — Ship it

## Goal
A cafe owner downloads one file, runs it, and is selling in under ten minutes.

## Depends on
Phase 11.

## Tasks

- [x] **Task 1 — Windows Service.**
  `kardianos/service` wrapping the server. Install, uninstall, start, stop as
  subcommands. Auto-start on boot, restart on crash. Logs to the Windows event
  log as well as the file log. Runs under an account that can reach the printer.

- [x] **Task 1b - The service's own account, and restarts that back off.**
  Added after Task 1 (ADR-264). The service runs as `NT SERVICE\mez`, not
  LocalSystem; the data directory's ACL names that account; the restarts
  wait 5 seconds, 30 seconds, then 5 minutes each time.

- [x] **Task 2 — Installer.**
  Inno Setup. Installs the binary, registers the service, creates a desktop
  shortcut launching Edge or Chrome with `--app=http://localhost:7777 --kiosk`,
  opens the firewall port for LAN devices, and runs a first-run setup wizard.
  Clean uninstall that offers to keep the data directory. Signed if a
  certificate is available; if not, document the SmartScreen warning honestly in
  the README rather than letting users hit it cold.

- [x] **Task 3 — First-run wizard.**
  Venue name, currency, timezone, tax profile, financial year start, invoice
  series prefix, first admin staff member and PIN, printer discovery and test
  print. At the end: the LAN URL and a pairing code for a second device, on
  screen, in large type.

  Split on 2026-10-07, before any of it was built: it is an endpoint that
  goes around ADR-058, a screen with a design (D24), a search of the
  network, and the installer's hand-off. Four sessions, not one.

  - [x] **Task 3a - First run over HTTP, from this computer only.**
    What `mez setup` does, as a service both it and the API call:
    `GET /api/v1/setup` says whether the venue is still to be made,
    `POST /api/v1/setup` makes it and answers with the first pairing code.
    Only a browser on the server's own machine may post; ADR-058 gives way
    that far and no further. No screen.
  - [x] **Task 3b - The wizard's questions (D24).**
    A spec first (`docs/design/screens/setup-wizard.md`). The till opens on
    the wizard while there is no venue: venue name, currency, timezone, tax
    profile and registration, financial year start, invoice series prefix,
    the first admin and their PIN. It posts 3a, pairs this device with the
    code, signs the admin in, and sets the prefix through the settings API.
    D24's tables and menu import steps are not in this task's list and are
    left out.
  - [x] **Task 3c - Printer discovery and a test print.**
    A search of the LAN for port 9100 and of Windows' own printers, behind
    `printers.manage`; the wizard's printer step adds the one chosen as the
    receipt printer and prints a test page. Skippable: a venue with no
    printer yet still finishes.
  - [x] **Task 3d - The last screen, and the hand-off.**
    The LAN URL and a pairing code for a second device, in large type; the
    back office paired on this machine (`docs/KNOWN.md`, ADR-144's entry).
    The installer's last page and `PairScreen`'s words stop naming
    `mez setup`; `running.md` and `deploy.md` say the wizard first. Walked in
    a browser from an empty data directory.

  Two more, added on 2026-10-07 after 3d, from what 3a to 3d left open
  and the owner chose to close before backups:

  - [x] **Task 3e - The setup token.**
    The venue can be made over HTTP only by a browser holding a token kept
    in the data directory (ADR-270): the installer hands it over in the
    address it opens, `mez setup-link` prints it, a server started in a
    terminal says it. Closes `STATE.md`'s question 9.
  - [x] **Task 3f - A QR code for the address, and addresses by name.**
    The wizard's last step draws the address as a QR code a tablet's
    camera can open (`internal/domain/qr` already draws them), and names
    each address by its network adapter, with virtual adapters last and no
    guess between two real ones. The printer search skips virtual
    adapters too.

- [ ] **Task 4 — Backups.**
  Litestream integrated: continuous replication to S3, R2 or a local folder,
  configured in back office (this is phase 10's Task 6l, moved here,
  ADR-141: a Backups section of the back office settings screen, over a
  server write path in the settings service, `settings.edit`; it also
  covers `<data dir>/photos/`, ADR-110). A manual "back up now" button. A documented,
  tested restore procedure — **actually restore from a backup and verify it**,
  do not just write the docs.

  Split on 2026-10-07, before any of it was built: it is a new dependency
  that sits on the database's write-ahead log, two kinds of destination, a
  folder of files the database does not hold, a write path with a secret
  in it, a screen, and a restore that has to be done for real. Six
  sessions, not one.

  - [x] **Task 4a - Litestream in the binary, to a folder.**
    `internal/backup` over Litestream as a library (no second program, no
    cgo): started with the server and stopped with it, replicating
    `mez.db` to a folder named in a file in the data directory. Back up
    now, a status (when it last reached the destination, and what went
    wrong), and a restore into a new file, tested by restoring a database
    that was being written to and comparing it. No API, no screen, no S3.
  - [x] **Task 4b - S3 and R2.**
    The second kind of destination: bucket, path, region, endpoint (R2 and
    the other S3-compatibles), key and secret. The AWS SDK comes with it.
    Tested against an S3 that answers on this machine; say plainly if no
    real bucket was written to.
  - [x] **Task 4c - The photos folder (ADR-110).**
    `<data dir>/photos/` copied beside the database's backup, to a folder
    and to a bucket: files named by their hash, so only the missing ones
    go, on the same clock and on "back up now". Restore brings them back.
  - [x] **Task 4d - The write path and the API.**
    The settings service reads and writes the destination under
    `settings.edit`, checks it can be written to before keeping it, and
    restarts replication with no restart of the server. The secret goes in
    and never comes back out. `GET` and `PUT /api/v1/settings/backup`,
    `POST /api/v1/settings/backup/now`, the status with them, generated
    types.
  - [x] **Task 4e - The Backups section of settings.**
    The spec first (`docs/design/screens/backoffice-settings.md`, its
    Backup row stops saying Soon). Off, a folder, or a bucket; the status
    in colour and a word; "Back up now". Walked in a browser.
  - [ ] **Task 4f - Restore, done and written down.**
    `mez restore`: from a destination into an empty data directory, the
    database and the photos, refusing to write over a venue. Then do it:
    back up a venue that has sold, restore it on another data directory,
    start mez on it, and check the invoices, the next invoice number and
    the photos. `deploy.md`'s Backups section is rewritten from what was
    done, and `running.md`, the README and `KNOWN.md` stop saying phase 12.

- [ ] **Task 5 — Docker and Linux.**
  A small multi-stage image. `docker-compose.yml` with a volume and the relay
  optional. A systemd unit file for bare-metal Linux. Same binary, no second
  codebase.

- [ ] **Task 6 — Release automation.**
  GoReleaser or an equivalent Actions workflow: tag → cross-compiled binaries,
  the Windows installer, the Docker image, checksums, and a changelog.

- [ ] **Task 7 — Documentation.**
  README with a screenshot and a sixty-second pitch. Install guides for Windows,
  Docker and Linux. Hardware compatibility notes on printers and drawers. A
  self-hosted relay guide. CONTRIBUTING with the architecture overview.
  Troubleshooting for the five failures that will actually happen: printer not
  found, device cannot reach the server, SmartScreen warning, tunnel down,
  drawer will not kick.

## Done when
A clean Windows machine goes from downloaded installer to first printed invoice
in under ten minutes, with no terminal window and no documentation beyond the
wizard.
