# Troubleshooting

The five things most likely to go wrong at a venue, each starting from what
you see on the screen, in the words mez uses, then what to check, in order.

- [Where to look first](#where-to-look-first)
- [The printer is not found, or will not print](#the-printer-is-not-found-or-will-not-print)
- [A tablet or till cannot reach the server](#a-tablet-or-till-cannot-reach-the-server)
- [Windows warns about the installer](#windows-warns-about-the-installer)
- [Guests cannot order from their phones](#guests-cannot-order-from-their-phones)
- [The cash drawer will not open](#the-cash-drawer-will-not-open)

If mez itself will not start, see
[`running.md`](running.md#when-it-will-not-start). The steps here are
written from the code and from runs on a developer's computer; none has yet
been followed at a venue with a real printer on the counter.

## Where to look first

| What | Where |
|---|---|
| Whether this till is talking to the server | The word at the top of the till: **Live**, **Reconnecting**, **Server unreachable**, **No network** |
| What did not print, and why | **Print queue**, on the till |
| Whether guests' phones can reach the venue | Back office, **Devices**, **Ordering from phones** |
| The server's own account of anything | `logs/mez.log` in the data directory (`C:\ProgramData\mez\logs\mez.log` on Windows). On Windows, warnings and errors are also in Event Viewer, **Windows Logs**, **Application**, source **mez**. Under systemd, `journalctl -u mez` |
| Whether the server is up, and which build it is | `http://localhost:7777/api/v1/health`, in a browser on the server |

The screens say little about network failures on purpose, and the log says
all of it. When a screen says only that something could not be reached, the
log has the address and the reason.

## The printer is not found, or will not print

[`hardware.md`](hardware.md) says what a printer has to be and how it is
connected. Start there if the printer is new.

### The setup wizard's search finds nothing

The search looks for anything answering on port 9100 on the networks the
computer is on, and on Windows for printers installed on the computer. It
says what it looked at: "Searched 192.168.1.0/24 and this computer's own
printers." If it says "This computer is not on a network that could be
searched", the computer has no network connection of its own.

1. **Is the printer on and connected?** A network printer has a light on
   its network socket. Print its self-test slip (hold the feed button while
   switching it on): it shows the printer's IP address.
2. **Is that address on the network that was searched?** If the search
   said `192.168.1.0/24` and the printer's address begins `192.168.0.` or
   `10.`, they are on different networks. This happens with a guest Wi-Fi,
   a second router, or a printer still on its factory address. Put the
   printer on the same network as the computer: its manual says how to set
   its address.
3. **Enter it by hand.** Choose **Enter its address** and type the IP from
   the self-test slip. The search is a convenience; the address is what
   matters.
4. **A USB printer** is found only once it is installed in Windows. If
   Windows's own Printers list does not show it, mez cannot either.

The search is in the setup wizard only. In the back office, under
**Printers**, a printer is added by its address or its Windows name.

### Prove the printer, apart from mez

From the server's computer, in a terminal:

```sh
mez --test-print 192.168.1.50
```

If a slip comes out, the printer, its address and the network are all
right, and the problem is in what mez has been told: check the address and
the paper width under **Printers**. If it answers

```
test-print: escpos: connecting to the printer at 192.168.1.50:9100: ... Check the printer is on, on this network, and at that address
```

then nothing at that address is taking prints, and no setting in mez will
change that. Go back to step 1 above.

### It printed yesterday, and today the Print queue says "Could not reach the printer"

Nothing is lost: every print is kept and tried again, ten times over about
a quarter of an hour, then it waits. Once the printer is back, open
**Print queue** on the till and choose **Retry**.

1. Is it switched on, with paper, and its lid shut?
2. Is its cable in at both ends?
3. **Has its address changed?** This is the usual cause after a router was
   restarted or a night passed. Print the self-test slip and compare the
   address with the one under **Printers**. If it differs, correct it
   there, then give the printer a fixed address in the router (a DHCP
   reservation) so it does not happen again.
4. The log has the reason the network gave, on lines reading
   `print job failed, will retry` and `print job dead, needs a person`.

### Other things the Print queue says

- **"no printer is set up for this; set one up and retry"**: there is no
  receipt printer, or the item's station has no printer. Set it under
  **Printers**, **Where things print**, then **Retry**. A station that is
  meant to have no printer will keep showing these.
- **"no printer named ... on this computer"**: a USB printer's name in mez
  does not match any printer Windows has. It was renamed, removed, or
  mistyped. Copy the name exactly from Windows's Printers list.
- **"... uses the Windows spooler, and this server is not on Windows"**: a
  USB printer was set on a Linux or Docker server. Use a network printer.

### It prints, but wrongly

Lines wrapping, stray characters where the barcode should be, question
marks in place of names, no cut: these are about what the printer can do,
and [`hardware.md`](hardware.md#try-it-before-service) covers each.

## A tablet or till cannot reach the server

What you see depends on how far it got:

- The browser's own error page ("This site can't be reached"), before any
  mez screen appears.
- On the pairing screen: "Can't reach the mez server at 192.168.1.20:7777.
  Check this device is on the venue's network."
- On a till that was working: **Server unreachable** or **No network** at
  the top, with a **Retry now** button. **Reconnecting** for a few seconds
  is normal and mends itself.

A till needs the server, and the server needs no internet. If the venue's
internet is down and the tills still say **Live**, nothing is wrong.

Check in this order. Each step rules out everything before it.

1. **Is mez running?** On the server's own computer, open
   `http://localhost:7777`. If the till opens there, mez is up, and the
   trouble is between the tablet and the computer: go to step 2. If not:
   - Windows, installed: in a terminal opened as administrator,
     `& "C:\Program Files\mez\mez.exe" service status`, and
     `service start` if it is stopped. If it will not stay up, Event
     Viewer says why (see the table above). A port already in use is the
     common one.
   - Linux: `systemctl status mez`, then `journalctl -u mez -n 50`.
   - Docker: `docker compose ps`, then `docker compose logs mez`.
2. **Is the address right?** A tablet uses the computer's address on the
   network, `http://192.168.1.20:7777`, never `localhost`, and `http`, not
   `https`. The setup wizard's last screen shows the address, with a QR
   code. To find it again on Windows, run `ipconfig` and read the **IPv4
   Address** of the Ethernet or Wi-Fi adapter.
3. **Has the computer's address changed?** If every tablet stopped at
   once, after a restart of the router or the computer, it probably has.
   Find the new one as in step 2, and reserve it in the router so that it
   stays.
4. **Is the tablet on the same network?** A tablet on the guest Wi-Fi, or
   on mobile data, is not. Many routers also keep Wi-Fi devices from
   reaching each other ("client isolation", "AP isolation", or a guest
   network): turn that off for the network the tills are on.
5. **Is Windows's firewall letting it in?** The installer adds a rule
   named `mez`, for networks Windows calls **Private** or **Domain**. If
   Windows thinks the venue's network is **Public**, tablets are kept
   out, and that is the usual cause when mez works on the computer and
   nowhere else.
   - In Windows settings, **Network & internet**, open the connection and
     set the network profile to **Private**.
   - If the installer's box "Let tills, tablets and kitchen screens on
     this network connect" was unticked, or mez was set up by hand, add
     the rule: [`deploy.md`](deploy.md#windows), "Let tablets in".
   - Other security software with its own firewall needs the same
     permission given to `mez.exe`, port 7777.
6. **Was mez told to listen only to itself?** If `listen_addr` is
   `127.0.0.1:7777` (in `config.toml`, or a flag the service was
   installed with), only the computer can reach it. That is right behind
   a proxy and wrong for a counter: it should be `:7777`.

### It reaches the server, and says something else

- **"This venue is not set up yet"**, with "Setting up is done on the
  computer mez is installed on": the server is fine and has no venue. Set
  it up on that computer first.
- **"That code didn't work. Codes last 10 minutes and work once..."**:
  make a new code in the back office under **Devices**, **Pair a device**,
  or with `mez pair` at the computer.
- **"That code pairs a kitchen display, and this is the terminal"** (or
  the other way round): the code was made for another kind of device.
  Make one for this kind.
- **"Too many tries. Wait ..."**: wait as long as it says.
- **Device unpaired**, or **Device not recognised**, "pair it again": the
  device was removed under **Devices**, or the server's data was replaced
  (a restore on a new computer, a new install). Pair it again.

## Windows warns about the installer

The installer is not signed, because this project has no certificate. So
when you run it, a blue box says **"Windows protected your PC"**, with only
a **Don't run** button in sight.

1. Click **More info**. The file's name appears, and a second button.
2. Click **Run anyway**.

Your browser may warn first, that the file "isn't commonly downloaded":
choose to keep it.

The warning means Windows has not seen this file often and it carries no
publisher's certificate. It does not mean Windows found anything in it.
Being asked twice about a file from the internet is fair, so check it is
the file that was published:

```powershell
Get-FileHash .\mez_<version>_windows_amd64_setup.exe -Algorithm SHA256
```

and compare the result with that file's line in
`mez_<version>_checksums.txt` on the same release page. Download it only
from this repository's releases.

**If there is no "More info", or no "Run anyway"**, the computer has been
set not to allow it, and clicking will not get past that:

- A computer managed by a company or a school may block unsigned programs
  by policy. Whoever manages it has to allow it.
- Windows 11's **Smart App Control**, where it is on, blocks unsigned
  programs outright and offers no way round for one file. It is under
  **Windows Security**, **App & browser control**. Turning it off is a
  decision about the whole computer, not about mez; read what Windows says
  there before you do.

The plain `mez_<version>_windows_amd64.exe` is unsigned too and meets the
same warning. Building mez from source on the computer itself does not.
The [README](../README.md#the-windows-installer-and-the-warning-windows-shows)
says the same at more length.

After the installer runs, it may say one of three things itself:

- "mez was copied to this computer, but could not be set to start with
  Windows", with the reason below it.
- "mez is installed, but did not start", with the reason.
- "mez is installed and was started, but is not answering yet": wait a
  minute and open the shortcut again.

For all three: Event Viewer, **Windows Logs**, **Application**, source
**mez** says in words why.

## Guests cannot order from their phones

Guests reach the venue through a relay on the internet, which the server
dials out to. When that link is down, a guest who scans a table's code
sees **"Ordering is offline"**: "This cafe's online ordering is not
reachable right now. Please order at the counter."

**The counter is not affected.** Tills, the kitchen display and printing
go on working. Only ordering from phones stops.

Open the back office, **Devices**, **Ordering from phones**. It says which
of these it is, when it will try again, and below that the reason the last
try failed:

| It says | What it means | What to do |
|---|---|---|
| "Guests order at the counter. Ordering from a phone needs a relay address and key in this venue's settings file." | No relay is set. Nothing is wrong | To turn it on: [`deploy.md`](deploy.md#guests-ordering-from-their-phones) |
| "Reaching the relay. Guests cannot order from their phones until it answers." | It is connecting now | Wait a few seconds |
| "The relay cannot be reached, so guests cannot order from their phones. The counter is not affected." | The last try failed, and another is coming | See below |
| "The relay does not accept this venue's key..." | The key is wrong, or was turned off at the relay | Check `relay_key` was copied whole. Otherwise ask whoever runs the relay for a new one |
| "Another computer is connected with this venue's key, and guests reach that one..." | Two copies of mez are using one key | Stop the other (an old computer, a test copy), or give this one its own key |
| "The link to the relay is stopped. It starts again when mez does." | mez is shutting down, or the link ended for good | Restart mez |

When **the relay cannot be reached**, mez keeps trying by itself, a second
after the first failure and then less often, up to once a minute. It needs
nothing from you once the cause is gone. Find the cause in this order:

1. **Is the venue's internet up?** Open any website on the server's
   computer. If the internet is down, this is why, and it mends itself
   when the internet returns.
2. **Is the relay up?** Open the relay's address with `/healthz` on the
   end, such as `https://relay.example.com/healthz`, from any device. If
   it does not answer, the trouble is at the relay, and whoever runs it
   has to mend it ([`build/relay/README.md`](../build/relay/README.md) if
   that is you).
3. **Is something at the venue blocking it?** The link is an outgoing
   secure connection on port 443, as a website is. A firewall or filter
   that allows only named sites has to allow the relay's.
4. **Read the reason.** The line under the status, and the log's lines
   reading `tunnel: not connected`, carry the error as the network gave
   it.

A key that is refused, and a key in use elsewhere, are tried again only
every five minutes. After mending either, restart mez and it connects at
once.

**If mez will not start after the relay was set**, the setting itself is
wrong, and mez stops with the reason rather than start without it:
`relay_url is set but relay_key is not`, `relay_key does not look like a
venue key`, or `relay url ...: scheme must be wss or https`. Correct the
setting, or remove both to run without a relay.

The health check says the same in one word:
`"tunnel": "ok"`, `"failing"`, or `"unknown"` with no relay set.

## The cash drawer will not open

The drawer is opened by the receipt printer, when mez tells it to.
[`hardware.md`](hardware.md#the-cash-drawer) has the whole of how. Check
in this order:

1. **Was it meant to open?** It opens for a sale that took cash, and for a
   refund that gave cash back. It does not open for card or UPI, for a
   reprinted receipt, for a Z-report, for cash paid in or out, or for the
   count at the end of a shift. No button on the till opens it. For those,
   use the drawer's key.
2. **Did the receipt print?** The drawer is opened by the same print. If
   no receipt came out, the printer is the problem: see
   [the printer section](#the-printer-is-not-found-or-will-not-print), and
   use the key meanwhile.
3. **Is the drawer plugged into the receipt printer?** Into the socket on
   the printer marked DK or with a drawer symbol, and into the printer
   that prints receipts, not the kitchen's. It is a different socket from
   the network one beside it.
4. **Is the drawer locked?** Many drawers' keys have a locked position
   in which nothing opens them.
5. **Test it apart from a sale**, from the server's computer:

   ```sh
   mez --test-print 192.168.1.50 -drawer
   ```

   The slip ends "Cash drawer: kicked. It should be open."

If the slip prints and the drawer stays shut, with the plug in and the
drawer unlocked, the printer was told and did not do it. What is left:

- **The printer does not act on the command mez sends.** mez sends one
  kind of drawer command and has no setting for another. Some printers
  take only the older kind, and some have this one switched off in their
  own settings: look in the printer's manual or its setup tool for
  "real-time commands" or "drawer kick".
- **The drawer is wired to the other pin.** mez pulses pin 2. A drawer
  sold as a "second drawer", or with an unusual cable, may answer only on
  pin 5.
- **The drawer and printer do not suit each other**: a 12 volt drawer on a
  24 volt printer, or the other way round.
- **The cable or the drawer has failed.** Try the drawer on another
  printer, or another drawer on this one.

For a USB printer, `-drawer` with `-save` writes the page to a file to
send through Windows ([`hardware.md`](hardware.md#try-it-before-service)).
