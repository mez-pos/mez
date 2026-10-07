#!/bin/sh
# The dev loop: a dev venue, the server on its own port, and the terminal and
# back office apps under Vite with hot reload. `make dev` runs this after
# building bin/mez. Ctrl+C stops all three.
#
#   DEV_DATA  the dev venue's data directory     (default .dev-data)
#   DEV_PORT  the port the dev server listens on (default 7791, so a Docker
#             venue on 7777 keeps running beside it)
#
# The first run sets the venue up as a demo cafe and prints one pairing code
# for each app. Later runs reuse the venue: a code that has expired, or a
# browser that was never paired, takes `bin/mez pair` (the line below says how).

set -eu

DATA=${DEV_DATA:-.dev-data}
PORT=${DEV_PORT:-7791}
BIN=${DEV_BIN:-bin/mez}
ORIGIN="http://127.0.0.1:$PORT"

# The owner's PIN is known only on the run that sets the venue up: a venue
# made by an older dev.sh, or one whose PIN was changed since, has another.
fresh=""
if [ ! -f "$DATA/mez.db" ]; then
	fresh=yes
	echo "Setting up a dev venue in $DATA"
	# Setup creates the database file before it checks anything, so a setup
	# that fails would leave a file with no venue, which the next run would
	# take for a finished one.
	if ! "$BIN" setup -data-dir "$DATA" -name "Dev Cafe" -owner Priya -owner-pin 246813 \
		-timezone Asia/Kolkata -tax-profile gst_in -tax-registration 29ABCDE1234F1Z5 \
		-demo-menu; then
		rm -f "$DATA"/mez.db "$DATA"/mez.db-shm "$DATA"/mez.db-wal
		exit 1
	fi
	echo "Codes for the other two apps (each lasts 10 minutes):"
	"$BIN" pair -data-dir "$DATA" -device-kind back_office
	"$BIN" pair -data-dir "$DATA" -device-kind kitchen
else
	echo "Dev venue in $DATA. Another code: $BIN pair -data-dir $DATA -device-kind terminal"
	echo "A fresh venue (the demo floor and staff): stop this, delete $DATA, run it again."
fi

pids=""
stop() {
	trap - EXIT INT TERM
	for p in $pids; do
		kill "$p" 2>/dev/null || true
	done
}
trap stop EXIT INT TERM

"$BIN" -data-dir "$DATA" -listen-addr "127.0.0.1:$PORT" &
server=$!
pids="$server"

# Vite's proxy would answer every call with a connection error if the server
# were not up, and a port already taken would only show in the server's log.
# So wait for it, and stop here if it died.
ready=""
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
	if ! kill -0 "$server" 2>/dev/null; then
		echo "The server stopped at startup: is port $PORT in use? Set DEV_PORT to another." >&2
		exit 1
	fi
	# Checked again after the answer: it may be another server on the port.
	if curl -fsS "$ORIGIN/api/v1/health" >/dev/null 2>&1 && kill -0 "$server" 2>/dev/null; then
		ready=yes
		break
	fi
	sleep 0.5
done
[ -n "$ready" ] || echo "The server is not answering at $ORIGIN yet; going on anyway."

# vite is run directly, not through npm, so the pid killed on the way out is
# the one serving.
VITE="$(pwd)/web/node_modules/.bin/vite"
(cd web/terminal && MEZ_DEV_SERVER="$ORIGIN" exec "$VITE") &
terminal=$!
(cd web/backoffice && MEZ_DEV_SERVER="$ORIGIN" exec "$VITE") &
backoffice=$!
pids="$pids $terminal $backoffice"

# Vite keeps to its port and exits if it is taken, which would leave the
# banner below pointing at someone else's server.
sleep 3
if ! kill -0 "$terminal" 2>/dev/null; then
	echo "The terminal's Vite stopped at startup: is port 5173 in use?" >&2
	exit 1
fi
if ! kill -0 "$backoffice" 2>/dev/null; then
	echo "The back office's Vite stopped at startup: is port 5174 in use?" >&2
	exit 1
fi

echo ""
echo "  server       $ORIGIN"
echo "  terminal     http://localhost:5173   (kitchen display: /kitchen)"
echo "  back office  http://localhost:5174/backoffice/"
if [ -n "$fresh" ]; then
	echo "  sign in as Priya, PIN 246813; the demo staff and their PINs are above"
else
	# Not printed: this venue was set up on an earlier run, and what its PINs
	# are now is not something this script knows.
	echo "  sign in with the PINs this venue was set up with"
	echo "  forgotten one? $BIN reset-pin -data-dir $DATA   (lists who there is; add -staff stf_... for a new PIN)"
fi
echo ""

wait
