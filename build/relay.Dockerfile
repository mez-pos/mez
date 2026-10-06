# The optional relay in a container: one static binary, no frontends, no
# database, nothing on disk. See ADR-008 and ADR-201.
#
# Build from the repository root:
#   docker build -f build/relay.Dockerfile -t mez-relay .
# Run it behind something that terminates TLS, bound to this machine only,
# so that the proxy is the one thing that can reach it. RELAY_TRUST_PROXY
# is for that proxy's word on who the guest is; set it only when a proxy is
# in front, or a guest could name their own address and have it believed.
#   docker run -p 127.0.0.1:8080:8080 -e RELAY_SECRET=... -e RELAY_TRUST_PROXY=true mez-relay
# Make a key for a venue with the same secret:
#   docker run --rm -e RELAY_SECRET=... mez-relay issue-key my-cafe

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/relay cmd/relay
COPY internal/relay internal/relay
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" -o /out/relay ./cmd/relay

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/relay /relay
ENV RELAY_LISTEN_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/relay"]
