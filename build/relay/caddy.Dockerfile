# Caddy with the Cloudflare DNS module, which stock Caddy does not carry and
# a wildcard certificate needs.
FROM caddy:2-builder AS build
RUN xcaddy build --with github.com/caddy-dns/cloudflare

FROM caddy:2
COPY --from=build /usr/bin/caddy /usr/bin/caddy
