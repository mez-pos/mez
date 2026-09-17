# Running Nofiltr POS

Nofiltr POS is one binary. It needs no database server, no Docker and no
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
| `nofiltr_<version>_windows_amd64.exe` | Windows 10 and 11, 64-bit |
| `nofiltr_<version>_linux_amd64` | Linux, 64-bit |
| `nofiltr_<version>_darwin_arm64` | macOS, Apple silicon |

Building needs Go 1.23 or newer and a shell with `sh` on the path. On Windows
that means Git Bash or WSL, not `cmd.exe` or PowerShell.

Two builds of the same commit produce byte-identical binaries, so you can check
that what you are about to run matches the source it claims to come from.

## Starting it

```sh
./nofiltr
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
| Windows | `C:\ProgramData\Nofiltr POS` |
| Linux and macOS | `./data`, beside wherever you started it from |

Back up that directory and you have backed up the venue.

Logs are in `logs/` inside it, as `nofiltr.log`. The current file is rotated to
`nofiltr.log.1` once it reaches 8 MB, and five rotations are kept, so the logs
never grow past about 48 MB.

## Changing the port, and everything else

There are four places a setting can come from. Each one beats the ones above it:

1. The built-in defaults
2. `config.toml`, beside the binary
3. Environment variables
4. Command-line flags

So, to move the server to port 8080, any of these will do:

```sh
./nofiltr -listen-addr :8080
```

```sh
NOFILTR_LISTEN_ADDR=:8080 ./nofiltr
```

```toml
# config.toml, next to the binary
listen_addr = ":8080"
```

The full set of settings:

| Setting | Flag | Environment | Default |
|---|---|---|---|
| `listen_addr` | `-listen-addr` | `NOFILTR_LISTEN_ADDR` | `:7777` |
| `data_dir` | `-data-dir` | `NOFILTR_DATA_DIR` | platform, see above |
| `log_level` | `-log-level` | `NOFILTR_LOG_LEVEL` | `info` |
| `log_format` | `-log-format` | `NOFILTR_LOG_FORMAT` | `auto` |
| `venue_id` | `-venue-id` | `NOFILTR_VENUE_ID` | empty |
| `relay_url` | `-relay-url` | `NOFILTR_RELAY_URL` | empty, so no relay |
| `timezone` | `-timezone` | `NOFILTR_TIMEZONE` | the machine's own |
| `dev_origin` | `-dev-origin` | `NOFILTR_DEV_ORIGIN` | empty, so no CORS |

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

Run `./nofiltr -h` for the same list from the binary itself.

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

## Stopping it

Ctrl+C, or `SIGTERM` from a service manager. Either way it stops accepting new
requests, finishes the ones already running, and closes everything in the
reverse of the order it opened. It gives itself ten seconds for that and then
exits regardless.

## When it will not start

**`bind: address already in use`, or `Only one usage of each socket address`.**
Something already holds port 7777, usually an older copy of Nofiltr still
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
