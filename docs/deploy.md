# Deploying mez

mez is one binary and one data directory. Every way of running it below is
the same binary; they differ only in what starts it and where the data lives.

- [Choose where it runs](#choose-where-it-runs)
- [Getting a binary](#getting-a-binary)
- [First run, on any platform](#first-run-on-any-platform)
- [Windows](#windows)
- [Linux](#linux)
- [Docker](#docker)
- [On a domain with HTTPS](#on-a-domain-with-https)
- [Backups](#backups)
- [Upgrading](#upgrading)
- [What phase 12 adds](#what-phase-12-adds)

For every setting, the log files and troubleshooting, see
[`running.md`](running.md).

## Choose where it runs

**Put it in the building.** The machine running mez is the one source of
truth, and every till, tablet and kitchen screen talks to it over the local
network. That is what lets a venue keep selling when its internet is down,
and it is the only way the server can reach printers, which sit on the same
network at TCP port 9100.

| Where | Sells offline | Reaches printers | Use it for |
|---|---|---|---|
| A PC or mini PC at the venue (Windows or Linux) | Yes | Yes | Running a venue. The recommended setup |
| Docker on a machine at the venue | Yes | Yes | A venue that already runs a home server or NAS |
| A VPS or cloud host, on a domain | **No** | **No**, without a VPN to the venue | Demos, training, trying it out, a back office reached from home |

Reaching guests from the internet while the server stays in the building is
what the relay in phase 11 is for. Until then, a cloud-hosted mez is a demo,
not a till.

## Getting a binary

Build all three platforms from a clone (Go 1.23+, Node 22.12+, and `make` with
a POSIX shell; on Windows, Git Bash or WSL):

```sh
make dist
```

| File in `dist/` | Runs on |
|---|---|
| `mez_<version>_windows_amd64.exe` | Windows 10 and 11, 64-bit |
| `mez_<version>_linux_amd64` | Linux, 64-bit |
| `mez_<version>_darwin_arm64` | macOS, Apple silicon |

Each one is self-contained: the three web apps and the timezone database are
inside it, and nothing needs installing beside it.

No `make`? The same build by hand:

```sh
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -tags frontend -o mez.exe ./cmd/mez
```

Change `GOOS` and `GOARCH` (`linux`/`amd64`, `darwin`/`arm64`, `linux`/`arm64`
for a Raspberry Pi 4 or 5) for other machines. Leaving out `-tags frontend`
builds a server that serves placeholder pages instead of the apps.

## First run, on any platform

**Where the server has a browser of its own** (a Windows till, a desktop):
start it and open the **setup link** it prints,
`http://localhost:7777/setup#token=...`, on that machine (the Windows
installer opens it for you; `mez setup-link` prints it again). It opens the
setup wizard, which asks what `mez setup` takes as flags, finds
a receipt printer, and ends with you signed in and a pairing code for a
second device on screen. It works only from the machine itself, at
`localhost` (ADR-266): not from another device, not through a proxy, and not
into a container, where a published port does not arrive from loopback. And
it needs the token in that link (ADR-270): the plain address shows where to
get it, and asks nothing.

**Everywhere else** (Docker, a headless Linux box, a server behind a proxy),
and for a demo cafe: setup runs once, from a terminal on the machine itself,
before the server starts. Pair the **back office** first: it is where every
other device's pairing code is made.

```sh
mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
          -device-kind back_office -device-name "Office"
```

A venue that charges tax says so now, because an invoice cannot be issued
without it:

```sh
mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
          -tax-profile gst_in -tax-registration 29ABCDE1234F1Z5 \
          -device-kind back_office -device-name "Office"
```

| Flag | Default | Meaning |
|---|---|---|
| `-name` | required | The venue's name, as staff and guests see it |
| `-legal-name` | the name | The registered name printed on invoices |
| `-owner` | `Owner` | The owner's name |
| `-owner-pin` | `MEZ_OWNER_PIN`, else drawn and printed | The owner's PIN, 6 to 8 digits; prefer the environment variable, which the shell does not remember |
| `-timezone` | required | An IANA name such as `Asia/Kolkata` or `Europe/London`; the machine's own zone is refused |
| `-currency` | `INR` | ISO 4217: `INR`, `GBP`, `USD`, ... It cannot be changed later |
| `-fy-start-month` | `4` | The month the financial year starts: 4 in India, 1 in most places |
| `-tax-profile` | `none` | `none`, `gst_in`, `vat_uk` or `sales_tax_us` |
| `-tax-registration` | empty | GSTIN or VAT number, required by those profiles |
| `-tax-pricing` | the profile's | `inclusive` or `exclusive` |
| `-device-kind` | `terminal` | What the first pairing code pairs: `terminal`, `kitchen` or `back_office` |
| `-device-name` | `Counter 1` | What that device is called |
| `-demo-menu` | off | Makes a demo cafe: an 18-item menu priced in rupees, twelve tables, and a manager, cashier and cook with public PINs. Never for a venue that sells |
| `-data-dir` | the platform default | Where the database goes |

It prints the owner's PIN (only once) and a pairing code that lasts ten
minutes. Running it again on the same database is refused: one data directory
is one venue.

`mez pair` makes another pairing code at the machine, any time after setup,
with `-device-kind` (`terminal`, `kitchen` or `back_office`), `-device-name`
and `-data-dir` as above. In Docker:
`docker run --rm -v mez-data:/data mez pair -device-kind back_office`.

Then start the server and finish in the browser:

1. Open `http://localhost:7777/backoffice` and enter the pairing code.
2. Sign in with the owner's PIN. Change it under **People**.
3. Under **Devices**, choose **Pair a device** for each till and kitchen
   display, and enter each code on that device at
   `http://<server address>:7777`.
4. Under **Settings**, check the venue, tax, invoice numbering, receipt and
   shift rules. Under **Printers**, add each printer and its stations.

## Windows

**With the installer.** Run `mez_<version>_windows_amd64_setup.exe` and say
yes to Windows's question. It is not signed, so Windows first shows
"Windows protected your PC": choose **More info**, then **Run anyway** (the
[README](../README.md#the-windows-installer-and-the-warning-windows-shows)
says what that warning does and does not mean). The installer:

- copies `mez.exe` to `C:\Program Files\mez`;
- registers the service (below), running as `NT SERVICE\mez`, and starts it;
- adds a firewall rule named `mez` for the program, on networks Windows
  calls Private or Domain. On a network marked **Public** tablets are kept
  out: set the venue's network to Private in Windows settings;
- puts a **mez** shortcut on the desktop and in the Start menu, which opens
  the till full screen in Edge (or Chrome). Alt+F4 leaves it.

Running a newer installer over an older one upgrades in place and keeps the
data. Removing mez (Settings, Apps, mez, Uninstall) stops and removes the
service and the firewall rule, then asks whether to delete the data in
`C:\ProgramData\mez`; it is kept unless you answer Yes twice. A
`config.toml` you put beside `mez.exe` is left where it is.

When the installer finishes, leave **Open mez now** ticked: it opens the
setup wizard, with the token that lets the venue be made. Nothing needs a
terminal. The desktop shortcut does not carry the token, so until the venue
exists it shows where to get it: run the installer again (it keeps
everything), or in a terminal opened as administrator run
`& "C:\Program Files\mez\mez.exe" setup-link` and open what it prints.

A venue can still be created from a terminal opened as administrator
instead, before the shortcut is first opened:

```powershell
& "C:\Program Files\mez\mez.exe" setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata `
        -device-kind back_office -device-name "Office"
```

The rest of this section is the same thing done by hand, with the `.exe`
alone.

**Install.** Make a folder such as `C:\mez`, put
`mez_<version>_windows_amd64.exe` in it, and rename it `mez.exe`. In
PowerShell, in that folder:

```powershell
.\mez.exe setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata `
                -device-kind back_office -device-name "Office"
.\mez.exe
```

The data lives in `C:\ProgramData\mez` (the database `mez.db`, `logs\` and
`photos\`). Windows may show a SmartScreen warning for an unsigned binary
built on another machine; choose **More info**, then **Run anyway**.

**Let tablets in.** Allow the port through the firewall once, as
administrator:

```powershell
New-NetFirewallRule -DisplayName "mez" -Direction Inbound -Protocol TCP -LocalPort 7777 -Action Allow -Profile Private
```

Set the venue's network to **Private** in Windows settings, and give the PC a
fixed address (a DHCP reservation in the router) so tablets can always find
it.

**Start it with Windows.** Register it as a Windows service. In a PowerShell
opened with **Run as administrator**, in that folder:

```powershell
.\mez.exe service install
.\mez.exe service start
```

It then starts with the computer, before anyone signs in, and Windows starts
it again when it dies: after five seconds, then thirty, then every five
minutes until it stays up. `mez service status` says whether it is
running, `mez service stop` stops it (requests in flight finish first), and
`mez service uninstall` removes it and leaves the data. Flags given to
`install` are the ones the service starts with, for example
`.\mez.exe service install -listen-addr :8080`; a `config.toml` beside the
binary is read as before.

Warnings and errors go to the Windows event log as well as to
`C:\ProgramData\mez\logs\mez.log`: in Event Viewer, **Windows Logs**,
**Application**, source **mez**. If the till does not open after a start,
look there first: a port already in use is said in words.

The service runs as its own account, `NT SERVICE\mez`, which Windows makes
for it: it has no password and no rights over the rest of the computer.
`install` gives it the data directory. It reaches a printer on the LAN and a
USB printer installed on the PC. Keep `mez.exe` in a folder every account
can read, such as `C:\mez`, and not under a user's own folders (Downloads,
Desktop): the service's account cannot open those.

A printer that only one Windows user can see (a shared printer connected
under their account) needs the service to run as them:

```powershell
$env:MEZ_SERVICE_PASSWORD = "their password"
.\mez.exe service install -user ".\cafe"
Remove-Item Env:MEZ_SERVICE_PASSWORD
```

That account must be able to write `C:\ProgramData\mez` and must hold the
Windows right **Log on as a service** (Local Security Policy, Local
Policies, User Rights Assignment); install checks neither.
`-user LocalSystem` runs it as the computer's most powerful account, which
needs neither and is not advised for a server guests' phones reach.

**A till on the same PC.** A full-screen browser window with no address bar:

```powershell
& "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --app=http://localhost:7777 --kiosk --edge-kiosk-type=fullscreen
```

Put that in a shortcut in `shell:startup` to open the till at sign-in.

## Linux

Any 64-bit distribution with systemd. A Raspberry Pi 4 or 5 is enough for a
small venue; build with `GOARCH=arm64`.

```sh
sudo useradd --system --home /var/lib/mez --create-home mez
sudo install -m 755 mez_<version>_linux_amd64 /usr/local/bin/mez
sudo -u mez /usr/local/bin/mez setup -data-dir /var/lib/mez \
     -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
     -device-kind back_office -device-name "Office"
```

`/etc/systemd/system/mez.service`:

```ini
[Unit]
Description=mez point of sale
After=network-online.target
Wants=network-online.target

[Service]
User=mez
Group=mez
Environment=MEZ_DATA_DIR=/var/lib/mez
Environment=MEZ_TIMEZONE=Asia/Kolkata
Environment=MEZ_LOG_FORMAT=json
ExecStart=/usr/local/bin/mez
Restart=on-failure
RestartSec=5
# mez finishes in-flight requests on SIGTERM and gives itself ten seconds.
TimeoutStopSec=15
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/mez

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now mez
journalctl -u mez -f
```

Open port 7777 to the local network if a firewall is on (`sudo ufw allow from
192.168.0.0/16 to any port 7777`, adjusted to your network).

## Docker

`build/Dockerfile` builds mez from source, with the three apps embedded, into
an image of about 30 MB that holds nothing but the binary. It runs as an
unprivileged user and keeps everything in `/data`. From the repository root:

```sh
docker compose -f build/docker-compose.yml build
docker compose -f build/docker-compose.yml run --rm mez setup \
       -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
       -device-kind back_office -device-name "Office"
docker compose -f build/docker-compose.yml up -d
```

Edit `MEZ_TIMEZONE` in `build/docker-compose.yml` first: a container's clock
is UTC, and the venue's day, shift dates and Z-reports follow this setting.

| Task | Command (add `-f build/docker-compose.yml`) |
|---|---|
| Logs | `docker compose logs -f mez` |
| Stop | `docker compose stop` |
| Health | `curl http://localhost:7777/api/v1/health` |
| Printer test | `docker compose run --rm mez --test-print 192.168.1.50` |

The data is in the named volume `mez-data`, and survives `down`, rebuilds
and upgrades. `docker compose down -v` deletes it, venue and all.

Without compose:

```sh
docker build -f build/Dockerfile -t mez .
docker volume create mez-data
docker run --rm -v mez-data:/data mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata -device-kind back_office
docker run -d --name mez --restart unless-stopped -p 7777:7777 \
       -e MEZ_TIMEZONE=Asia/Kolkata -v mez-data:/data mez
```

Printers are reached from inside the container over the ordinary network, so
the host must be able to reach them on port 9100. On Docker Desktop for
Windows and macOS, a printer on the LAN usually works; if it does not, run the
container on a Linux host.

## On a domain with HTTPS

Read [Choose where it runs](#choose-where-it-runs) first: a mez on a cloud
server stops selling when the venue's internet drops, and cannot reach the
venue's printers. A domain also suits a mez that *is* at the venue, reached
from home over a VPN or tunnel.

Run mez on `127.0.0.1:7777` (`-listen-addr 127.0.0.1:7777`, or the Docker
port mapping `"127.0.0.1:7777:7777"`) so that only the proxy can reach it, and
put a reverse proxy in front that terminates HTTPS. The proxy must:

- pass the `Host` header through unchanged: the live stream refuses a
  WebSocket whose origin differs from the host it was asked for;
- upgrade WebSocket connections on `/api/v1/stream`;
- not time out an idle WebSocket in under a minute (mez pings every 20 s).

**Caddy** does all three by default and fetches the certificate itself.
`/etc/caddy/Caddyfile`:

```
pos.example.com {
    reverse_proxy 127.0.0.1:7777
}
```

**nginx**, with a certificate from certbot:

```nginx
server {
    listen 443 ssl;
    server_name pos.example.com;

    ssl_certificate     /etc/letsencrypt/live/pos.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pos.example.com/privkey.pem;

    client_max_body_size 10m;   # menu photos

    location / {
        proxy_pass http://127.0.0.1:7777;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 120s;
    }
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name pos.example.com;
    return 301 https://$host$request_uri;
}
```

Point the domain's DNS `A` record at the server, then open
`https://pos.example.com/backoffice`.

**What protects it.** Everything but pairing, the health check and menu
photos needs a paired device's token (the live stream included), and pairing codes
are issued only by a signed-in manager, expire in ten minutes, and are rate
limited. Staff PINs lock after five failures in fifteen minutes. That is
designed for a venue's own network. On the open internet, also:

- always serve HTTPS, never plain HTTP;
- revoke any device you no longer use, under **Devices**;
- give staff PINs of six or more digits;
- keep the server's OS updated, and expose only ports 80 and 443.

## Backups

Everything is in the data directory: `mez.db`, `photos/` and `logs/`. Back it
up and you have backed up the venue. The database is SQLite in WAL mode, so
copy it while mez is **stopped**, or the copy may miss the latest sales.

```sh
# Linux
sudo systemctl stop mez
sudo tar czf mez-backup-$(date +%F).tgz -C /var/lib/mez .
sudo systemctl start mez

# Docker
docker compose -f build/docker-compose.yml stop
docker run --rm -v mez-data:/data -v "$PWD":/backup alpine \
       tar czf /backup/mez-backup.tgz -C /data .
docker compose -f build/docker-compose.yml start
```

On Windows, end the `mez` task, copy `C:\ProgramData\mez`, and run it again.

Do it after closing each day, and keep a copy off the machine. Continuous
backup to S3, R2 or a folder, configured from the back office, arrives with
Litestream in phase 12.

## Upgrading

1. Take a backup, as above. Migrations only go forward, and an older mez is
   not built to run on a database a newer one has migrated, so the backup is
   how you go back.
2. Stop mez, replace the binary (or `git pull` and rebuild the image), start
   it. Migrations apply by themselves on startup.
3. Check `/api/v1/health` reports the new `version` and `"db": "ok"`.

Paired devices stay paired across an upgrade.

## What phase 12 adds

[`phases/phase-12.md`](phases/phase-12.md) is the plan. In short: a Windows
installer (Inno Setup) that registers the Windows service, opens the firewall
and creates the kiosk shortcut; a setup wizard, so a new Windows venue needs no `mez setup` (built);
Litestream backups configured in the back office; a systemd unit and image
published with each release; and release automation with checksums.
