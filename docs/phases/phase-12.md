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

- [ ] **Task 3 — First-run wizard.**
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
  - [ ] **Task 3d - The last screen, and the hand-off.**
    The LAN URL and a pairing code for a second device, in large type; the
    back office paired on this machine (`docs/KNOWN.md`, ADR-144's entry).
    The installer's last page and `PairScreen`'s words stop naming
    `mez setup`; `running.md` and `deploy.md` say the wizard first. Walked in
    a browser from an empty data directory.

- [ ] **Task 4 — Backups.**
  Litestream integrated: continuous replication to S3, R2 or a local folder,
  configured in back office (this is phase 10's Task 6l, moved here,
  ADR-141: a Backups section of the back office settings screen, over a
  server write path in the settings service, `settings.edit`; it also
  covers `<data dir>/photos/`, ADR-110). A manual "back up now" button. A documented,
  tested restore procedure — **actually restore from a backup and verify it**,
  do not just write the docs.

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
