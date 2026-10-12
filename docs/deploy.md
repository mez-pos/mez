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
- [Guests ordering from their phones](#guests-ordering-from-their-phones)
- [The back office from anywhere](#the-back-office-from-anywhere)
- [Backups](#backups)
- [Upgrading](#upgrading)

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

Guests ordering from their own phones do not change this. The server stays
in the building and dials out to a relay, which is the only part on the
internet: see [Guests ordering from their phones](#guests-ordering-from-their-phones).
A cloud-hosted mez is a demo, not a till.

## Getting a binary

Each release on GitHub, under **Releases**, has these, and a
`mez_<version>_checksums.txt` to check them against:

| File | Runs on |
|---|---|
| `mez_<version>_windows_amd64_setup.exe` | Windows 10 and 11, 64-bit: the installer |
| `mez_<version>_windows_amd64.exe` | The same, without the installer |
| `mez_<version>_linux_amd64` | Linux, 64-bit, with `mez.service` |
| `mez_<version>_linux_arm64` | A Raspberry Pi 4 or 5, with `mez.service` |
| `mez_<version>_darwin_arm64` | macOS, Apple silicon |

and a Docker image, `ghcr.io/byjoellobo/mez:<version>` (the version without
its `v`, for amd64 and arm64; there is no `latest`), with
`docker-compose.yml`. There is no release yet; until there is, build them.

Check a download before running it. In PowerShell,
`Get-FileHash .\mez_<version>_windows_amd64_setup.exe -Algorithm SHA256`
and compare it with that file's line in the checksums; on Linux and macOS,
`sha256sum -c mez_<version>_checksums.txt --ignore-missing`.

From a clone (Go 1.25+, Node 22.12+, and `make` with a POSIX shell; on
Windows, Git Bash or WSL), all of them at once:

```sh
make dist
```

The installer is one more step, on Windows, with
[Inno Setup 6](https://jrsoftware.org/isinfo.php): `make installer` (pass
`ISCC=/path/to/ISCC.exe` if it is not on the path).

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
| `-timezone` | required, unless `MEZ_TIMEZONE` or `config.toml` already names one | An IANA name such as `Asia/Kolkata` or `Europe/London`; the machine's own zone is never used |
| `-currency` | `INR` | ISO 4217: `INR`, `GBP`, `USD`, ... It cannot be changed later |
| `-fy-start-month` | `4` | The month the financial year starts: 4 in India, 1 in most places |
| `-tax-profile` | `none` | `none`, `gst_in`, `vat_uk` or `sales_tax_us` |
| `-tax-registration` | empty | GSTIN or VAT number, required by those profiles |
| `-tax-pricing` | the profile's | `inclusive` or `exclusive` |
| `-device-kind` | `terminal` | What the first pairing code pairs: `terminal`, `kitchen` or `back_office` |
| `-device-name` | `Counter 1` | What that device is called |
| `-demo-menu` | off | Makes a demo cafe: an 18-item menu priced in rupees, twelve tables, and a manager, cashier, waiter and cook with public PINs. Never for a venue that sells |
| `-data-dir` | the platform default | Where the database goes |

It prints the owner's PIN (only once) and a pairing code that lasts ten
minutes. Running it again on the same database is refused: one data directory
is one venue.

`mez pair` makes another pairing code at the machine, any time after setup,
with `-device-kind` (`terminal`, `kitchen` or `back_office`), `-device-name`
(default Counter, Kitchen or Back office, by kind) and `-data-dir` as above. In Docker:
`docker run --rm -v mez-data:/data mez pair -device-kind back_office`.

Then start the server and finish in the browser:

1. Open `http://localhost:7777/backoffice` and enter the pairing code.
2. Sign in with the owner's PIN. Change it under **Staff** (**New PIN**
   on your own row), and add each person who works there with **Add a
   person**: a name, a role and a PIN.
3. Under **Devices**, choose **Pair a device** for each till, kitchen
   display and extra back office, and enter each code on that device. A
   code is for one kind of device: a till opens
   `http://<server address>:7777`, a kitchen display
   `http://<server address>:7777/kitchen`, a back office
   `http://<server address>:7777/backoffice`.
4. Under **Floor plan**, make your areas and tables (**Add an area**,
   then **Add a table**); a counter-only cafe can skip this.
5. Under **Settings**, check the venue, tax, invoice numbering, receipt and
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
  the till in a window of its own in Edge (or Chrome), maximised, with no
  address bar. Ticking **Open mez full screen** in the installer makes it a
  kiosk instead: no title bar and no window buttons, for a computer that
  is only a till. Alt+F4 leaves that.

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

A printer shared from another computer cannot be used at all
([`hardware.md`](hardware.md#two-ways-to-connect-one)). If a printer
installed on this PC is one the service's account cannot open, the service
can run as a Windows user who can:

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

**A till on the same PC.** A browser window of its own, with no address bar:

```powershell
& "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --app=http://localhost:7777 --start-maximized
```

Put that in a shortcut in `shell:startup` to open the till at sign-in. For a
computer that is only a till, `--kiosk --edge-kiosk-type=fullscreen` in place
of `--start-maximized` fills the screen and takes the title bar away: there
are then no window buttons, and Alt+F4 is how it is closed.

## Linux

Any 64-bit distribution with systemd. A Raspberry Pi 4 or 5 is enough for a
small venue, with `mez_<version>_linux_arm64`. It is the same program as on
Windows: one file, no database server, nothing else to install.

```sh
sudo useradd --system --home /var/lib/mez --create-home mez
sudo install -m 755 mez_<version>_linux_amd64 /usr/local/bin/mez
sudo install -m 644 build/mez.service /etc/systemd/system/mez.service
```

[`build/mez.service`](../build/mez.service) is the unit; a release has it
beside the binaries as `mez.service`. Before starting it,
open it and set `MEZ_TIMEZONE` to the venue's own: a server's clock is often
UTC, and the venue's day, its shift dates and its Z-reports follow this
setting. Then make the venue and start it:

```sh
sudo -u mez /usr/local/bin/mez setup -data-dir /var/lib/mez \
     -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata \
     -device-kind back_office -device-name "Office"
sudo systemctl daemon-reload
sudo systemctl enable --now mez
journalctl -u mez -f
```

The unit runs mez as its own user, keeps everything in `/var/lib/mez`
(closed to everyone else), starts it again if it falls over, and lets it
write nowhere else on the machine. Other settings (a relay, a payment
gateway) go one to a line as `MEZ_NAME=value` in `/etc/mez/mez.env`, which
the unit reads if it is there, so that an upgrade can replace the unit
without losing them.

Open port 7777 to the local network if a firewall is on (`sudo ufw allow from
192.168.0.0/16 to any port 7777`, adjusted to your network).

`mez pair`, `mez unlock` and the other commands run at the machine take the
same `-data-dir /var/lib/mez`, as the `mez` user: `sudo -u mez
/usr/local/bin/mez pair -data-dir /var/lib/mez -device-kind terminal`.
Without it they look in `./data`, find no database there, and say so.

Three things differ from Windows:

- **A backup to a folder needs the folder named in the unit.** mez may
  write only to its own directory. Make the folder, give it to the `mez`
  user, and allow it:

  ```sh
  sudo mkdir -p /mnt/backup/mez && sudo chown mez:mez /mnt/backup/mez
  sudo systemctl edit mez
  ```

  and in the editor that opens:

  ```ini
  [Service]
  ReadWritePaths=/mnt/backup/mez
  ```

  then `sudo systemctl restart mez`, and set the folder under **Settings**,
  **Backup**. Without it the Backup section says "Not backing up" and the
  reason says the file system is read-only. A backup to a bucket needs none
  of this.
- **A changed backup destination takes effect at the next restart.** The
  first time a backup is turned on it starts at once. Changing it to
  somewhere else, or turning it off, is saved and waits for
  `sudo systemctl restart mez`; the Backup section says so until then.
- **Run `mez restore` as the `mez` user**, with the service stopped, so
  that what it writes belongs to the service:

  ```sh
  sudo systemctl stop mez
  sudo -u mez /usr/local/bin/mez restore -from /mnt/backup/mez -data-dir /var/lib/mez
  sudo systemctl start mez
  ```

  [Backups](#getting-a-venue-back) has the rest.

This was run on 2026-10-07 under systemd 252 on Debian 12: the unit starts
mez, starts it again after it is killed, and stops it in well under a
second; a backup to a folder is refused without `ReadWritePaths` and works
with it; and a venue was restored from that folder by the commands above.

## Docker

`build/Dockerfile` builds mez from source, with the three apps embedded, into
an image of about 50 MB that holds nothing but the binary. It runs as an
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

Make the venue with `setup` as above, not with the wizard in the browser:
the wizard only answers a browser on the server's own machine, and from
inside a container every browser is somewhere else.

| Task | Command (add `-f build/docker-compose.yml`) |
|---|---|
| Logs | `docker compose logs -f mez` |
| Stop | `docker compose stop` |
| Health | `curl http://localhost:7777/api/v1/health` |
| Printer test | `docker compose run --rm mez --test-print 192.168.1.50` |
| Another pairing code | `docker compose run --rm mez pair -device-kind terminal` |

The data is in the named volume `mez-data`, and survives `down`, rebuilds
and upgrades. `docker compose down -v` deletes it, venue and all.

**With the image from a release**, nothing is built and no clone is
needed. The `docker-compose.yml` beside a release is this same file, and it
builds from source: to use it without the source, delete its `build:` block
(three lines) and change `image: mez:local` to
`image: ghcr.io/byjoellobo/mez:<version>`. Or without compose:

```sh
docker volume create mez-data
docker run --rm -v mez-data:/data ghcr.io/byjoellobo/mez:<version> setup \
       -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata -device-kind back_office
docker run -d --name mez --restart unless-stopped --stop-timeout 20 -p 7777:7777 \
       -e MEZ_TIMEZONE=Asia/Kolkata -v mez-data:/data ghcr.io/byjoellobo/mez:<version>
```

No release has been made, so no image has been pulled from the registry:
this is the image the release workflow builds, run here from a local build.

Without compose, from source:

```sh
docker build -f build/Dockerfile -t mez .
docker volume create mez-data
docker run --rm -v mez-data:/data mez setup -name "Filter Coffee House" -owner Priya -timezone Asia/Kolkata -device-kind back_office
docker run -d --name mez --restart unless-stopped --stop-timeout 20 -p 7777:7777 \
       -e MEZ_TIMEZONE=Asia/Kolkata -v mez-data:/data mez
```

Printers are reached from inside the container over the ordinary network, so
the host must be able to reach them on port 9100. On Docker Desktop for
Windows and macOS, a printer on the LAN usually works; if it does not, run the
container on a Linux host.

### Backups in Docker

A backup to a **bucket** needs nothing but the settings: turn it on under
**Settings**, **Backup**.

A backup to a **folder** needs the folder mounted into the container. The
image has `/backup` for it. In `build/docker-compose.yml`, uncomment one of
the two lines under "A backup to a folder":

- a folder of the host's on another disk (`/mnt/usb/mez-backup:/backup`),
  made writable by user 65532, which is who mez runs as in the container:
  `sudo chown 65532:65532 /mnt/usb/mez-backup`;
- or a named volume (`mez-backup:/backup`), which is on the same disk as
  the venue and so protects against a mistake and not against the disk.

Then `docker compose up -d` again, and under **Settings**, **Backup**
choose **A folder** and enter `/backup`.

As on any Linux, the first time a backup is turned on it starts at once,
and a destination changed after that takes effect at the next
`docker compose restart mez`.

To get a venue back, stop the container and run `restore` with the new
volume and the backup both mounted:

```sh
docker compose -f build/docker-compose.yml stop
docker run --rm -v mez-data:/data -v /mnt/usb/mez-backup:/backup mez:local restore -from /backup
docker compose -f build/docker-compose.yml up -d
```

`mez-data` must have no venue in it: on a new machine it does not, and on
one that has, restore into a new volume and point the compose file at it.
From a bucket, pass the bucket's flags and `-e
MEZ_RESTORE_SECRET_ACCESS_KEY=...` in place of the mount.
[Backups](#getting-a-venue-back) has the rest.

This was run on 2026-10-07 with Docker 29 on Linux: the image built, a venue
was set up in it, it backed up to a named volume mounted at `/backup`,
stopped in half a second, was restored into a new volume by the command
above, and started as the venue it had been.

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

**What protects it.** Everything but pairing, the health check, menu
photos, the setup wizard (which answers only to `localhost`) and the guest
pages under `/order` needs a paired device's token (the live stream
included), and pairing codes
are issued only by a signed-in manager, expire in ten minutes, and are rate
limited. Five wrong PINs in fifteen minutes lock a person for the rest of
that time, and the third such lock in a day holds until a manager clears it
under **Staff** (or `mez unlock` at the machine). That is
designed for a venue's own network. On the open internet, also:

- always serve HTTPS, never plain HTTP;
- revoke any device you no longer use, under **Devices**;
- give staff PINs of six or more digits;
- keep the server's OS updated, and expose only ports 80 and 443.

## Guests ordering from their phones

A guest scans the code on their table, sees the menu on their own phone,
and orders; the order arrives at the till as a request for staff to accept.
The phone has to be able to reach the server, and the server is in the
building with no address on the internet. A **relay** gives it one: mez
dials out to the relay and holds the connection open, and the relay passes
each guest's request down it. Nothing at the venue is opened to the
internet, and no port is forwarded on the router.

It is optional. Without it mez calls nothing on the internet, and guests
order at the counter.

**1. Get an address and a key.** From whoever runs the relay, or run one
yourself ([`build/relay/README.md`](../build/relay/README.md): one small
server and a domain). You are given:

- the relay's address, such as `https://relay.example.com`;
- a key for your venue, which begins `mzr_`. It is a secret: anyone holding
  it can answer guests in your venue's name;
- the address guests reach your venue at, such as
  `https://my-cafe.example.com`.

**2. Give mez three settings.**

| Setting | Value | For |
|---|---|---|
| `relay_url` | The relay's address, with nothing after the host | Where mez dials |
| `relay_key` | The key | Who mez says it is |
| `public_url` | The address guests reach your venue at | What is drawn into each table's code |

All three are needed. Without `public_url` the link comes up but no code
can be printed: the back office says "The venue has not said where guests
reach it."

Where they go ([`running.md`](running.md#changing-the-port-and-everything-else)
has every way):

- **Windows, installed**: a file `C:\Program Files\mez\config.toml`, made
  in an editor opened as administrator:

  ```toml
  relay_url  = "https://relay.example.com"
  relay_key  = "mzr_my-cafe...."
  public_url = "https://my-cafe.example.com"
  ```

  mez restricts that file to administrators and its own account each
  time it starts; between writing the file and the next start, any account
  on the computer can read it, the key with it. Restart the service after
  editing it.
- **Linux, under systemd**: in `/etc/mez/mez.env`, readable only by root
  (`sudo mkdir -p /etc/mez && sudo install -m 600 /dev/null
  /etc/mez/mez.env` first):

  ```
  MEZ_RELAY_URL=https://relay.example.com
  MEZ_RELAY_KEY=mzr_my-cafe....
  MEZ_PUBLIC_URL=https://my-cafe.example.com
  ```

- **Docker**: the same three names under `environment:` in
  `build/docker-compose.yml`, where they wait commented out.

**3. Restart mez**, since settings are read when it starts: `mez service
stop` and `start`, `sudo systemctl restart mez`, or `docker compose up -d`.
A setting that cannot be right stops mez from starting, and it says which.

**4. Check it.** In the back office, **Devices**, **Ordering from phones**
should say "Guests can order from their phones. Connected since ...". Then
open the address guests use on a phone that is **not** on the venue's
Wi-Fi.

**5. Print the codes.** In the back office, on the **Floor plan**, each
table has a code and there is a sheet of all of them to print and cut up.
A code holds the address, so changing `public_url` later means printing
them again.

What to know before relying on it:

- **It needs the internet, and only it does.** When the venue's internet or
  the relay is down, a guest sees "Ordering is offline ... Please order at
  the counter", and the counter goes on as before.
  [`troubleshooting.md`](troubleshooting.md#guests-cannot-order-from-their-phones)
  has what each state means and what to do.
- **Whoever runs the relay can see what passes through it**: what guests
  order, and from which table. It keeps none of it, but the guest's secure
  connection ends at the relay and a second one carries it on to you. Use a
  relay run by someone you would trust with that, or your own.
- **One computer per key.** A second mez started with the same key takes
  the guests from the first.
- **A relay allows each venue 1200 requests a minute** unless its operator
  raised it, which a full room of phones can pass (ADR-246). Guests past
  the limit are asked to try again.
- **Taking payment from guests' phones** is a further setting, a payment
  gateway's keys: see `gateway` in
  [`running.md`](running.md#changing-the-port-and-everything-else). The
  gateway reports back to `public_url`, so it needs the relay up too.

## The back office from anywhere

The owner opens the back office in a browser on their own phone, tablet or
computer, away from the venue: at home, before opening, to change a price
or look at yesterday. It is the same back office on each: on a computer
with its sidebar, on a phone with the sections behind a menu button. It
goes through the same relay that carries guests' orders, so nothing more is
opened at the venue. It is off until you turn it on.

**What can be done from away, and what cannot.** The menu and its prices,
stock and recipes, the floor plan and its table codes, staff and their
PINs, the reports and the day's files, the audit, and the receipt, tax and
venue settings: all as at the venue. Nothing that takes money or works the
room: no orders, payments, voids or refunds, no closing the day or pooling
tips, no printing, no printers, no kitchen. Three settings are read and not
changed: where payments go, where the backup goes, and how invoices are
numbered. A new device is paired with a code made at the venue. The back
office says "Away from the venue" at the top while it is used this way, and
leaves out what it cannot do there.

**Who gets in.** A device paired as a **Back office**, and on it someone
who manages the venue: the owner, a manager. A cashier's or a waiter's PIN
is refused from away even when it is right, and a till's or a kitchen
display's pairing opens nothing from outside the building.

**Signing in from away is stricter than at the venue.** Three wrong PINs
from away, with no right one from away between them, lock that person
everywhere, at the till too, and the lock does not lift by itself. Someone
else who manages staff clears it at the venue (**People**, the person,
**Unlock**), which cannot be done from away; a venue with nobody else runs
`mez unlock` on its computer ([`running.md`](running.md)). Wrong PINs at
the venue and from away are counted apart, so a fumble at the till does not
bring this closer. A sign-in from away lasts twelve hours and then asks for
the PIN again, and a sign-in made at the venue is not carried home: the
same computer opened from away asks again. If someone is locked this way
and it was not them, a paired phone is in the wrong hands: revoke it
(below) before clearing the lock.

**1. Get a key that allows it.** The relay's operator issues one
([`build/relay/README.md`](../build/relay/README.md): `relay issue-key
my-cafe --remote`). It reads `mzr_my-cafe.<id>.remote.<...>`. The key you
have for guests' orders does not allow it, and cannot be changed into one.
The new key replaces it and does both jobs.

**2. Turn it on.** With the relay's three settings
([above](#guests-ordering-from-their-phones)), two things change:

```toml
relay_url  = "https://relay.example.com"
relay_key  = "mzr_my-cafe.....remote...."
public_url = "https://my-cafe.example.com"
remote_backoffice = true
```

or `MEZ_REMOTE_BACKOFFICE=true` with the new `MEZ_RELAY_KEY` in
`/etc/mez/mez.env` or `build/docker-compose.yml`. Only `true` and `false`
are taken. Restart mez. It does not start with `remote_backoffice` on and
no relay, or on a key that does not allow it, and says which.

**3. Make a code, at the venue.** In the back office, **Devices**, the card
**Back office from anywhere** now shows the address,
`https://my-cafe.example.com/backoffice/`. **Pair a device**, name it
("Priya's phone"), choose **Back office**, **Make a code**. The code lasts
ten minutes and works once.

**4. Open the address on the phone or the computer**, on mobile data or
any network. Enter the code, then sign in with your PIN. It stays paired:
from then on it is the address and the PIN. Each phone and each computer is
paired once, with a code of its own.

**A lost or sold phone:** **Devices**, its row, **Revoke**. That works from
away as well as at the venue. To stop all of it, set `remote_backoffice =
false` and restart, or have the relay's operator revoke the key.

**A venue paired to mez Cloud** (**Devices**, **Connect to mez Cloud**)
needs none of the four steps, and its owner types no PIN from away: they
sign in to their mez Cloud account, and mez Cloud vouches for them to the
venue. Pairing adds two things you will see, and neither is a mistake:

- **A person named for the account, under People.** They are who the
  account's owner is at the venue when they come in from mez Cloud. They
  have the owner's role and no PIN, and cannot be given one, here or with
  `mez reset-pin`: at a till the owner signs in as themselves, with their
  own PIN. They are not on the sign-in tiles.
- **A device called "mez Cloud", under Devices.** It is what those
  sign-ins are counted as coming from. Nobody holds it.

What can be done from away is the same list as above, and so is everything
under "What to know" below. A sign-in lasts twelve hours.

**To shut mez Cloud's sign-in and stay paired**, so guests still order and
the backup still goes: **Devices**, the "mez Cloud" row, **Revoke**. Anyone
signed in that way is out at once. Deactivating the person under People
does the same. Pairing again, at the venue, puts both back.

**If mez Cloud says the venue's clock is wrong**, it is: what mez Cloud
signs is good for five minutes, and a venue computer more than a few
minutes fast or slow refuses it. Set the computer to take its time from
the internet.

The log marks these requests `via=relay by=cloud`.

What to know before relying on it:

- **The relay must give the venue a hostname of its own**
  (`https://my-cafe.example.com`, a relay run with `RELAY_VENUE_DOMAIN`).
  On a relay that reaches venues by path (`.../v/my-cafe/`) the back office
  does not open, and the card shows no address.
- **Nothing moves by itself.** At the venue a screen updates as things
  happen. From away it shows what it last read, and reads again when you
  come back to the tab or reload.
- **Whoever runs the relay can see what passes through it**, as with
  guests' orders, and here that includes your PIN as you type it and
  everything you read. Use a relay run by someone you would trust with the
  till, or your own.
- **It needs the venue's computer on and its internet up.** With either
  down the address answers "Ordering is offline", which is the relay's one
  page for a venue it cannot reach.
- **A relay counts requests from one address**, 120 a minute unless its
  operator raised it (`RELAY_IP_PER_MINUTE`). The limit is set for guests,
  and someone moving quickly between the back office's screens can pass
  it: a screen then says to try again in a few seconds, in the words
  written for guests ("A lot of people are ordering at once").
- **The log says which requests came from away**: each has `via=relay`
  ([`running.md`](running.md)).

## Backups

mez keeps a second copy of the venue somewhere other than the venue's
computer, and keeps it up to date by itself: every half minute it sends
what has changed since the last time. It is off until you say where the
copy should go.

### Turn it on

Sign in to the back office as the owner and open **Settings**, then
**Backup**. Choose where the backup goes:

- **A folder.** Written in full, like `D:\mez-backup` or
  `\\nas\backups\cafe`. Use another disk, a USB drive or a network share.
  A folder on the same disk as mez is lost with that disk.
- **A bucket.** Amazon S3, Cloudflare R2, or anything else that speaks S3.
  You need the bucket's name, an access key ID and its secret, and either
  the region (Amazon) or the service's address (everyone else; for R2,
  `https://<account>.r2.cloudflarestorage.com`).

When you press **Save**, mez tries the place first. It writes a small note
there (`mez-backup.txt`) and keeps the setting only if it could, so a
mistyped folder or a wrong key is found out now, by you, and not on the day
the backup is needed. This can take up to ten seconds; **Back up now**
waits up to thirty.

The card above it says how the backup is doing: **Backing up**, with the
time everything was last there, or **Not backing up**, with the reason.
While it is not backing up the venue keeps selling, and what was sold is
copied when the destination comes back.

On Windows a new destination takes effect at once. On Linux and in Docker
a destination changed while a backup is already running takes effect when
mez is next restarted, and the screen says so.

### What is backed up

- The database: every order, payment, invoice, shift, the menu, the staff
  and the settings.
- The menu's photos.

Not backed up: the logs, `config.toml`, and `backup.json` itself (which
holds the bucket's secret). Keep a note of your payment gateway's keys and
of where the backup goes, somewhere other than the venue's computer.

A restore can go back up to a week: mez keeps a whole copy from each night
for seven days, and the changes in between.

### What to know before you rely on it

- **What a dead computer loses is the last half minute.** The backup
  sends what changed every 30 seconds, so if the disk fails, what was
  sold since the last sending is not in the backup: at most half a
  minute of orders. A bill printed in that half minute is then the only
  record of its sale, and after a restore its invoice number is given
  again to the next sale, because numbering carries on from what the
  backup holds: if a customer shows you such a bill, keep a copy and
  tell your accountant. Stopping mez properly, or shutting the computer
  down, sends everything first and loses nothing. **Back up now** on the Backup
  screen sends at once.
- **The half minute can be changed**, by whoever runs the computer, with
  `backup_sync_interval` in the settings file or
  `MEZ_BACKUP_SYNC_INTERVAL` ([`running.md`](running.md)): from `5s` to
  `10m`, written with its unit. Shorter loses less and asks more of the
  destination. That matters for a bucket, which bills each request: at
  30 seconds a busy cafe makes about 80,000 billed requests a month, a
  few rupees on Cloudflare R2; at 5 seconds, about four times that. A
  folder costs nothing either way, and `5s` is a fair setting for one.
- **A bucket needs the internet when mez starts.** If mez is started with
  no internet, nothing is copied until the bucket first answers, and the
  sales made in that time are sent together when it does. If the
  computer's disk fails before then, those sales are gone. A connection
  that drops after mez has started loses nothing: the changes wait on the
  computer and go when it returns. A folder has no such gap. If your
  internet is unreliable, back up to a folder.
- **A USB drive swapped for another** is noticed within a minute and
  filled from scratch. A bucket that was emptied is noticed within five. On Linux and in Docker, restart mez after swapping.
- **On Linux and in Docker**, a folder has to be made reachable first:
  see [Linux](#linux) and [Backups in Docker](#backups-in-docker).
- **Look at the Backup section now and then.** A backup that has been
  failing for a month is not a backup. The status is on that screen and
  nowhere else yet.
- The destination has a folder `db/` with one folder inside it for each
  unbroken run of backing up, and `photos/`. Do not edit, rename or delete
  anything in them by hand. mez removes its own old runs a week after they
  were replaced.
- If the computer has `AWS_*` environment variables set for some other
  program, they can change where a bucket backup goes. Unset them for mez.

### Getting a venue back

You need the destination (the folder, or the bucket and its key) and a
computer with mez on it and **no venue yet** in its data directory. On a
new computer, install mez and leave the setup wizard unfinished: a new
install that has no venue in it is moved aside by the restore, and one
that has a venue is never touched.

1. Stop mez if it is running. On Windows, in an administrator terminal:
   `mez service stop`. With systemd: `sudo systemctl stop mez`.
2. See what is there:

   ```sh
   mez restore -from D:\mez-backup -list
   ```

3. Restore it:

   ```sh
   mez restore -from D:\mez-backup
   ```

   From a bucket, the secret is read from the environment so that it is
   not left in the terminal's history:

   ```sh
   # PowerShell
   $env:MEZ_RESTORE_SECRET_ACCESS_KEY = "the secret"
   mez restore -bucket corner-cafe -endpoint https://ACCOUNT.r2.cloudflarestorage.com -access-key-id KEY

   # Linux
   MEZ_RESTORE_SECRET_ACCESS_KEY='the secret' mez restore -bucket corner-cafe -region ap-south-1 -access-key-id KEY
   ```

   It restores the database, checks it, restores the photos, and tells you
   whose venue it is and what its last invoice was:

   ```
   Restoring from D:\mez-backup
     The backup begun 7 Oct 2026 14:20, last written 7 Oct 2026 14:23
     The database is restored and checked: C:\ProgramData\mez\mez.db
     1 menu photos restored.
     Corner Cafe: 3 invoices, the last INV/26-27/000003, issued 7 Oct 2026 14:22.
   ```

4. Start mez. Sign in and check the last invoices are the ones you expect
   before selling on it. The next invoice carries on from the last.
5. Turn the backup on again under **Settings**, **Backup**. A restored
   venue does not back up until you do: it begins a backup of its own
   beside the one it came from, which is left as it was.

`mez restore` never writes over a venue. If there is a database in the way
it stops and says so; move the data directory aside first if you mean to
replace it.

Tills and tablets that were paired stay paired, if the new computer has the
same address as the old one. Otherwise pair them again.

To restore into somewhere other than the default data directory, add
`-data-dir`. To restore an older run than the newest, take its name from
`-list` and add `-generation NAME`. A bucket kept under a folder other than
`mez` takes `-path`, and a saved `backup.json` can be handed over whole
with `-config` instead of typing the destination again.

### Reading a backup without restoring it

`mez mirror` opens the back office over a copy of a backup, to read: the
reports, the menu, the stock and the staff as they were when the backup was
last written. The venue's computer does not need to be on, and nothing is
restored onto anything. It runs on any computer that can reach the backup.

```
mez mirror -from D:\mez-backup
```

or a bucket, or a saved `backup.json`, exactly as `mez restore` takes them.
It says what it has opened and how to get in:

```
A mirror of Corner Cafe, as it was at 11 Oct 2026 23:10.

  Open          http://127.0.0.1:8090/backoffice/
  Pairing code  7K3M-9QX2   (for a browser not paired to this mirror before; good
                until 23:42, once. Start the mirror again for another)
  Remembered    0 browsers paired before, in C:\Users\you\AppData\Roaming\mez\mirror
```

Open the address in a browser on that computer, enter the code, and sign in
with the PIN of someone who manages the venue: the PINs are the venue's own,
as the backup holds them. The back office says "Away from the venue". A
browser is paired once: the next time the mirror is started it is
remembered, and only the PIN is asked.

What to know:

- **Nothing can be changed.** Anything that would change the venue is
  refused, in a sentence, whatever the screen offers. Nothing is printed,
  and no guest can order from it.
- **It follows the backup while you are using it.** Once a minute it
  looks for anything the venue has backed up since, and shows it; whoever
  is signed in stays signed in. Every screen it draws is as of a moment,
  and that moment moves. Left open and untouched it asks the backup
  nothing, so the first screen after a quiet spell is the old copy, and
  the next is up to date. `-follow 5m` looks less often, `-follow 0`
  never: the mirror then stays as it was when started.
- **It listens on its own computer only**, at `127.0.0.1:8090`, unless
  `-listen-addr` says otherwise.
- **Three wrong PINs in a row lock that person on the mirror for a day.**
  Not at the venue: the venue never hears of it. While locked, the right
  PIN reads as a wrong one, and starting the mirror again does not lift
  it. Nothing on a screen lifts it either, since a mirror changes nothing:
  it lifts by itself after a day.
- **What a mirror remembers is one small file for each venue**, in
  `mez/mirror` under your configuration directory (the mirror says where
  when it starts; `-state-dir` puts it elsewhere): the browsers paired to
  it and the locks. No PIN and no token is in it. Deleting the file unpairs
  every browser and lifts every lock.
- **The venue's own tills and back offices do not open a mirror.** Only a
  browser paired with the mirror's own code does, so a device revoked at
  the venue has no way in through an old backup.
- **The copy is kept in the system's temporary folder and removed when the
  mirror stops** (Ctrl-C). A mirror that is killed leaves it behind, and
  the next one started says where: it is a copy of the venue's sales, so
  remove it.
- **The backup is only read.** A mirror writes nothing to the folder or
  the bucket, and the venue never hears of the browser paired to it.
- `-generation NAME` opens an older run than the newest, by its name from
  `mez restore -list`.

This procedure was carried out on 2026-10-07 on Windows: a venue made three
sales, its server was killed without warning, and it was restored from its
backup folder into an empty directory. All three invoices and the photo
were there, mez started on it, the paired till signed in, and the next sale
was invoice four.

### Copying it by hand

You can still copy the data directory yourself, for a snapshot before an
upgrade. The database is SQLite in WAL mode, so copy it while mez is
**stopped**, or the copy may miss the latest sales.

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

On Windows, stop the service, copy `C:\ProgramData\mez`, and start it
again.

## Upgrading

1. Take a backup: check **Settings**, **Backup** says Backing up and press
   **Back up now**, or copy the data directory by hand as above. Migrations
   only go forward, and an older mez is
   not built to run on a database a newer one has migrated, so the backup is
   how you go back.
2. Put the new mez in place. Migrations apply by themselves when it starts.
   - **Windows, installed:** run the newer installer over the old one. It
     stops the service, replaces `mez.exe` and starts it again.
   - **Windows, by hand:** `mez service stop`, replace `mez.exe`,
     `mez service start`.
   - **Linux:** `sudo systemctl stop mez`, `sudo install -m 755
     mez_<version>_linux_amd64 /usr/local/bin/mez`, `sudo systemctl start
     mez`. Install the release's `mez.service` over the old one too, then
     `sudo systemctl daemon-reload`: set `MEZ_TIMEZONE` in it again, and
     anything in `/etc/mez/mez.env` or a drop-in is kept.
   - **Docker:** change the image's version (or `git pull` and build), then
     `docker compose up -d`.
3. Check `/api/v1/health` reports the new `version` and `"db": "ok"`.

Paired devices stay paired across an upgrade.
