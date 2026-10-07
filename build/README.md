# Build

Packaging and distribution inputs. Nothing here is imported by the Go module.
`make dist` cross-compiles the binaries; this directory is what wraps them.

| File | Status | What |
|---|---|---|
| `Dockerfile` | Here | Builds mez from source with the apps embedded, onto a distroless image (about 50 MB) running as a non-root user, data in `/data`, a backup's folder at `/backup` |
| `docker-compose.yml` | Here | The image with a named volume, a restart policy and the timezone; a backup's folder and a relay, each commented out |
| `mez.service` | Here | The systemd unit for Linux: its own user, `/var/lib/mez`, restarted if it falls over, allowed to write nowhere else |
| `relay.Dockerfile` | Here | The optional relay alone: one static binary with no frontends and no database, configured by `RELAY_*` variables (`relay help` lists them) |
| `relay/` | Here | Deploying the relay to a server behind Cloudflare: a script, compose file and Caddyfile ([`relay/README.md`](relay/README.md)) |
| `installer/mez.iss` | Here | The Windows installer (Inno Setup 6): `make dist && make installer`. `installer/mez.ico` is made from `web/shared/brand/mez-app-icon.svg` |
| goreleaser config | Phase 12, Task 6 | Tagged releases with checksums |

Build the image from the repository root, since the build needs the whole
source tree:

```sh
docker compose -f build/docker-compose.yml build
```

[`docs/deploy.md`](../docs/deploy.md#docker) covers setup, running, backups
and putting it behind a domain.
