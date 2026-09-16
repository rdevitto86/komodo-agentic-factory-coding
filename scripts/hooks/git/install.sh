#!/usr/bin/env bash
#
# install.sh - point a repo's git hooks at this directory.
#
# Run:  bash install.sh /path/to/repo         one repo
#       bash install.sh ~/komodo/*/*          many repos, skips non-repos
#       bash install.sh --status /path/...    report only, change nothing
#
# It sets core.hooksPath, which is local git config for that repo and is
# never committed. Nothing is copied, so editing a hook here takes effect
# in every installed repo on the next commit.
#
# core.hooksPath replaces .git/hooks wholesale. An existing hook in
# .git/hooks stops running; the script reports any it finds so nothing
# disappears silently.
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# git rewrites a path argument into its native form before storing it, so hold the form it stores
if command -v cygpath >/dev/null 2>&1; then
  HOOK_DIR="$(cygpath -m "$HOOK_DIR")"
fi
STATUS_ONLY=0

if [ "${1:-}" = "--status" ]; then
  STATUS_ONLY=1
  shift
fi

[ "$#" -gt 0 ] || { printf 'usage: install.sh [--status] <repo> [repo...]\n' >&2; exit 2; }

for hook in "$HOOK_DIR"/pre-commit "$HOOK_DIR"/pre-push; do
  [ -x "$hook" ] || { printf 'install.sh: %s is not executable\n' "$hook" >&2; exit 1; }
done

INSTALLED=0
SKIPPED=0

for repo in "$@"; do
  [ -d "$repo" ] || continue
  git -C "$repo" rev-parse --git-dir >/dev/null 2>&1 || { SKIPPED=$((SKIPPED + 1)); continue; }

  name="$(basename "$repo")"
  current="$(git -C "$repo" config --local --get core.hooksPath || true)"

  # A relative core.hooksPath is resolved against the repo root, as git itself does.
  state="unset"
  if [ -n "$current" ]; then
    if [ "$current" = "$HOOK_DIR" ]; then
      state="current"
    else
      case "$current" in
        /* | [A-Za-z]:/* | [A-Za-z]:\\*) current_abs="$current" ;;
        *) current_abs="$repo/$current" ;;
      esac
      if [ -d "$current_abs" ]; then
        state="different"
      else
        state="stale"
      fi
    fi
  fi

  if [ "$STATUS_ONLY" -eq 1 ]; then
    if [ "$state" = "stale" ]; then
      printf '%-45s stale: %s\n' "$name" "$current"
    else
      printf '%-45s %s\n' "$name" "${current:-<unset>}"
    fi
    continue
  fi

  if [ "$state" = "current" ]; then
    printf '%-45s already installed\n' "$name"
    continue
  fi

  legacy="$(git -C "$repo" rev-parse --git-path hooks)"
  if [ -d "$repo/$legacy" ]; then
    orphans="$(find "$repo/$legacy" -maxdepth 1 -type f ! -name '*.sample' -perm -u+x 2>/dev/null)"
    if [ -n "$orphans" ]; then
      printf '%-45s note: these .git/hooks files stop running:\n' "$name"
      printf '  %s\n' $orphans
    fi
  fi

  if [ "$state" = "stale" ]; then
    printf '%-45s note: hooks had not been running (core.hooksPath pointed at missing %s)\n' "$name" "$current"
  fi

  git -C "$repo" config --local core.hooksPath "$HOOK_DIR"
  printf '%-45s installed\n' "$name"
  INSTALLED=$((INSTALLED + 1))
done

[ "$STATUS_ONLY" -eq 1 ] && exit 0

printf '\n%d installed, %d skipped (not a git repo)\n' "$INSTALLED" "$SKIPPED"
printf 'Uninstall: git -C <repo> config --local --unset core.hooksPath\n'
