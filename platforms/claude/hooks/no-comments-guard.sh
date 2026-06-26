#!/usr/bin/env bash
set -euo pipefail

PAYLOAD=$(cat)
FILE=$(echo "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
[ -z "$FILE" ] && exit 0

case "$FILE" in
  *.go|*.ts|*.tsx|*.js|*.jsx|*.mjs|*.cjs|*.svelte|*.c|*.h|*.cc|*.cpp|*.hpp|*.rs|*.java|*.kt|*.scala|*.swift)
    FAMILY="cfamily" ;;
  *.py|*.sh|*.bash|*.zsh|*.rb)
    FAMILY="hash" ;;
  *)
    exit 0 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
RULES="$SCRIPT_DIR/../../../scripts/lib/comment-rules.awk"
[ -f "$RULES" ] || exit 0

ADDED=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.new_string // .tool_input.content // empty' 2>/dev/null)
[ -z "$ADDED" ] && exit 0

VIOLATIONS=$(printf '%s\n' "$ADDED" | FAMILY="$FAMILY" awk -f "$RULES")

if [ -n "$VIOLATIONS" ]; then
  {
    echo "BLOCKED: this edit violates standards/comments.md."
    echo ""
    echo "In $(basename "$FILE"):"
    echo "$VIOLATIONS"
    echo ""
    echo "Zero comments, full stop — no function/method docs, no declaration"
    echo "comments, no inline or trailing notes, no file/package headers."
    echo "Only machine directives and test-file section banners are exempt."
    echo "Remove the line(s) and retry."
  } >&2
  exit 2
fi
exit 0
