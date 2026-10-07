# Hardware: printers and cash drawers

What mez needs from a receipt printer and a cash drawer, how it reaches
them, and what it cannot use. Read it before buying anything.

**No printer has been printed on yet.** Everything here is taken from the
bytes mez sends, which its tests check against the published command set,
and not from a printer on a counter. No make or model is listed as working
because none has been tried. The test print below takes a minute and tells
you about yours; if you run it, the result is worth an issue on the
repository, pass or fail.

- [What a printer needs](#what-a-printer-needs)
- [Two ways to connect one](#two-ways-to-connect-one)
- [Paper](#paper)
- [Try it before service](#try-it-before-service)
- [What is printed, and where](#what-is-printed-and-where)
- [What the text can hold](#what-the-text-can-hold)
- [The cash drawer](#the-cash-drawer)
- [When a print does not come out](#when-a-print-does-not-come-out)
- [Finding a printer on the network](#finding-a-printer-on-the-network)
- [Not supported](#not-supported)

## What a printer needs

A thermal receipt printer that speaks **ESC/POS**, the Epson TM command
set. Most 80mm and 58mm receipt printers sold to cafes do, under many
names; the box or the manual says "ESC/POS" or "Epson compatible".

mez uses a small part of it, and a printer has to do all of this part:

| What | Command | Used for |
|---|---|---|
| Reset, and code page PC437 | `ESC @`, `ESC t 0` | The start of every print |
| Left, centre, right | `ESC a` | Headings, totals |
| Bold, underline | `ESC E`, `ESC -` | Totals, the test page |
| Wider and taller text | `GS !` | The venue's name, the total, the table on a kitchen ticket |
| Feed | `ESC d` | Space between parts |
| Barcode, Code 128 | `GS k 73` | The invoice or credit note number at the foot of a receipt |
| QR code, model 2 | `GS ( k` | The test page only |
| Cut | `GS V` | The end of every print |
| Drawer pulse | `DLE DC4` | The cash drawer, see [below](#the-cash-drawer) |

A printer with no cutter should pass over the cut and leave you to tear the
paper. A printer that cannot draw a Code 128 barcode prints the receipt
without one, or prints stray characters in its place: the test page shows
which.

## Two ways to connect one

**On the network**, by its IP address. This is the way to prefer, and the
only way on Linux, in Docker and on macOS. The printer has an Ethernet or
Wi-Fi port and listens on TCP port 9100, as network receipt printers do.
mez opens a connection, writes the print, and closes it.

- The address is an IP on the venue's own network, such as `192.168.1.50`,
  with `:port` after it if the port is not 9100. A name (`printer.local`)
  is refused, and so is any address out on the internet (ADR-259).
- Give the printer a **fixed address**: a DHCP reservation in the router,
  or a static address set on the printer. A printer whose address changes
  overnight is a printer mez cannot find in the morning.
- The printer's own self-test slip usually shows its address: hold the feed
  button while switching it on.
- Prefer a cable to Wi-Fi where one can be run. A print is retried when the
  network drops it, but a kitchen ticket that arrives a minute late is
  still late.

**By USB, on Windows only**, through a printer installed on the same
computer mez runs on. In the back office choose **USB, on this computer
(Windows)** and give the printer's name exactly as Windows shows it under
Printers. mez hands the print to Windows as raw bytes, untouched, so:

- The printer must be installed in Windows first, with the maker's driver
  or with Windows's own "Generic / Text Only" driver. Either passes raw
  bytes through.
- It must be plugged into **this** computer. A printer shared from another
  computer (`\\PC\Printer`) is refused.
- The Windows service runs as its own account
  ([`deploy.md`](deploy.md#windows)); a printer installed on the computer
  is installed for every account.

On Linux and in Docker a USB printer cannot be used: a printer set to USB
there fails at once, with the reason on the print queue.

Bluetooth and serial printers are not supported directly. One that Windows
has installed as a printer may work through the USB route above, since
Windows hides the cable; this has not been tried.

## Paper

| Roll | Characters across | Setting |
|---|---|---|
| 80mm | 48 | The default |
| 58mm | 32 | Choose **58mm** for the printer in the back office, `-paper 58` for the test print |

These are the widths in the printer's standard font. A few 80mm printers
print 42 characters across in it, not 48; on those, lines will wrap. The
ruler on the test page shows this at once, and there is no setting for it.

## Try it before service

From the computer mez will run on, with nothing set up and the server not
running:

```sh
mez --test-print 192.168.1.50            # 80mm
mez --test-print 192.168.1.50 -paper 58  # 58mm
mez --test-print 192.168.1.50 -drawer    # and open the drawer
```

One slip should come out and be cut, with:

- a line of digits that fills exactly one line, no more and no less;
- left, centre and right text, bold, underline, and text at double height,
  double width and both;
- a barcode that a phone reads as `NF/26-27/000001`;
- a QR code that a phone reads as `mez printer test OK`;
- with `-drawer`, the drawer open.

If all of that is right, everything mez prints will be. If one part is
wrong, that part is what your printer does not do.

For a USB printer, write the page to a file and send it through Windows:

```
mez --test-print x -save test.bin
copy /b test.bin \\THIS-PC\ShareName
```

in Command Prompt, not PowerShell (the address is not used with `-save`,
and the printer has to be shared under a name for `copy` to reach it). Or
add the printer in the back office under **Printers** and choose **Test
page**, which goes through the same path a receipt does. That test page
never opens the drawer.

## What is printed, and where

| Print | Goes to | Cut | Drawer |
|---|---|---|---|
| Receipt, with the invoice | The receipt printer | Partial, so it hangs | Opens, if cash was taken |
| A reprinted receipt | The receipt printer | Partial | Stays shut |
| Credit note, for a refund or a void | The receipt printer | Partial | Opens, if cash went back |
| Kitchen ticket | The printer of the station each item is made at | Full | |
| Z-report | The receipt printer | Full | |
| Test page | The printer tested | Partial | Only with `-drawer` |

A venue has one receipt printer, and each station (kitchen, bar) may have a
printer of its own or none. They are set in the back office under
**Printers**, **Where things print**. One printer can do everything: make
it the receipt printer and give it every station.

A station with no printer gets no paper ticket: its orders are on the
kitchen display, which needs no printer at all. The ticket is still kept,
unprinted, and the Print queue counts it as not printed. The same goes for
receipts at a venue with no receipt printer.

## What the text can hold

mez sends plain ASCII and nothing else, because it is the one thing every
ESC/POS printer prints the same. So, on paper:

- `₹` prints as `Rs.`, `£` as `GBP`, `€` as `EUR`.
- Accented Latin letters lose their accents: `Café` prints as `Cafe`.
- **Anything else prints as `?`**: Devanagari, Tamil, Arabic, Chinese,
  emoji. An item named in Hindi is a row of question marks on the kitchen
  ticket.

The screens show every script; only the paper is limited. If your menu is
not in Latin letters, give each item a Latin name for now.

## The cash drawer

The drawer plugs into the **printer**, not the computer: the socket on the
back of the printer marked DK or with a drawer symbol, which takes the
phone-style plug on the drawer's cable. mez tells the printer to pulse it.

- It must be on the **receipt printer**. Nothing else is ever told to open
  a drawer.
- It opens when a receipt prints for a sale that took cash, and when a
  credit note prints for cash handed back. It opens as the print starts,
  not after.
- It does not open for a card or UPI sale, for a reprint, or for a
  Z-report.
- **Nothing on the till opens the drawer by itself.** Paying cash in or
  out of the drawer, and counting it at the end of a shift, do not open
  it: use the drawer's key, or leave it on the latch while counting.
- If the receipt printer is off or out of reach, the drawer does not open
  either. Keep the key where the cashier can find it.

What mez sends is fixed: the real-time pulse command (`DLE DC4 1`), on
**pin 2** of the socket, for **200 milliseconds**. There is no setting for
the pin or the length.

- Nearly all single drawers are wired to pin 2. A second drawer on pin 5
  cannot be opened.
- Some printers act only on the older pulse command (`ESC p`), or have the
  real-time one switched off in their settings. On those the drawer will
  not open from mez. `mez --test-print ADDRESS -drawer` settles it in ten
  seconds, and it is the thing to check before buying a printer for a
  till.
- The drawer has to suit the printer: most printers drive a 24 volt
  drawer, some 12. The printer's manual says which.

A drawer with its own USB or network connection is not supported.

## When a print does not come out

Every print is kept in the database before it is sent, as the exact bytes,
so nothing is lost when a printer is off, jammed or out of paper.

- A print that fails is tried again by itself: after 2 seconds, then 4, 8
  and so on up to every 5 minutes, ten times in all, over about a quarter
  of an hour. Long enough to change a roll.
- After the tenth it stops and waits for a person. The till's **Print
  queue** lists it, and it can be sent again from there once the printer
  is back.
- Each printer has its own line: a jammed bar printer holds back the bar's
  tickets and nobody else's.
- The screens say only **Could not reach the printer**. What the network
  said is in the server's log (`mez.log`, or the Windows event log).
- A print the printer accepted counts as printed. A network printer does
  not say whether paper came out, so mez cannot tell an empty roll or an
  open lid from a good print. Somebody looking at the printer can.

## Finding a printer on the network

The setup wizard and the back office's **Printers** screen can search. The
search tries port 9100 on every address of the networks the computer is
on (up to four, 254 addresses each, skipping virtual adapters), and on
Windows lists the printers installed on the computer.

It finds **candidates**. Anything that answers on port 9100 is listed,
which includes office printers that are not receipt printers, and a
printer on another network or a different port is not found. Pick one and
print the test page: that is the proof.

On Linux and in Docker only the network is searched. In Docker the search
looks at the container's network, not the venue's, and finds nothing:
enter the address by hand.

## Not supported

- Printers that do not speak ESC/POS: Star printers in their own Star
  mode (many can be switched to ESC/POS emulation), label printers, and
  office inkjet or laser printers.
- Dot-matrix (impact) kitchen printers. The layouts assume a thermal
  printer's widths, and none has been tried.
- USB printers on Linux, in Docker and on macOS.
- Printers shared from another computer.
- Printing from the tablet or phone itself. Every print goes from the
  server, which is why a tablet needs no printer set up on it.
- Scripts other than Latin on paper, logos and images.
- A second cash drawer, a drawer on pin 5, and a drawer not wired through
  the receipt printer.
- Knowing the printer's state: paper out, lid open, cutter jammed.
- Card machines, barcode scanners, scales and customer displays. A USB
  barcode scanner types like a keyboard and needs nothing from mez, but
  no screen is built around one.
