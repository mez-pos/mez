#!/bin/sh
# What a release needs beside the binaries: their checksums, and its notes.
# The release workflow (.github/workflows/release.yml) runs both; so can
# anyone, to see what a release would say before it is made.
#
#   build/release.sh checksums DIR      write DIR/mez_<version>_checksums.txt
#   build/release.sh notes [VERSION]    print the notes for VERSION (a tag)
#
# VERSION defaults to what `git describe` says of HEAD, as the Makefile's does.

set -eu

version() {
	git describe --tags --always --dirty 2>/dev/null || echo dev
}

# sha256 of each file given, as `sha256sum` prints it. macOS has shasum.
sums() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$@"
	else
		shasum -a 256 "$@"
	fi
}

checksums() {
	dir=${1:?usage: release.sh checksums DIR}
	v=${2:-$(version)}
	out="mez_${v}_checksums.txt"
	# In the directory, so the file names in it are bare: it is checked from
	# wherever the files were downloaded to. Sorted by name, in the C locale,
	# so that two runs over the same files make the same file. Text files (the
	# unit, the compose file) are summed in binary mode like the rest: a
	# checksum that depended on a line ending would be no checksum.
	(
		cd "$dir"
		rm -f "$out"
		files=$(ls -1 | LC_ALL=C sort)
		[ -n "$files" ] || { echo "release.sh: nothing in $dir to sum" >&2; exit 1; }
		# shellcheck disable=SC2086
		sums -b $files | sed 's/ \*/  /' >"$out.tmp"
		mv "$out.tmp" "$out"
	)
	echo "  checksums $dir/$out"
}

notes() {
	v=${1:-$(version)}
	# The tag before this one, if there is one: what is new is what came since.
	prev=$(git describe --tags --abbrev=0 "$v^" 2>/dev/null || true)
	if [ -n "$prev" ]; then
		range="$prev..$v"
		since="since $prev"
	else
		range="$v"
		since="in this first release"
	fi

	cat <<EOF
## Installing

| You have | Download |
|---|---|
| Windows 10 or 11 | \`mez_${v}_windows_amd64_setup.exe\`, the installer |
| Linux, 64-bit | \`mez_${v}_linux_amd64\`, with \`mez.service\` |
| A Raspberry Pi 4 or 5 | \`mez_${v}_linux_arm64\`, with \`mez.service\` |
| Docker | \`docker pull ghcr.io/byjoellobo/mez:${v#v}\`, with \`docker-compose.yml\` |
| macOS, Apple silicon | \`mez_${v}_darwin_arm64\` |

[docs/deploy.md](https://github.com/byjoellobo/mez/blob/${v}/docs/deploy.md) says what to do with each.

**Windows will warn you about the installer.** It is not signed, so
SmartScreen says "Windows protected your PC". Choose **More info**, then
**Run anyway**, after checking the file is the one published here:

\`\`\`powershell
Get-FileHash .\\mez_${v}_windows_amd64_setup.exe -Algorithm SHA256
\`\`\`

and compare it with the line for that file in \`mez_${v}_checksums.txt\`. On
Linux and macOS: \`sha256sum -c mez_${v}_checksums.txt --ignore-missing\`.

**Upgrading?** Back up first: Settings, Backup, **Back up now**. Migrations
only go forward.

## What changed $since

EOF
	# One line a commit, as its author wrote it. Merges say nothing of their own.
	git log --no-merges --reverse --pretty='- %s (%h)' "$range"
}

cmd=${1:-}
[ $# -gt 0 ] && shift
case "$cmd" in
checksums) checksums "$@" ;;
notes) notes "$@" ;;
*)
	sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
