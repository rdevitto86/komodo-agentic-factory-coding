#!/usr/bin/env bash

set -euo pipefail

if ! git rev-parse --git-dir > /dev/null 2>&1; then
  exit 0
fi

if ! git rev-parse HEAD > /dev/null 2>&1; then
  exit 0
fi

UNSTAGED=$(git diff --stat HEAD 2>/dev/null)
STAGED=$(git diff --cached --stat 2>/dev/null)

if [[ -z "$UNSTAGED" && -z "$STAGED" ]]; then
  exit 0
fi

echo ""
echo "┌─ Session changes ───────────────────────────────────┐"

if [[ -n "$STAGED" ]]; then
  echo "│ Staged"
  echo "$STAGED" | sed 's/^/│   /'
fi

if [[ -n "$UNSTAGED" ]]; then
  echo "│ Unstaged"
  echo "$UNSTAGED" | sed 's/^/│   /'
fi

echo "└─────────────────────────────────────────────────────┘"
