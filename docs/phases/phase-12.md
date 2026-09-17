# Phase 12 — Ship it

## Goal
A cafe owner downloads one file, runs it, and is selling in under ten minutes.

## Depends on
Phase 11.

## Tasks

- [ ] **Task 1 — Windows Service.**
  `kardianos/service` wrapping the server. Install, uninstall, start, stop as
  subcommands. Auto-start on boot, restart on crash. Logs to the Windows event
  log as well as the file log. Runs under an account that can reach the printer.

- [ ] **Task 2 — Installer.**
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

- [ ] **Task 4 — Backups.**
  Litestream integrated: continuous replication to S3, R2 or a local folder,
  configured in back office. A manual "back up now" button. A documented,
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
