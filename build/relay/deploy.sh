#!/bin/sh
# Put the relay on a server and start it, or update one that is running.
#
#   build/relay/deploy.sh root@SERVER DOMAIN [CLOUDFLARE_TOKEN_FILE]
#
# Run from the repository root. The server needs Docker with the compose
# plugin and nothing else. The first run makes /opt/mez-relay/.env there,
# with a signing secret drawn on the server; it is never printed and never
# copied back. A later run leaves .env alone, so venue keys stay valid.
# The token file is needed on the first run only. See build/relay/README.md.
set -eu

host=${1:?usage: deploy.sh root@SERVER DOMAIN [CLOUDFLARE_TOKEN_FILE]}
domain=${2:?usage: deploy.sh root@SERVER DOMAIN [CLOUDFLARE_TOKEN_FILE]}
token_file=${3:-}
dir=/opt/mez-relay
ssh=${SSH:-ssh}

[ -f go.mod ] && [ -d cmd/relay ] || { echo "run this from the repository root" >&2; exit 1; }
version=$(git describe --tags --always --dirty 2>/dev/null || echo deployed)

$ssh "$host" "umask 077 && mkdir -p $dir/src"

if ! $ssh "$host" "test -f $dir/.env"; then
	[ -n "$token_file" ] && [ -f "$token_file" ] || {
		echo "first deploy: give the file holding a Cloudflare token that can edit DNS for $domain" >&2
		exit 1
	}
	# The token goes over the connection's stdin, not its command line, so
	# it is in no process list and no shell history at either end.
	tr -d ' \r\n' < "$token_file" | $ssh "$host" "umask 077 && cd $dir && {
		printf 'RELAY_SECRET=%s\n' \"\$(openssl rand -hex 32)\"
		printf 'RELAY_VENUE_DOMAIN=%s\n' '$domain'
		printf 'RELAY_REVOKED=\n'
		printf 'CF_API_TOKEN=%s\n' \"\$(cat)\"
	} > .env"
	echo "made $dir/.env on the server"
fi

# Only what the relay is built from. The rest of the repository, the
# database code and the frontends included, never goes to this server.
tar -cf - go.mod go.sum cmd/relay internal/relay build/relay.Dockerfile |
	$ssh "$host" "rm -rf $dir/src && mkdir -p $dir/src && tar -xf - -C $dir/src"
tar -cf - -C build/relay compose.yml Caddyfile caddy.Dockerfile |
	$ssh "$host" "tar -xf - -C $dir"

$ssh "$host" "cd $dir && RELAY_VERSION='$version' docker compose up -d --build --remove-orphans && docker compose ps"
echo "deployed $version to $host for $domain"
