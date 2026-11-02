# mez in a container. The same binary as the Windows .exe, built from source
# with the three frontends embedded, on an image with nothing else in it.
#
# Build from the repository root:
#   docker build -f build/Dockerfile -t mez .
# Or use build/docker-compose.yml. See docs/deploy.md.

# 1. The three frontends. One npm workspace, so every package.json comes first
#    and `npm ci` is cached until a dependency changes.
#    Built on the machine doing the building whatever the image is for: what
#    comes out is JavaScript, the same for every processor.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
COPY web/terminal/package.json terminal/
COPY web/backoffice/package.json backoffice/
COPY web/guest/package.json guest/
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# 2. The server, static and without cgo (ADR-003), with the bundles embedded.
#    Also on the building machine's own processor, with Go cross-compiling
#    for the image's: an arm64 image built on an amd64 machine then runs
#    nothing under emulation, which is the difference between a minute and
#    half an hour.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/terminal/dist web/terminal/dist
COPY --from=web /src/web/backoffice/dist web/backoffice/dist
COPY --from=web /src/web/guest/dist web/guest/dist
ARG VERSION=docker
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
# Set by BuildKit to what the image is for. Without BuildKit they are empty
# and Go builds for the machine it is on, as it did before.
ARG TARGETOS
ARG TARGETARCH
# The tablet app is carried when the release put it in place (ADR-421).
RUN tags=frontend; \
    if [ -f internal/device/tabletapp/app/mez.apk ] && [ -f internal/device/tabletapp/app/app.json ]; then \
      tags=frontend,tabletapp; \
    fi; \
    CGO_ENABLED=0 GOOS=${TARGETOS:-$(go env GOOS)} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -trimpath -tags "$tags" \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/mez ./cmd/mez \
 && mkdir -p /out/data /out/backup

# 3. Nothing but the binary. The timezone database is inside it, and it runs
#    as an unprivileged user that owns the data directory.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/mez /mez
COPY --from=server --chown=65532:65532 /out/data /data
# Somewhere to mount a backup's folder (docs/deploy.md, "Backups"). It is
# here, owned by the user mez runs as, so that a new named volume mounted on
# it takes that owner: a volume made over a path the image does not have is
# root's, and mez could not write its backup there. Not a VOLUME: with
# nothing mounted, nothing should be kept.
COPY --from=server --chown=65532:65532 /out/backup /backup
ENV MEZ_DATA_DIR=/data \
    MEZ_LISTEN_ADDR=:7777
VOLUME /data
EXPOSE 7777
ENTRYPOINT ["/mez"]
