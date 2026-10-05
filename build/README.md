# Build

Packaging and distribution inputs. Nothing here is imported by the Go module.
`make dist` cross-compiles the binaries; this directory is what wraps them.

| File | Status | What |
|---|---|---|
| `Dockerfile` | Here | Builds mez from source with the apps embedded, onto a distroless image (about 30 MB) running as a non-root user, data in `/data` |
| `docker-compose.yml` | Here | The image with a named volume, a restart policy and the timezone |
| `relay.Dockerfile` | Here | The optional relay alone: one static binary with no frontends and no database, configured by `RELAY_*` variables (`relay help` lists them) |
| Inno Setup script | Phase 12, Task 2 | The Windows installer |
| systemd unit | Phase 12, Task 5 | Shipped with releases; [`docs/deploy.md`](../docs/deploy.md#linux) has one to copy today |
| goreleaser config | Phase 12, Task 6 | Tagged releases with checksums |

Build the image from the repository root, since the build needs the whole
source tree:

```sh
docker compose -f build/docker-compose.yml build
```

[`docs/deploy.md`](../docs/deploy.md#docker) covers setup, running, backups
and putting it behind a domain.
