# mez

An open-source, self-hostable point of sale for cafes, bars and small
restaurants. One binary on the venue's own computer. It keeps selling with no
internet connection.

**One ticket, wherever it starts.** Counter, table or kitchen: the same ticket,
the same kitchen display, the same invoice series, the same night's Z-report.

> **Status: alpha.** The till, kitchen display, payments, invoices, shifts,
> Z-reports, the back office, guest QR ordering, the Windows service and the
> Windows installer are built and tested, and so is the setup wizard a new venue
> opens on, and backups to a folder or a bucket, set from the back office. Not yet recommended for a venue's only
> till. [`docs/STATE.md`](docs/STATE.md) says exactly where the build is.

## What it does

| Area | Built |
|---|---|
| **Register** | Categories and items, modifiers, quantities, notes, discounts with manager approval, open tickets, hold and resume |
| **Tables** | Floor plan, seat, move, join and part tables, live state across every till |
| **Kitchen** | Kitchen display per station, courses and firing, recall, ESC/POS kitchen tickets |
| **Payment** | Cash, card and UPI, split bills, tips, voids and refunds, credit notes, gapless invoice numbering per financial year |
| **Tax** | GST (India, CGST/SGST), UK VAT, US sales tax, inclusive or exclusive pricing, HSN codes |
| **Printing** | ESC/POS receipts and tickets over TCP:9100, a durable print queue, cash drawer kick |
| **Shifts** | Open and close, blind drawer count, cash in and out, Z-report, tip pooling, Tally and CSV exports |
| **Back office** | Menu editor with photos and sold-out, recipes and stock, staff, roles and permissions, sales reports with heatmap, audit trail, every venue setting, device pairing |

Everything runs on the venue's machine. Tills, tablets and kitchen screens are
browsers on the same network. There is no cloud account, no subscription and
no database server to install.

## Quick start

You need a binary. Until releases are published, build one (it takes a
minute, see [Building from source](#building-from-source)):

```sh
make dist        # binaries for Windows, Linux and macOS in dist/
```

Then, on the machine that will run the venue:

```sh
./mez
```

Start `mez` in a terminal. With no venue yet it prints a **setup link**,
`http://localhost:7777/setup#token=...`: open it in a browser **on that
machine** (`mez setup-link` prints it again). The link carries a token kept
in the data directory, so that only someone who may read that directory can
make the venue. It opens the setup wizard: the venue's
name, currency and time zone; tax and invoice numbering; your name and a PIN;
a receipt printer and a test page; and at the end the address and a pairing
code for a second device. It ends with you signed in at the till. The wizard
only works from the machine itself, at `localhost`: from any other device it
says where to go.

The back office is at `http://localhost:7777/backoffice` in the same browser,
on the same pairing; its **Devices** screen pairs every further till and
kitchen display.

**To try it with a demo cafe**, or on a server with no browser of its own
(Docker, a headless box), set the venue up from a terminal instead, before
starting it:

```sh
./mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
            -device-kind back_office -device-name "Office" -demo-menu
```

`setup` prints the owner's PIN (once, write it down) and a pairing code that
works for ten minutes. `-demo-menu` adds a demo menu, tables and staff with
PINs it prints: never use it for a real venue. `./mez setup -h` lists the
tax, currency and financial year options, and `./mez pair -device-kind
kitchen` makes another pairing code at the machine.

Check a receipt printer before service, with no setup needed:

```sh
./mez --test-print 192.168.1.50
```

## The Windows installer, and the warning Windows shows

`make dist && make installer` builds `mez_<version>_windows_amd64_setup.exe`
(it needs [Inno Setup 6](https://jrsoftware.org/isinfo.php)). Run it on the
venue's PC: it copies mez to `C:\Program Files\mez`, sets it to start with
Windows as a service under its own account, lets tablets on the same network
through the firewall, puts a full-screen **mez** shortcut on the desktop and
starts it. Removing it (Settings, Apps) asks whether to keep the venue's
data, and keeps it unless you say otherwise twice.

**The installer is not signed, and Windows will warn you.** When you run it,
a blue box says "Windows protected your PC" and "Microsoft Defender
SmartScreen prevented an unrecognized app from starting", with only a
**Don't run** button in sight. Click **More info**, then **Run anyway**.
Your browser may also say the download "isn't commonly downloaded"; choose
to keep it.

What that warning means: Windows has not seen this file often, and it does
not carry a publisher's certificate. It does not mean Windows found anything
in it. A certificate that removes the warning is bought yearly from a
certificate authority, and this project does not have one. You are being
asked to trust a file from the internet, which is a fair thing to be asked
twice about, so:

- Download it only from this repository's releases page.
- Or build it yourself from the source, which is what the commands above do.

If a certificate is ever in use, the installer and its uninstaller are
signed by the same `make installer` (`MEZ_SIGNTOOL`), and this section will
say so.

## Ways to run it

| How | Status | Guide |
|---|---|---|
| **Windows**, the `.exe` on the counter PC | Works today. Installer and Windows service arrive in phase 12 | [Windows](docs/deploy.md#windows) |
| **Linux**, bare metal with systemd | Works today | [Linux](docs/deploy.md#linux) |
| **Docker**, with a volume for the data | Works today: `build/Dockerfile` and `build/docker-compose.yml` | [Docker](docs/deploy.md#docker) |
| **On a domain** with HTTPS, behind Caddy or nginx | Works today, with trade-offs to read first | [On a domain](docs/deploy.md#on-a-domain-with-https) |

[`docs/deploy.md`](docs/deploy.md) covers all four, plus backups and
upgrades. [`docs/running.md`](docs/running.md) covers every setting, where the
data lives, and what to do when it will not start.

## Building from source

You need Go 1.25 or newer, Node 22.12 or newer, and a shell with `sh` and
`make` (on Windows, Git Bash with GNU Make, or WSL).

```sh
git clone https://github.com/byjoellobo/mez.git
cd mez
make build-web   # format check, tests and production builds of the three apps
make check       # gofmt, go vet, Go tests, TypeScript typecheck, build bin/mez
make dist        # release binaries with the apps embedded, in dist/
```

`make help` lists every target. No cgo, so one machine cross-compiles for all
three platforms, and two builds of the same commit are byte-identical.

## How it is built

```
  tills, tablets, kitchen screens (browsers on the LAN)
                 |  HTTP + WebSocket
  +--------------v--------------------------------------+
  |  mez  (one Go binary)                               |
  |  REST API  .  live event hub  .  print queue        |
  |  three embedded web apps: terminal, back office,    |
  |  guest                                              |
  |  SQLite in WAL mode: the single source of truth     |
  +--------------+--------------------------------------+
                 |  TCP 9100
          receipt and kitchen printers, cash drawer
```

Go with the standard library's HTTP server, SQLite through `modernc.org/sqlite`
(pure Go), SQL written by hand and compiled with sqlc, React and TypeScript
with Vite and Tailwind. Orders are an append-only event log; what a screen
shows is a projection of it. Money is always integer minor units.
[`docs/01-architecture.md`](docs/01-architecture.md) has the full picture.

## Contributing

Contributions are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md): how
to set up, the rules that are never broken, how to test, and how changes are
proposed.

## Documentation

| File | What it covers |
|---|---|
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Setting up, the workflow, the rules, sending a change |
| [`docs/deploy.md`](docs/deploy.md) | Windows, Linux, Docker, a domain with HTTPS, backups, upgrades |
| [`docs/releasing.md`](docs/releasing.md) | Making a release, for whoever does |
| [`docs/running.md`](docs/running.md) | Settings, data directory, logs, printer test, troubleshooting |
| [`docs/00-product.md`](docs/00-product.md) | Scope and non-goals |
| [`docs/01-architecture.md`](docs/01-architecture.md) | The system and why it is shaped this way |
| [`docs/02-domain.md`](docs/02-domain.md) | Money, orders, invoices and tax |
| [`docs/03-conventions.md`](docs/03-conventions.md) | How code is written here |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | Every non-obvious decision, as numbered ADRs |
| [`docs/STATE.md`](docs/STATE.md) | Where the build is, and what is known to be broken |
| [`docs/phases/`](docs/phases/) | The build plan, phase by phase |
| [`docs/design/`](docs/design/) | Design tokens and screen specs |
| [`web/README.md`](web/README.md) | The three frontends and their dev servers |
| [`CLAUDE.md`](CLAUDE.md) | How an AI agent session works on this repository |

## Licence

The server, terminal and back office are licensed under **AGPL-3.0**. See
[LICENSE](LICENSE). If you run a modified mez for others over a network, you
must offer them its source.

The guest ordering app (`web/guest/`) is intended to be **MIT**, since it is
embedded in other people's pages. It will carry its own `LICENSE` file when it
ships in phase 11; until then it is covered by the AGPL like the rest.
