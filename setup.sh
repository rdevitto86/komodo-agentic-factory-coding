#!/usr/bin/env bash
# setup.sh — Bootstrap komodo-claude symlinks into ~/.claude
# Run once after cloning: bash setup.sh
# Flags: --dry-run  Print what would be done without making changes.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$REPO_DIR/claude"
CLAUDE_DIR="$HOME/.claude"

DRY_RUN=false
for arg in "$@"; do
  [ "$arg" = "--dry-run" ] && DRY_RUN=true
done

do_run() {
  if $DRY_RUN; then
    echo "    [dry-run] would run: $*"
  else
    "$@"
  fi
}

if $DRY_RUN; then
  echo "komodo-claude: DRY RUN — no changes will be made"
else
  echo "komodo-claude: setting up symlinks from $SOURCE_DIR → $CLAUDE_DIR"
fi

do_run mkdir -p "$CLAUDE_DIR"

# Remove stale symlinks from the pre-consolidation layout. Standards, skills, and
# docs are no longer top-level — they're encapsulated inside each agent's folder.
for stale in standards skills docs; do
  link="$CLAUDE_DIR/$stale"
  if [ -L "$link" ] && [[ "$(readlink "$link")" == "$REPO_DIR"/* ]]; then
    echo "  $stale — removing stale symlink from old layout"
    do_run rm "$link"
  fi
done

for src in "$SOURCE_DIR"/*; do
  name="$(basename "$src")"
  dest="$CLAUDE_DIR/$name"

  if [ -L "$dest" ]; then
    current="$(readlink "$dest")"
    if [ "$current" = "$src" ]; then
      # Already points at the correct source — no action needed, even on re-run.
      echo "  $name — symlink already correct, skipping"
    else
      echo "  $name — stale symlink (→ $current), refreshing"
      do_run rm "$dest"
      do_run ln -s "$src" "$dest"
    fi
  elif [ -e "$dest" ]; then
    echo "  $name — backing up existing to $name.bak"
    do_run mv "$dest" "${dest}.bak"
    do_run ln -s "$src" "$dest"
    echo "  $name — linked"
  else
    do_run ln -s "$src" "$dest"
    echo "  $name — linked"
  fi
done

echo ""

if bash "$REPO_DIR/scripts/validate-refs.sh"; then
  echo "Reference validation passed."
else
  echo "WARNING: dead references found (see above). Fix them before the next session."
fi

echo ""
if $DRY_RUN; then
  echo "Dry run complete. No changes were made."
else
  echo "Done. Restart Claude Code to pick up the new settings."
fi
