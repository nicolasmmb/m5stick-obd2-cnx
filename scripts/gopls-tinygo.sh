#!/bin/sh
# gopls wrapper for TinyGo projects.
#
# gopls needs TinyGo's cached GOROOT and build tags to understand packages
# like "machine" and "tinygo.org/x/espradio". Values are derived from
# `tinygo info <target>` so they survive TinyGo upgrades.
set -eu

TARGET="${TINYGO_TARGET:-m5stick-c}"
TINYGO_BIN="${TINYGO_BIN:-$HOME/.local/bin/tinygo}"
GOPLS_BIN="${GOPLS_BIN:-$HOME/go/bin/gopls}"

# GUI-launched editors often have a minimal PATH.
export PATH="/usr/local/go/bin:$HOME/.local/bin:$HOME/go/bin:$PATH"

info="$("$TINYGO_BIN" info "$TARGET")"

goroot="$(printf '%s\n' "$info" | sed -n 's/^cached GOROOT:[[:space:]]*//p')"
goos="$(printf '%s\n' "$info" | sed -n 's/^GOOS:[[:space:]]*//p')"
goarch="$(printf '%s\n' "$info" | sed -n 's/^GOARCH:[[:space:]]*//p')"
tags="$(printf '%s\n' "$info" | sed -n 's/^build tags:[[:space:]]*//p' | tr ' ' ',')"

if [ -z "$goroot" ] || [ -z "$goos" ] || [ -z "$goarch" ] || [ -z "$tags" ]; then
	echo "gopls-tinygo: could not parse 'tinygo info $TARGET'" >&2
	exit 1
fi

export GOROOT="$goroot"
export GOOS="$goos"
export GOARCH="$goarch"
export GOFLAGS="-tags=$tags"

exec "$GOPLS_BIN" "$@"
