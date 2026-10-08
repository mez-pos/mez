# Running mez

mez is one binary. It needs no database server, no Docker and no
internet connection. Put it on the machine behind the counter, start it, and it
serves the register on port 7777.

This page covers starting it, where it keeps your data, every setting, and
what to do when it will not start. For installing it on Windows, Linux, Docker
or a domain with HTTPS, and for backups and upgrades, see
[`deploy.md`](deploy.md).

## Getting a binary

Download the one for your platform, or build it:

```sh
make dist
```

That produces four binaries in `dist/`, none of which need anything installed
alongside them:

| File | Runs on |
|---|---|
| `mez_<version>_windows_amd64.exe` | Windows 10 and 11, 64-bit |
| `mez_<version>_linux_amd64` | Linux, 64-bit |
| `mez_<version>_linux_arm64` | A Raspberry Pi 4 or 5 |
| `mez_<version>_darwin_arm64` | macOS, Apple silicon |

On Windows the installer is the easier way in, and there is a Docker image:
[`deploy.md`](deploy.md#getting-a-binary) lists everything a release holds.

Building needs Go 1.25 or newer, Node 22.12 or newer, and `make` with `sh` on
the path. On Windows that means Git Bash or WSL, not `cmd.exe` or PowerShell.

Two builds of the same commit produce byte-identical binaries, so you can check
that what you are about to run matches the source it claims to come from.

## Setting up, once

**In a browser.** Start `mez` in a terminal. With no venue yet it prints a **setup link**,
`http://localhost:7777/setup#token=...`: open it in a browser **on that
machine** (`mez setup-link` prints it again). The link carries a token kept
in the data directory, so that only someone who may read that directory can
make the venue. It opens the setup wizard: the venue's
name, currency and time zone; tax and invoice numbering; your name and a PIN;
a receipt printer and a test page; and at the end the address and a pairing
code for a second device. It ends with you signed in at the till. The wizard
only works from the machine itself, at `localhost`: from any other device it
says where to go.
Reload the page before pressing **Create the venue** and what you typed is
lost; nothing has been made until then.

**From a terminal**, for a demo cafe, a script, or a server with no browser
of its own (Docker, a headless box, one behind a proxy). Before the first
run, create the venue, its owner and the code that pairs the first device.
Do this on the machine itself: being sat at it is what the command trusts,
because nothing else exists yet to authorise it.

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

Write the PIN down: it is shown here and never again. To choose one instead,
set `MEZ_OWNER_PIN` in the environment for the setup command; it is then not
printed at all. The `-owner-pin` flag does the same, but a PIN on a command
line stays in the shell's history, so prefer the environment.

A venue that charges tax says so here, since an invoice cannot be issued
without it:

```sh
./mez setup -name "Filter Coffee House" -owner Priya   -tax-profile gst_in -tax-registration 29ABCDE1234F1Z5
```

`./mez setup -h` lists the rest: currency, financial year, the legal name
for invoices, and what the first device is called. Running it a second time on
the same database refuses: one machine serves one venue.

Pair the back office first, with `-device-kind back_office`: the back office's
**Devices** screen is where every other device's pairing code is made. A
first code for a till still works for selling, but leaves no screen to pair a
second device from. If that is what you did, `mez pair` makes another code at
the machine, with no sign-in:

```sh
./mez pair -device-kind back_office -device-name Office
```

`-device-kind` is `terminal`, `kitchen` or `back_office`, and the name is
optional. Run it as often as needed; each code lasts ten minutes and pairs one
device. It takes `-data-dir` as the server does, and works while the server runs.

A PIN that keeps failing locks the person until someone who manages staff
clears it on the **Staff** screen (ADR-260). If every such person is locked,
`mez unlock` clears one at the machine, with no sign-in, by the id the Staff
screen shows:

```sh
./mez unlock -staff stf_A1B2C3D4
```

A forgotten PIN is reset on the **Staff** screen by someone who holds more
than the person does: a manager resets a waiter's, an owner a manager's, and
only another owner an owner's. A venue with one owner who has forgotten
theirs has nobody to ask, so `mez reset-pin` gives a new one at the machine
(ADR-304). Run it with nothing to see who there is and their ids, then with
the id:

```sh
./mez reset-pin
./mez reset-pin -staff stf_A1B2C3D4
```

It prints a new six-digit PIN, once. The person is signed out everywhere and
any lock on them is cleared; they sign in with it and change it under Staff.
The PIN is drawn, not typed on the command line, so it is not left in the
shell's history. Anyone who can run commands as the account that owns the
data directory can do this, which is the same trust `mez pair` and
`mez unlock` rest on: keep that machine and that account to people who may.

`-demo-menu` makes the venue a demo cafe for trying it out: a menu, twelve
tables in two areas, and a person for each role (a manager, a cashier, a
waiter and a kitchen cook) with PINs that `setup` prints. Those PINs are public, so never
use `-demo-menu` on a venue that sells.
[`deploy.md`](deploy.md#first-run-on-any-platform) lists every flag.

## Starting it

```sh
./mez
```

It prints a line per event to the terminal and stops on Ctrl+C, finishing any
request that is already in flight before it exits.

Then open `http://localhost:7777` on the same machine, or
`http://<the machine's address>:7777` from a tablet on the same network.

Five addresses are served:

| Address | What it is |
|---|---|
| `/` | The register (the till), for staff |
| `/kitchen` | The kitchen display, for a screen paired as a kitchen device |
| `/backoffice` | Menu, staff, reports and settings, for the owner |
| `/order` | The guest ordering page, opened from a QR code |
| `/setup` | The setup wizard, once, from the machine itself only |

A pairing code is made for one kind of device and works only on that
address: a kitchen code typed into the till is refused and says so.

A binary built without the apps (no `-tags frontend`, as `go build` alone
gives) serves a placeholder page instead, and says so at startup on the line
reading `serving placeholder frontends`. `make dist` always includes them.

## A day at the till

What a service looks like once the venue is set up. Every step below was
walked against the demo venue on 2026-10-07.

1. **Open the shift.** The till asks for it before the first order: count
   the float by denomination and press **Open shift & start selling**.
   Anyone who may take payment can open it.
2. **Take orders.** Tap a dish to add it; tap the dish's card for its
   choices, course and a kitchen note. **Takeaway** is the default;
   **Dine-in** comes from the **Tables** screen (**Seat and order**), where
   the table then shows as Seated, Ordered, Paying or To clear on every
   till. **Send to Kitchen** puts the lines on the kitchen display (a
   station with a printer gets a ticket too); **Save Order** holds the
   ticket to come back to.
3. **Take payment.** **Pay** opens the tender screen: cash with quick
   amounts and the change to return, card, or UPI (**Collect by UPI**, then
   **Mark as paid** once the guest's app says so), a tip apart from the
   bill, and **Split Bill** by item or by amount. The invoice is numbered
   as the payment lands and the receipt goes to the print queue.
4. **Guests' own orders.** With a table code scanned, a phone's first order
   shows at the till as a **table request** in the top bar: **Accept and
   send** puts it on the kitchen display and the phone sees it move from
   "with the kitchen" to "ready".
5. **Watch the print queue.** **Not printed N** in the top bar is what is
   waiting for a printer; open it to retry or to see why.
6. **Close the day.** Someone with `shift.manage` (the owner or a manager;
   not a cashier) opens their name, top right, then **Close the shift**:
   count the drawer, see it against what was expected, and **Close the
   day**. The Z-report is issued and printed, and is on the back office's
   **Z-reports** screen with the day's exports.

## Where your data lives

Everything the server owns (the database, the menu's photos and the logs)
sits in one directory:

| Platform | Default |
|---|---|
| Windows | `C:\ProgramData\mez` |
| Linux and macOS | `./data`, in the directory you ran it from |
| Linux, under the systemd unit | `/var/lib/mez`, which the unit sets |
| Docker | `/data` in the container, the volume `mez-data` |

Every command that reads the venue (`setup`, `pair`, `unlock`, `reset-pin`, `restore`,
`setup-link`, `table-codes`) looks in the same place, and takes `-data-dir`
when the server was given one.

The server keeps that directory to itself: on Windows only administrators,
the service's own account and the account that ran setup can open it, and
elsewhere it is mode 0700.

Inside it: `mez.db` (with `mez.db-wal` and `mez.db-shm` while it runs),
`photos/` for menu photos, and `logs/`. With a backup turned on there is
also `backup.json`, which says where it goes and may hold a bucket's
secret, and Litestream's working files, whose names begin
`.mez.db-litestream`; leave both alone. [`deploy.md`](deploy.md#backups)
says how to turn the backup on, what it covers, and how to get a venue
back with `mez restore`.

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
| `relay_key` | `-relay-key` | `MEZ_RELAY_KEY` | empty; a secret, needed when `relay_url` is set |
| `remote_backoffice` | `-remote-backoffice` | `MEZ_REMOTE_BACKOFFICE` | `false`. `true` lets the back office be opened through the relay, away from the venue (ADR-306). Only the two words are taken. It needs `relay_url`, and a `relay_key` the relay's operator issued for it (`relay issue-key VENUE --remote`): with either missing mez does not start, and says which. **Leave it off at a real venue for now**: the rules that hold a request from outside to the owner's work, and away from payments and the till, are phase 13's next task |
| `timezone` | `-timezone` | `MEZ_TIMEZONE` | the machine's own for the server; `setup` requires an IANA name |
| `dev_origin` | `-dev-origin` | `MEZ_DEV_ORIGIN` | empty, so no CORS |
| `gateway` | `-gateway` | `MEZ_GATEWAY` | empty, so no gateway; `razorpay` or `stripe` |
| `gateway_key_id` | `-gateway-key-id` | `MEZ_GATEWAY_KEY_ID` | empty; needed for `razorpay`. Stripe has none |
| `gateway_webhook_secret` | `-gateway-webhook-secret` | `MEZ_GATEWAY_WEBHOOK_SECRET` | empty, so no webhook is believed; a secret. Set it where the webhook is made at the gateway: for Razorpay, `<public_url>/order/api/hook/razorpay` and the payment link events; for Stripe, `<public_url>/order/api/hook/stripe`, the `checkout.session.*` events, and the endpoint's signing secret (`whsec_`) |
| `public_url` | `-public-url` | `MEZ_PUBLIC_URL` | empty; the address guests reach the venue at, `https://cafe.example`. A gateway sends a guest back to it after paying. **Needed for `stripe`**, which makes no payment page without it |
| `gateway_key_secret` | `-gateway-key-secret` | `MEZ_GATEWAY_KEY_SECRET` | empty; a secret, needed when `gateway` is set: Razorpay's key secret, or Stripe's secret key (`sk_`). Prefer the environment: a flag is visible to anyone who can list this machine's processes |
| | `-config` | | `config.toml` beside the binary |

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
    "db": "ok",
    "disk": "unknown",
    "printer": "unknown",
    "tunnel": "unknown"
  }
}
```

`status` answers one question: is this process serving. The checks are reported
separately: `db` is `ok` when the database answers and `failing` when it does
not. `tunnel` is `unknown` on a venue with no relay set, `ok` while the
venue is connected to its relay, and `failing` while it is set and not
connected. `disk` and `printer` always read `unknown`: nothing answers them
yet, so look at the **Printers** screen for a printer and at the disk
itself. `commit` and `build_date` identify exactly which
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

[`hardware.md`](hardware.md) says what a printer and a cash drawer need to
be, how each is connected, and what mez cannot use.

## Stopping it

Ctrl+C, or `SIGTERM` from a service manager. Either way it stops accepting new
requests, finishes the ones already running, and closes everything in the
reverse of the order it opened. It gives itself ten seconds for that and then
exits regardless.

## When it will not start

This is for a server that does not come up. For a printer that is not
found, a tablet that cannot reach the server, Windows's warning about the
installer, guests who cannot order from their phones, and a cash drawer
that will not open, see [`troubleshooting.md`](troubleshooting.md).

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
frontends. Build them (`make build-web`) and rebuild with `make build`, or use
`make dist`, which always includes them.

## Every command

`mez` alone starts the server. The rest are run at the machine itself, which
is what they trust; each takes `-h`.

| Command | What it does |
|---|---|
| `mez setup` | Makes the venue, its owner and a first pairing code, once ([above](#setting-up-once)) |
| `mez setup-link` | Prints the address of the setup wizard, with its token, while there is no venue; with one, the till's address |
| `mez pair` | Makes a pairing code for one more device |
| `mez unlock -staff stf_...` | Clears the lock on a person whose PIN kept failing |
| `mez reset-pin` | Lists who works at the venue with their ids; with `-staff stf_...`, gives that person a new PIN, shown once, and clears their lock |
| `mez table-codes` | Lists every table with the code a guest's phone scans, making any that are missing. The back office's Floor plan prints them |
| `mez restore` | Brings a venue back from its backup into an empty data directory ([`deploy.md`](deploy.md#getting-a-venue-back)) |
| `mez service install`, `uninstall`, `start`, `stop`, `status` | The Windows service ([`deploy.md`](deploy.md#windows)) |
| `mez --test-print ADDRESS` | Prints a test page, with no venue needed ([above](#testing-your-receipt-printer)) |
| `mez -h` | Says which build this is, on its first line, and lists the server's flags |

## What is known to be missing

[`KNOWN.md`](KNOWN.md) lists, screen by screen, what is broken, left out or
has not been seen working, and [`STATE.md`](STATE.md) says where the build
has got to.
