#!/usr/bin/env bash
#
# setup.sh - install this repo into ~/.claude.
#
# Run:     bash setup.sh              link, then test and verify
#          bash setup.sh --dry-run    print every action, change nothing
#
# What it does, in order:
#   1. prune    removes symlinks left by the old layout (STALE_LINKS)
#   2. link     symlinks every home/* entry to ~/.claude/<name>
#   3. verify   runs test-hooks.sh then doctor.sh
#
# Nothing is copied. ~/.claude/<name> is a symlink back into this repo,
# so editing a file here takes effect in the next session with no
# reinstall. An existing real file is moved to <name>.bak-<timestamp>
# rather than overwritten.
#
# Restart Claude Code afterwards; settings.json and hooks load at start.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE="$REPO_ROOT/home"
TARGET="$HOME/.claude"
STAMP="$(date +%Y%m%d-%H%M%S)"

DRY_RUN=0

case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) printf 'usage: setup.sh [--dry-run]\n' >&2; exit 2 ;;
esac

STALE_LINKS=(standards modes templates profile orchestration docs)

say() { printf '%s\n' "$*"; }
run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    say "    would: $*"
  else
    "$@"
  fi
}

[ -d "$SOURCE" ] || { say "missing $SOURCE"; exit 1; }

say ""
say "installing agent config"
say "  from  $SOURCE"
say "  into  $TARGET"
say ""

run mkdir -p "$TARGET"

say "  pruning stale links from the previous layout"
for name in "${STALE_LINKS[@]}"; do
  path="$TARGET/$name"
  if [ -L "$path" ]; then
    say "    unlink $name"
    run rm -f "$path"
  elif [ -e "$path" ]; then
    say "    kept   $name (real directory, left alone)"
  fi
done

say ""
say "  linking"
for entry in "$SOURCE"/*; do
  [ -e "$entry" ] || continue
  name="$(basename "$entry")"
  dest="$TARGET/$name"

  if [ -L "$dest" ]; then
    run rm -f "$dest"
  elif [ -e "$dest" ]; then
    say "    backup $name -> $name.bak-$STAMP"
    run mv "$dest" "$dest.bak-$STAMP"
  fi

  say "    link   $name"
  run ln -s "$entry" "$dest"
done

if [ "$DRY_RUN" -eq 1 ]; then
  say ""
  say "dry run complete, nothing changed"
  exit 0
fi

say ""
bash "$REPO_ROOT/scripts/test-hooks.sh"
bash "$REPO_ROOT/scripts/doctor.sh"

say "restart Claude Code to pick up settings.json and hooks"
say ""
