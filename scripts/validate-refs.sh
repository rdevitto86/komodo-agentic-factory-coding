#!/usr/bin/env bash
# validate-refs.sh — Check that every ~/.claude/agents/... reference in claude/**/*.md
# resolves to an actual file in this repo (mapping ~/.claude/ → claude/).

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLAUDE_DIR="$REPO_DIR/claude"

dead=()

while IFS= read -r line; do
  # Extract the file containing the reference and the ref itself
  src_file="${line%%:*}"
  ref="$(printf '%s' "$line" | grep -oE '~/.claude/[^`\)\ ]+' | head -1)"
  [ -z "$ref" ] && continue

  # Map ~/.claude/ → claude/
  local_path="$REPO_DIR/${ref/#\~\/.claude\//claude/}"

  if [ ! -e "$local_path" ]; then
    dead+=("$src_file -> $ref (resolved: $local_path)")
  fi
done < <(grep -rn '~/.claude/agents/' "$CLAUDE_DIR" --include='*.md' 2>/dev/null)

if [ ${#dead[@]} -eq 0 ]; then
  echo "validate-refs: all ~/.claude/agents/ references are valid."
  exit 0
fi

echo "validate-refs: DEAD REFERENCES FOUND:"
for entry in "${dead[@]}"; do
  echo "  $entry"
done
exit 1
