#!/usr/bin/env bash
set -euo pipefail

PAYLOAD=$(cat)
COMMAND=$(echo "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null)
[ -z "$COMMAND" ] && exit 0

RULES=(
  'git commit:::\bgit[[:space:]]+commit([[:space:]]|$)'
  'git branch:::\bgit[[:space:]]+branch([[:space:]]|$)'
  'git checkout -b:::\bgit[[:space:]]+checkout[[:space:]]+-b([[:space:]]|$)'
  'git switch -c:::\bgit[[:space:]]+switch[[:space:]]+-c([[:space:]]|$)'
  'git reset --hard:::\bgit[[:space:]]+reset[[:space:]]+--hard([[:space:]]|$)'
  'git clean -f:::\bgit[[:space:]]+clean[[:space:]]+-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|$)'
  'git push --force:::\bgit[[:space:]]+push\b.*(--force-with-lease|--force([[:space:]]|$)|[[:space:]]-f([[:space:]]|$))'
)

for RULE in "${RULES[@]}"; do
  LABEL="${RULE%%:::*}"
  PATTERN="${RULE#*:::}"
  if printf '%s' "$COMMAND" | grep -qoE "$PATTERN"; then
    echo "BLOCKED: $LABEL — only the user commits, branches, and merges (CLAUDE.md \"Hard rules\"). Run this yourself if intended." >&2
    exit 2
  fi
done

exit 0
