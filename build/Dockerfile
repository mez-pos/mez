# mez in a container. The same binary as the Windows .exe, built from source
# with the three frontends embedded, on an image with nothing else in it.
#
# Build from the repository root:
#   docker build -f build/Dockerfile -t mez .
# Or use build/docker-compose.yml. See docs/deploy.md.

# 1. The three frontends. One npm workspace, so every package.json comes first
#    and `npm ci` is cached until a dependency changes.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
COPY web/terminal/package.json terminal/
COPY web/backoffice/package.json backoffice/
COPY web/guest/package.json guest/
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# 2. The server, static and without cgo (ADR-003), with the bundles embedded.
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/terminal/dist web/terminal/dist
COPY --from=web /src/web/backoffice/dist web/backoffice/dist
COPY --from=web /src/web/guest/dist web/guest/dist
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath -tags frontend \
      -ldflags "-s -w -X main.version=${VERSION}" -o /out/mez ./cmd/mez \
 && mkdir -p /out/data

# 3. Nothing but the binary. The timezone database is inside it, and it runs
#    as an unprivileged user that owns the data directory.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/mez /mez
COPY --from=server --chown=65532:65532 /out/data /data
ENV MEZ_DATA_DIR=/data \
    MEZ_LISTEN_ADDR=:7777
VOLUME /data
EXPOSE 7777
ENTRYPOINT ["/mez"]
