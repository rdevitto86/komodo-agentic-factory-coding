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

OLD_STRING=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.old_string // empty' 2>/dev/null)
NEW_CONTENT=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.new_string // .tool_input.content // empty' 2>/dev/null)
[ -z "$NEW_CONTENT" ] && exit 0

if printf '%s' "$PAYLOAD" | jq -e '.tool_input.old_string' >/dev/null 2>&1; then
  OLD_CONTENT="$OLD_STRING"
else
  GIT_ROOT_PRE=$(git -C "$(dirname "$FILE")" rev-parse --show-toplevel 2>/dev/null || true)
  REL_FILE_PRE=""
  [ -n "$GIT_ROOT_PRE" ] && REL_FILE_PRE=$(git -C "$GIT_ROOT_PRE" ls-files --full-name "$FILE" 2>/dev/null || true)
  if [ -n "$REL_FILE_PRE" ]; then
    OLD_CONTENT=$(git -C "$GIT_ROOT_PRE" show "HEAD:$REL_FILE_PRE" 2>/dev/null || true)
  else
    OLD_CONTENT=""
  fi
fi

TMP_OLD=$(mktemp)
TMP_NEW=$(mktemp)
trap 'rm -f "$TMP_OLD" "$TMP_NEW"' EXIT
printf '%s' "$OLD_CONTENT" > "$TMP_OLD"
printf '%s' "$NEW_CONTENT" > "$TMP_NEW"

MARKED_STREAM=$(diff -U 1000000 "$TMP_OLD" "$TMP_NEW" | awk '
  /^@@/ { print "R\t"; next }
  /^(\+\+\+|---)/ { next }
  /^\+/ { print "A\t" substr($0, 2); next }
  /^-/ { next }
  /^ / { print "C\t" substr($0, 2); next }
' || true)

[ -z "$MARKED_STREAM" ] && exit 0

VIOLATIONS=$(printf '%s\n' "$MARKED_STREAM" | FAMILY="$FAMILY" MARKED=1 awk -f "$RULES")

if [ -n "$VIOLATIONS" ]; then
  GIT_ROOT=$(git -C "$(dirname "$FILE")" rev-parse --show-toplevel 2>/dev/null || true)
  if [ -n "$GIT_ROOT" ]; then
    REL_FILE=$(git -C "$GIT_ROOT" ls-files --full-name "$FILE" 2>/dev/null || true)
    if [ -n "$REL_FILE" ]; then
      HEAD_CONTENT=$(git -C "$GIT_ROOT" show "HEAD:$REL_FILE" 2>/dev/null || true)
      if [ -n "$HEAD_CONTENT" ]; then
        VIOLATIONS=$(printf '%s\n' "$VIOLATIONS" | while IFS= read -r v; do
          LINE_TEXT=$(printf '%s\n' "$v" | sed -E 's/^[[:space:]]*\[[^]]+\][[:space:]]?//')
          if printf '%s\n' "$HEAD_CONTENT" | grep -qxF "$LINE_TEXT"; then
            continue
          fi
          printf '%s\n' "$v"
        done)
      fi
    fi
  fi
fi

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
