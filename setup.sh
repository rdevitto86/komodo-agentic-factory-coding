#!/usr/bin/env bash
# setup.sh — Bootstrap komodo-claude symlinks into ~/.claude
# Run once after cloning: bash setup.sh

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$REPO_DIR/claude"
CLAUDE_DIR="$HOME/.claude"

echo "komodo-claude: setting up symlinks from $SOURCE_DIR → $CLAUDE_DIR"

mkdir -p "$CLAUDE_DIR"

for src in "$SOURCE_DIR"/*; do
  name="$(basename "$src")"
  dest="$CLAUDE_DIR/$name"

  if [ -L "$dest" ]; then
    current="$(readlink "$dest")"
    if [ "$current" = "$src" ]; then
      echo "  $name — symlink already correct, skipping"
    else
      echo "  $name — stale symlink (→ $current), refreshing"
      rm "$dest"
      ln -s "$src" "$dest"
    fi
  elif [ -e "$dest" ]; then
    echo "  $name — backing up existing to $name.bak"
    mv "$dest" "${dest}.bak"
    ln -s "$src" "$dest"
    echo "  $name — linked"
  else
    ln -s "$src" "$dest"
    echo "  $name — linked"
  fi
done

echo ""
echo "Done. Restart Claude Code to pick up the new settings."
