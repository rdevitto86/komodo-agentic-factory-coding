#!/usr/bin/env bash

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

dead=()

while IFS= read -r line; do
  src_file="${line%%:*}"
  ref="$(printf '%s' "$line" | grep -oE '~/.claude/(agents|standards|modes|templates)/[^`\)\ ]+' | head -1)"
  [ -z "$ref" ] && continue

  # Skip illustrative patterns: a literal "..." glob, a brace-expansion group
  # like {todo,memory}.md, or a <placeholder> segment — none name a real file.
  case "$ref" in
    *...*|*'{'*|*'<'*) continue ;;
  esac

  local_path="$REPO_DIR/${ref#\~/.claude/}"

  if [ ! -e "$local_path" ]; then
    dead+=("$src_file -> $ref (resolved: $local_path)")
  fi
done < <(grep -rn -E '~/\.claude/(agents|standards|modes|templates)/' "$REPO_DIR" --include='*.md' 2>/dev/null)

if [ ${#dead[@]} -eq 0 ]; then
  echo "validate-refs: all ~/.claude/{agents,standards,modes,templates}/ references are valid."
  exit 0
fi

echo "validate-refs: DEAD REFERENCES FOUND:"
for entry in "${dead[@]}"; do
  echo "  $entry"
done
exit 1
