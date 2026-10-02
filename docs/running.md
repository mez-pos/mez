# Running mez

mez is one binary. It needs no database server, no Docker and no
internet connection. Put it on the machine behind the counter, start it, and it
serves the register on port 7777.

This page covers starting it, where it keeps your data, and how to change the
port. It describes the server as it exists today, at the end of Phase 01: it
serves and it answers health checks, but it does not take orders yet.

## Getting a binary

Download the one for your platform, or build it:

```sh
make dist
```

That produces three binaries in `dist/`, none of which need anything installed
alongside them:

| File | Runs on |
|---|---|
| `mez_<version>_windows_amd64.exe` | Windows 10 and 11, 64-bit |
| `mez_<version>_linux_amd64` | Linux, 64-bit |
| `mez_<version>_darwin_arm64` | macOS, Apple silicon |

Building needs Go 1.23 or newer and a shell with `sh` on the path. On Windows
that means Git Bash or WSL, not `cmd.exe` or PowerShell.

Two builds of the same commit produce byte-identical binaries, so you can check
that what you are about to run matches the source it claims to come from.

## Setting up, once

Before the first run, create the venue, its owner and the code that pairs the
first device. Do this on the machine itself: being sat at it is what the
command trusts, because nothing else exists yet to authorise it.

```sh
./mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata
```

It prints the venue, the owner, a PIN it drew for them, and a pairing code
that works for ten minutes:

```
  Database    /var/lib/mez/mez.db
  Venue       ven_7K3M9QX2
  Owner       stf_A1B2C3D4
  Owner PIN   418362   (drawn now: write it down, then change it in the back office)
  Pairing code 6QG8-3BA9   (expires 09:44)
```

Write the PIN down: it is shown here and never again. Pass `-owner-pin` to
choose one instead, and it is not printed at all.

A venue that charges tax says so here, since an invoice cannot be issued
without it:

```sh
./mez setup -name "Filter Coffee House" -owner Priya   -tax-profile gst_in -tax-registration 29ABCDE1234F1Z5
```

`./mez setup -h` lists the rest: currency, financial year, the legal name
for invoices, and what the first device is called. Running it a second time on
the same database refuses: one machine serves one venue.

## Starting it

```sh
./mez
```

It prints a line per event to the terminal and stops on Ctrl+C, finishing any
request that is already in flight before it exits.

Then open `http://localhost:7777` on the same machine, or
`http://<the machine's address>:7777` from a tablet on the same network.

Three addresses are served:

| Address | What it is |
|---|---|
| `/` | The register, for staff |
| `/backoffice` | Menu, reports and settings, for the owner |
| `/order` | The guest ordering page, opened from a QR code |

Until Phase 05 these show a placeholder page rather than the real apps. The
server says so at startup, on the line reading `serving placeholder frontends`.

## Where your data lives

Everything the server owns, including the database, the logs and the backups,
sits in one directory:

| Platform | Default |
|---|---|
| Windows | `C:\ProgramData\mez` |
| Linux and macOS | `./data`, beside wherever you started it from |

Back up that directory and you have backed up the venue.

Logs are in `logs/` inside it, as `mez.log`. The current file is rotated to
`mez.log.1` once it reaches 8 MB, and five rotations are kept, so the logs
never grow past about 48 MB.

## Changing the port, and everything else

There are four places a setting can come from. Each one beats the ones above it:

1. The built-in defaults
2. `config.toml`, beside the binary
3. Environment variables
4. Command-line flags

So, to move the server to port 8080, any of these will do:

```sh
./mez -listen-addr :8080
```

```sh
MEZ_LISTEN_ADDR=:8080 ./mez
```

```toml
# config.toml, next to the binary
listen_addr = ":8080"
```

The full set of settings:

| Setting | Flag | Environment | Default |
|---|---|---|---|
| `listen_addr` | `-listen-addr` | `MEZ_LISTEN_ADDR` | `:7777` |
| `data_dir` | `-data-dir` | `MEZ_DATA_DIR` | platform, see above |
| `log_level` | `-log-level` | `MEZ_LOG_LEVEL` | `info` |
| `log_format` | `-log-format` | `MEZ_LOG_FORMAT` | `auto` |
| `venue_id` | `-venue-id` | `MEZ_VENUE_ID` | empty |
| `relay_url` | `-relay-url` | `MEZ_RELAY_URL` | empty, so no relay |
| `timezone` | `-timezone` | `MEZ_TIMEZONE` | the machine's own |
| `dev_origin` | `-dev-origin` | `MEZ_DEV_ORIGIN` | empty, so no CORS |

Notes on a few of them:

- `listen_addr` is `host:port`. Use `:7777` to accept connections from the whole
  network, or `127.0.0.1:7777` to accept only from the machine itself.
- `log_format` is `auto`, `json` or `text`. `auto` writes readable lines when you
  are watching a terminal and JSON when something is collecting the output.
- `timezone` is an IANA name such as `Asia/Kolkata`. It is used for rendering
  and for deciding which day a shift belongs to; stored times are always UTC.
  The zone database is inside the binary, so named zones work on a machine that
  has never had one installed.
- `dev_origin` is only for developing the frontends. Leave it empty.

Run `./mez -h` for the same list from the binary itself.

`config.toml` is a flat list of `key = value`. Comments start with `#`. Sections,
arrays and single quotes are not supported and are rejected with a line number
rather than ignored.

## Checking that it works

```sh
curl http://localhost:7777/api/v1/health
```

```json
{
  "status": "ok",
  "version": "v0.1.0",
  "commit": "9f1c2b3d4e5f60718293a4b5c6d7e8f90a1b2c3d",
  "build_date": "2026-09-17T21:53:14+05:30",
  "started_at": "2026-09-17T16:27:04Z",
  "uptime_seconds": 1,
  "checks": {
    "db": "unknown",
    "disk": "unknown",
    "printer": "unknown",
    "tunnel": "unknown"
  }
}
```

`status` answers one question: is this process serving. The checks are reported
separately and all read `unknown` today, because the subsystems that answer them
are built in later phases. `commit` and `build_date` identify exactly which
build is running, which is the first thing to establish on a support call.

## Testing your receipt printer

Before your first service, check the printer from the same computer. You do
not need to set anything up first, and the server does not need to be running:

```sh
./mez --test-print 192.168.1.50
```

Use the printer's own IP address; its self-test slip usually shows it (hold
the feed button while switching it on). Port 9100 is assumed; add `:9101` if
yours differs. The printer should print one slip with:

- a line of digits `1234567890...` that fills exactly one line. If it wraps or
  stops short, the paper width is wrong. `-paper 58` is for 58mm rolls; 80mm
  is the default.
- left, centre and right text, bold, underline, and large text
- a barcode and a QR code. Scan both with a phone: the barcode reads
  `NF/26-27/000001`, and the QR code reads `mez printer test OK`.
- a cut at the end

Add `-drawer` to open the cash drawer plugged into the printer as well. If
nothing prints, the command tells you it could not connect: check that the
printer is on, on the same network as this computer, and at that address.

`-save test.bin` writes the page to a file instead, so you can send it to a
printer shared by Windows as raw bytes (`copy /b test.bin \\THIS-PC\Printer`).

## Stopping it

Ctrl+C, or `SIGTERM` from a service manager. Either way it stops accepting new
requests, finishes the ones already running, and closes everything in the
reverse of the order it opened. It gives itself ten seconds for that and then
exits regardless.

## When it will not start

**`bind: address already in use`, or `Only one usage of each socket address`.**
Something already holds port 7777, usually an older copy of mez still
running. Stop it, or start this one on another port with `-listen-addr :7778`.

**`creating data directory ...: permission denied`.** The account running the
server cannot write to `data_dir`. Either grant it access or point it somewhere
it can write, with `-data-dir`.

**`unknown key "..."` on startup.** A typo in `config.toml`. The message lists
the keys the server knows. It refuses to start rather than ignore the line,
because a setting that is silently dropped is a change you believe you made.

**`timezone "...": unknown timezone`.** Use an IANA name such as
`Asia/Kolkata` or `Europe/Lisbon`, not an abbreviation like `IST`.

**The page says "placeholder bundle".** The binary was built without the
frontends. Rebuild with `-tags frontend` once `web/*/dist` exists, from Phase 05
onwards.

## What is not here yet

At the end of Phase 01 this server has no database, no menu, no orders and no
printing. It starts, serves, logs, answers health and shuts down cleanly. The
rest arrives phase by phase; `docs/STATE.md` says where the build has got to.
