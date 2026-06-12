#!/usr/bin/env bash

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RULES="$REPO_DIR/scripts/lib/comment-rules.awk"
TESTDATA="$REPO_DIR/scripts/lib/testdata"

family_for() {
  case "$1" in
    *.go|*.ts|*.tsx|*.js|*.jsx|*.mjs|*.cjs|*.svelte|*.c|*.h|*.cc|*.cpp|*.hpp|*.rs|*.java|*.kt|*.scala|*.swift)
      echo "cfamily" ;;
    *.py|*.sh|*.bash|*.zsh|*.rb)
      echo "hash" ;;
    *)
      echo "" ;;
  esac
}

FAIL=0

for fixture in "$TESTDATA"/*; do
  case "$fixture" in *.expected) continue ;; esac
  [ -f "$fixture" ] || continue

  expected="$fixture.expected"
  [ -f "$expected" ] || { echo "MISSING .expected for $(basename "$fixture")"; FAIL=1; continue; }

  family="$(family_for "$fixture")"
  [ -z "$family" ] && { echo "SKIP unknown family for $(basename "$fixture")"; continue; }

  actual="$(FAMILY="$family" awk -f "$RULES" "$fixture" | sed -E 's/^[[:space:]]*\[([^]]+)\].*/\1/')"
  want="$(cat "$expected")"

  if [ "$actual" = "$want" ]; then
    echo "PASS $(basename "$fixture")"
  else
    echo "FAIL $(basename "$fixture")"
    echo "  expected: $(echo "$want" | tr '\n' '|')"
    echo "  actual:   $(echo "$actual" | tr '\n' '|')"
    FAIL=1
  fi
done

exit "$FAIL"
