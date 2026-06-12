#!/usr/bin/env bash

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRIDGE_AGENTS_GO="$HOME/.komodo/bridge/agents.go"

if [ ! -f "$BRIDGE_AGENTS_GO" ]; then
  echo "validate-bridge-roster: ~/.komodo/bridge not present, skipping."
  exit 0
fi

missing=()

while IFS= read -r name; do
  if [ ! -f "$REPO_DIR/agents/$name/agent.md" ]; then
    missing+=("$name -> agents/$name/agent.md")
  fi
done < <(grep -oE 'loadSystemPrompt\("[a-z-]+"\)' "$BRIDGE_AGENTS_GO" | sed -E 's/loadSystemPrompt\("([a-z-]+)"\)/\1/')

if [ ${#missing[@]} -eq 0 ]; then
  echo "validate-bridge-roster: all bridge-referenced agents have agent.md"
  exit 0
fi

echo "validate-bridge-roster: MISMATCHES FOUND:"
for entry in "${missing[@]}"; do
  echo "  $entry"
done
exit 1
