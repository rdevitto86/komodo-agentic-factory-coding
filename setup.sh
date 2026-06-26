#!/usr/bin/env bash

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
LINK_DIRS=(agents standards modes templates)

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
  echo "komodo-claude: setting up symlinks from $REPO_DIR → $CLAUDE_DIR"
fi

do_run mkdir -p "$CLAUDE_DIR"

# Remove stale symlinks from superseded layouts. Skills/docs were folded into
# agent folders; "skills" and "docs" no longer exist as top-level link targets.
for stale in skills docs; do
  link="$CLAUDE_DIR/$stale"
  if [ -L "$link" ] && [[ "$(readlink "$link")" == "$REPO_DIR"/* ]]; then
    echo "  $stale — removing stale symlink from old layout"
    do_run rm "$link"
  fi
done

# Prune obsolete .bak files left by an earlier migration. A backup is obsolete
# once its live counterpart is already a correct symlink into this repo — the
# original file has been superseded. Runs before the link loop so a .bak created
# this run (the user's only copy of a pre-existing file) is never touched.
for name in "${LINK_DIRS[@]}" settings.json hooks AGENTS.md CLAUDE.md; do
  src="$REPO_DIR/$name"
  [ "$name" = "settings.json" ] && src="$REPO_DIR/platforms/claude/settings.json"
  [ "$name" = "hooks" ] && src="$REPO_DIR/platforms/claude/hooks"
  [ "$name" = "AGENTS.md" ] && src="$REPO_DIR/profile/AGENTS.md"
  [ "$name" = "CLAUDE.md" ] && src="$REPO_DIR/profile/CLAUDE.md"
  bak="$CLAUDE_DIR/$name.bak"
  link="$CLAUDE_DIR/$name"
  if [ -e "$bak" ] && [ -L "$link" ] && [ "$(readlink "$link")" = "$src" ]; then
    echo "  $name.bak — removing obsolete migration backup"
    do_run rm -rf "$bak"
  fi
done

# Warn about old-layout project snapshots (e.g. ~/.claude/<project>/CLAUDE.md).
# Claude Code never loads these; they only drift from the repo's real CLAUDE.md.
# Not auto-deleted — removing arbitrary dirs under ~/.claude is too destructive
# for a setup script, and a false positive could nuke real Claude Code data.
for dir in "$CLAUDE_DIR"/*/; do
  [ -d "$dir" ] || continue
  [ -e "${dir}CLAUDE.md" ] || continue
  name="$(basename "$dir")"
  # Known Claude Code dirs that legitimately exist — never flag these.
  case "$name" in
    agents|hooks|skills|standards|docs|projects|plugins|tasks|plans|cache|\
    backups|sessions|session-env|telemetry|shell-snapshots|file-history|\
    paste-cache|downloads|debug|ide) continue ;;
  esac
  echo "  WARNING: $name/ looks like an old project snapshot (has CLAUDE.md) — Claude Code does not load it; remove manually if stale: rm -rf \"$dir\""
done

link_one() {
  local src="$1" name="$2"
  local dest="$CLAUDE_DIR/$name"

  if [ -L "$dest" ]; then
    local current
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
}

for name in "${LINK_DIRS[@]}"; do
  link_one "$REPO_DIR/$name" "$name"
done

# Tool-adapter platform: settings.json and hooks live under platforms/claude/
# but Claude Code only loads them from ~/.claude/settings.json and ~/.claude/hooks/.
link_one "$REPO_DIR/platforms/claude/settings.json" "settings.json"
link_one "$REPO_DIR/platforms/claude/hooks" "hooks"
link_one "$REPO_DIR/profile/AGENTS.md" "AGENTS.md"
link_one "$REPO_DIR/profile/CLAUDE.md" "CLAUDE.md"

echo ""

if bash "$REPO_DIR/scripts/validate-refs.sh"; then
  echo "Reference validation passed."
else
  echo "WARNING: dead references found (see above). Fix them before the next session."
fi

if bash "$REPO_DIR/scripts/test-comment-rules.sh"; then
  echo "Comment rules test suite passed."
else
  echo "WARNING: comment rules test suite failed (see above). Fix them before the next session."
fi

if bash "$REPO_DIR/scripts/validate-bridge-roster.sh"; then
  echo "Bridge roster validation passed."
else
  echo "WARNING: bridge roster drift found (see above). Fix them before the next session."
fi

echo ""
if $DRY_RUN; then
  echo "Dry run complete. No changes were made."
else
  echo "Done. Restart Claude Code to pick up the new settings."
fi
