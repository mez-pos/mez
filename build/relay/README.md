# Deploying the relay

The optional relay on one small server, behind Cloudflare, with a hostname per
venue (`<venue>.<domain>`, ADR-203). These are the files that put the relay at
`relay.jskin.in` (ADR-204), and they will put it anywhere else.

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

A venue then sets, in its `config.toml` or environment:

```toml
relay_url = "https://relay.example.com"
relay_key = "mzr_my-cafe...."
```

and its guests reach it at `https://my-cafe.example.com/`.

To turn a key or a venue off, add the key id (it is in the log line when the
venue connects) or the venue id to `RELAY_REVOKED` in `.env`, comma-separated,
and run `docker compose up -d`.

## Check it

```sh
curl https://relay.example.com/healthz          # {"status":"ok",...}
curl -i https://nobody.example.com/             # 503, "Ordering is offline"
```

The second is right: a venue that is not connected and one that does not exist
get the same answer.
