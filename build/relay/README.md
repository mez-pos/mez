# Deploying the relay

For whoever runs a relay. A venue's owner who has been given an address and
a key wants [`docs/deploy.md`](../../docs/deploy.md#guests-ordering-from-their-phones),
"Guests ordering from their phones".

The optional relay on one small server, behind Cloudflare, with a hostname per
venue (`<venue>.<domain>`, ADR-203). These are the files that ran the relay at
`relay.jskin.in` from 2026-10-05 to 2026-11-02 (ADR-204, retired by ADR-411),
and they will put it anywhere else.

```
guest phone ──https──> Cloudflare ──https──> Caddy ──> relay
                                                         │ one socket per venue
mez at the venue ──dials out to relay.<domain>───────────┘
```

| File | What |
|---|---|
| `deploy.sh` | Ships the relay's source and these files to the server, builds, starts. Run it again to update |
| `compose.yml` | The relay and Caddy |
| `Caddyfile` | TLS with one wildcard certificate, then everything to the relay |
| `caddy.Dockerfile` | Caddy with the Cloudflare DNS module a wildcard certificate needs |

## What you need

- A server with Docker and its compose plugin. One or two cores is plenty: the
  relay stores nothing and a venue is one idle socket.
- A domain on Cloudflare, with two proxied `A` records to the server: `relay`
  and `*`. Other records on the domain are not affected; a name with its own
  record wins over the wildcard.
- A Cloudflare API token that can edit DNS for that one zone, in a file. It is
  for the certificate, and it stays on the server for renewals.
- A firewall that lets port 443 in from
  [Cloudflare's addresses](https://www.cloudflare.com/ips/) only. The relay
  believes Caddy about who a guest is, and Caddy believes Cloudflare, so nothing
  else may reach it. Docker's published ports go around `ufw`: use the
  provider's firewall.
- Cloudflare's SSL mode for the domain on **Full (strict)**.

## Deploy, and update

From the repository root, in a POSIX shell:

```sh
build/relay/deploy.sh root@SERVER example.com path/to/cloudflare-token   # first time
build/relay/deploy.sh root@SERVER example.com                            # every time after
```

Set `SSH` to change how it connects: `SSH="ssh -i ~/.ssh/mez-server"`.

The first run makes `/opt/mez-relay/.env` on the server, with a signing secret
drawn there. **That secret is what makes every venue's key valid.** It is not
printed and not copied anywhere: back it up yourself, or accept that a rebuilt
server means a new key for every venue. Later runs leave `.env` alone.

Only `cmd/relay` and `internal/relay` go to the server. The rest of the
repository does not.

## Running it

On the server, in `/opt/mez-relay`:

```sh
docker compose exec -T relay /relay issue-key my-cafe   # a key for a venue
docker compose logs -f relay                            # who connected, each request
docker compose ps
```

The venue's id is the first part of its guests' address, so it is 1 to 63
of `a-z`, `0-9` and `-`, not starting with `-`. Names the domain needs for
itself are refused (`relay`, `www`, `api`, `mail`, `order` and others). The
command prints the key and nothing else:

```
mzr_my-cafe.0123456789ab.EXAMPLE-not-a-real-signature-EXAMPLE-xxxxx
```

The middle part is the key's id. It is not secret, and it is what the log
and a revocation name. The whole key is a secret: send it to the venue the
way you would send a password. The relay keeps no list of keys, so a key
cannot be read back later; issue another.

A key issued with `--remote` also lets the venue's **back office** cross
the relay, so the owner can open it from home (ADR-306):

```sh
docker compose exec -T relay /relay issue-key my-cafe --remote
```

```
mzr_my-cafe.0123456789ab.remote.EXAMPLE-not-a-real-signature-EXAMPLE-x
```

The scope is signed into the key: a venue cannot add `remote` to a key it
was given. On a venue connected with a plain key the relay answers 404 for
`/backoffice/` and `/api/` without sending anything to the venue, so this
is where a hosted relay's operator decides who has it. The venue still has
to turn it on at its end (`remote_backoffice` in `docs/deploy.md`), and
with it off a remote key changes nothing.

A venue then sets three things, in its `config.toml` or environment:

```toml
relay_url  = "https://relay.example.com"
relay_key  = "mzr_my-cafe...."
public_url = "https://my-cafe.example.com"
```

and its guests reach it at `https://my-cafe.example.com/`. `relay_url` is
the relay's address with nothing after the host: mez adds `/agent` itself,
so an address ending in a path dials somewhere the relay does not answer.
`public_url` is what the venue's table codes are drawn from.

To turn a key or a venue off, add the key id (it is in the log line when the
venue connects) or the venue id to `RELAY_REVOKED` in `.env`, comma-separated,
and run `docker compose up -d`.

## Who is carried, and until when

A key says who a venue is. Whether the relay carries it is a second
question, and with nothing set the answer is yes for every key, for ever:
that is all a relay for your own venues needs.

To carry some venues and not others, or to a date, name them in a file and
point `RELAY_ENTITLEMENTS` at it. One venue to a line:

```
# venue      until        back office
my-cafe      2026-11-12   remote
corner-bar   forever      guests
old-place    never
*            forever
```

- **until** is a day, and the venue is carried through the end of it (UTC);
  or an instant, `2026-11-12T18:30:00Z`; or `forever`; or `never`.
- **back office** is `remote` or `guests`, and `remote` when left out.
  `guests` answers 404 for `/backoffice/` and `/api/` whatever the venue's
  key says. `remote` lets them cross only if the key was issued with
  `--remote` as well: the file can take the back office away, and cannot
  give it to a key that was not issued for it.
- **`*`** is every venue the file does not name. Without that line, a venue
  the file does not name is not carried, so a file with one line in it
  turns every other venue off. Start from `* forever` and add lines above
  it.
- Everything after a `#` is a note to yourself.

In Docker the file has to be put where the relay can read it: uncomment
`RELAY_ENTITLEMENTS` and the `volumes:` lines under `relay:` in
`compose.yml`, write the file as `/opt/mez-relay/entitlements`, and run
`docker compose up -d` once.

After that, **edit the file and leave the relay running.** It is read
again the next time a venue connects, and each connected venue is asked
about again once an hour, and at the moment its date falls if it has one.
A venue whose date has passed is disconnected, its guests see "Ordering is
offline", and it tries again every minute, so a date moved forward takes
hold within a minute of the edit. A file that cannot be read stops the
relay from starting; an edit that cannot be read is logged
(`what was last read stands`) and changes nothing until it is mended.

A venue that is turned away is told why in a word: `expired`, `never` or
`not_listed`. Its own log has it, and so does the relay's, as
`venue not entitled`. `RELAY_REVOKED` is as it was and comes first: a
revoked key or venue is refused as a bad key is, before the file is read.

## Check it

```sh
curl https://relay.example.com/healthz          # {"status":"ok",...}
curl -i https://nobody.example.com/             # 503, "Ordering is offline"
```

The second is right: a venue that is not connected and one that does not exist
get the same answer. Both were asked of `relay.jskin.in` on 2026-10-07 and
answered so.

## Settings

`deploy.sh` writes the first three into `.env`; the rest have defaults and
are added there, and to `compose.yml`'s `environment:`, only to change them.
`relay help` lists them.

| Name | What | Default |
|---|---|---|
| `RELAY_SECRET` | Signs every venue's key. At least 32 characters | Required |
| `RELAY_VENUE_DOMAIN` | Each venue is at `<venue>.<this domain>` | Empty; `deploy.sh` always sets it |
| `RELAY_REVOKED` | Venue ids and key ids to refuse, comma-separated | None |
| `RELAY_ENTITLEMENTS` | A file naming the venues carried, until when, and whether their back office crosses ([above](#who-is-carried-and-until-when)) | None: every key is carried |
| `RELAY_TRUST_PROXY` | `true` when your own proxy sets `X-Forwarded-For`. `compose.yml` sets it, for Caddy | `false` |
| `RELAY_VENUE_RPM` | Requests a minute each venue may receive | 1200 |
| `RELAY_VENUE_BURST` | Requests at once each venue may receive | 300 |
| `RELAY_IP_PER_MINUTE` | Requests a minute one client address may send, across every venue | 120 |
| `RELAY_IP_BURST` | Requests at once one client address may send | 60 |
| `RELAY_LISTEN_ADDR` | Where it listens, in plain HTTP | `:8080` |
| `RELAY_LOG_LEVEL` | `debug`, `info`, `warn` or `error` | `info` |

1200 a minute is less than a full room of phones can ask for at once
(ADR-246); a guest past it is told "A lot of people are ordering at once.
Please try again in a few seconds." Raise it for a busy venue.

The venue's allowance is shared by everyone in the room, so one client is
also held to its own: 120 a minute and 60 at once per address, and at most
16 requests open at a time, which is a phone opening the menu many times
over and far less than a script. A client past it gets the same message,
and nobody else at the venue notices. Raise the address limits only if many
guests reach you from one address, such as a venue whose wifi puts every
phone behind one public IP; with `RELAY_TRUST_PROXY` set, the address is
the one your proxy reports.

Set `RELAY_VENUE_DOMAIN`. A hostname for each venue is the way these files
deploy it and the way it has been run (ADR-203); the relay's older
`/v/<venue>/` addresses, which it serves when the domain is left out, have
not been run with the guest app as it now is, and are not described here.

## What it keeps, and what you must

The relay stores nothing: no venues, no orders, no keys. A guest's request
is passed down the venue's socket and the answer passed back. It does see
them as they pass, since the guest's TLS ends here: say so to the venues
you serve.

- **Back up `/opt/mez-relay/.env`**, somewhere that is not this server. It
  holds the signing secret. Lose it and every venue needs a new key, set
  by hand at the venue.
- **The log is the only record** of who connected: `relay: venue
  connected`, `relay: venue disconnected`, `relay: venue connection
  replaced` (a second computer with the same key), `relay: venue key
  refused`, and a line a request. Docker keeps it until the container is
  made again.
- **Cloudflare's address ranges are written into `Caddyfile`**, as of
  2026-10, and into your firewall. Check both against
  [the list](https://www.cloudflare.com/ips/) now and then: a range
  Cloudflare adds and you have not is guests turned away.
- The relay is not part of a release: there is no image of it to pull. It
  is built on the server by `deploy.sh`, from a clone.
