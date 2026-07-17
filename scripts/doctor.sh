#!/usr/bin/env bash

set -euo pipefail

CLAUDE_DIR="$HOME/.claude"
LINKS=(agents standards modes templates settings.json hooks skills AGENTS.md CLAUDE.md)

broken=0

echo "doctor: checking $CLAUDE_DIR symlinks"

for name in "${LINKS[@]}"; do
  link="$CLAUDE_DIR/$name"

  if [ -L "$link" ]; then
    if [ -e "$link" ]; then
      echo "  OK        $name -> $(readlink "$link")"
    else
      echo "  DANGLING  $name -> $(readlink "$link")"
      broken=$((broken + 1))
    fi
  elif [ -e "$link" ]; then
    echo "  NOT-A-LINK $name (exists but is a real file/dir, not a symlink)"
    broken=$((broken + 1))
  else
    echo "  MISSING   $name (no symlink present, run setup.sh)"
    broken=$((broken + 1))
  fi
done

if [ "$broken" -eq 0 ]; then
  echo "doctor: all $CLAUDE_DIR links resolve."
  exit 0
fi

echo "doctor: $broken broken link(s) under $CLAUDE_DIR — run setup.sh to fix."
exit 1
