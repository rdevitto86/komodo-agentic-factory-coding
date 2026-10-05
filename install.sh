#!/bin/sh
# Downloads this machine's komodo release, verifies its SHA-256, and hands every other step to komodo install.
# Environment: KOMODO_VERSION, KOMODO_RELEASE_URL. From a checkout with Go, run go run ./cmd/komodo install instead.
set -eu
base="${KOMODO_RELEASE_URL:-https://github.com/rdevitto86/komodo-agentic-factory-coding/releases/download}/${KOMODO_VERSION:-v1.0.0-beta.5}"
name="komodo-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
dir="$HOME/.komodo/bin"
mkdir -p "$dir"
curl -fsSL "$base/$name" -o "$dir/$name.new"
want=$(curl -fsSL "$base/SHA256SUMS" | awk -v n="$name" '$2 == n || $2 == "*" n { print $1 }')
got=$( (sha256sum "$dir/$name.new" 2>/dev/null || shasum -a 256 "$dir/$name.new") | awk '{ print $1 }')
[ -n "$want" ] && [ "$want" = "$got" ] || { rm -f "$dir/$name.new"; echo "install: checksum mismatch for $name; nothing was installed" >&2; exit 1; }
chmod +x "$dir/$name.new" && mv "$dir/$name.new" "$dir/$name"
exec "$dir/$name" install
