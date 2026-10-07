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
small venue; build with `GOARCH=arm64`. It is the same binary as on Windows:
one file, no database server, nothing else to install.

```sh
sudo useradd --system --home /var/lib/mez --create-home mez
sudo install -m 755 mez_<version>_linux_amd64 /usr/local/bin/mez
sudo install -m 644 build/mez.service /etc/systemd/system/mez.service
```

[`build/mez.service`](../build/mez.service) is the unit. Before starting it,
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

Without compose:

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

mez keeps a second copy of the venue somewhere other than the venue's
computer, and keeps it up to date by itself: each sale is at the
destination about a second after it is made. It is off until you say where
the copy should go.

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
the backup is needed. This can take up to twenty seconds.

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

- **A bucket needs the internet when mez starts.** If mez is started with
  no internet, nothing is copied until the bucket first answers, and the
  sales made in that time are sent together when it does. If the
  computer's disk fails before then, those sales are gone. A connection
  that drops after mez has started loses nothing: the changes wait on the
  computer and go when it returns. A folder has no such gap. If your
  internet is unreliable, back up to a folder.
- **A USB drive swapped for another** is noticed within a minute and
  filled from scratch. On Linux and in Docker, restart mez after swapping.
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
`-list` and add `-generation NAME`.

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
2. Stop mez, replace the binary (or `git pull` and rebuild the image), start
   it. Migrations apply by themselves on startup.
3. Check `/api/v1/health` reports the new `version` and `"db": "ok"`.

Paired devices stay paired across an upgrade.

## What phase 12 adds

[`phases/phase-12.md`](phases/phase-12.md) is the plan. In short: a Windows
installer (Inno Setup) that registers the Windows service, opens the firewall
and creates the shortcut; a setup wizard, so a new Windows venue needs no `mez setup` (built);
backups configured in the back office (built, [above](#backups)); a systemd unit
([`build/mez.service`](../build/mez.service), built) and an image
published with each release; and release automation with checksums.
