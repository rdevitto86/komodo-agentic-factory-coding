#!/usr/bin/env bash
#
# setup.sh - install this repo into ~/.claude.
#
# Run:     bash setup.sh                link, then test and verify
#          bash setup.sh --dry-run      print every action, change nothing
#          bash setup.sh --skip-verify  link only, skip step 4 (a caller
#                                       about to run its own `make verify`
#                                       right after, e.g. CI, skips the
#                                       redundant double run)
#          bash setup.sh --ref TAG      detach the clone at TAG before
#                                       linking, so the symlinks pin to a
#                                       release instead of the working
#                                       tree's current commit (requires a
#                                       resolvable ref and a clean tree)
#
# What it does, in order:
#   1. prune    removes symlinks left by the old layout (STALE_LINKS)
#   2. link     symlinks every claude-code/* entry to ~/.claude/<name>
#   3. overlay  copies settings.local.json.tmpl -> settings.local.json
#               and CLAUDE.local.md.tmpl -> CLAUDE.local.md, each once,
#               only if the personal overlay does not exist yet
#   4. verify   runs test_hooks.py then validate.py (skippable, see above)
#
# Everything else is a symlink, not a copy. ~/.claude/<name> is a
# symlink back into this repo, so editing a file here takes effect in
# the next session with no reinstall. An existing real file is moved
# to <name>.bak-<timestamp> rather than overwritten. settings.local.json
# and CLAUDE.local.md are the exceptions: each is a real, untracked,
# gitignored-by-convention copy so personal prefs (model, theme,
# effortLevel, the single-user/ADHD conversation rules, ...) never land
# in this repo. Claude Code's own settings precedence merges the former
# over settings.json at runtime, and CLAUDE.md's `@CLAUDE.local.md`
# import pulls in the latter.
#
# Restart Claude Code afterwards; settings.json and hooks load at start.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE="$REPO_ROOT/claude-code"
TARGET="${AGENT_HOME:-$HOME/.claude}"
STAMP="$(date +%Y%m%d-%H%M%S)"

DRY_RUN=0
SKIP_VERIFY=0
REF=""
REF_SET=0
REF_COMMIT=""

usage() {
  printf 'usage: setup.sh [--dry-run] [--skip-verify] [--target DIR] [--ref TAG]\n' >&2
  printf '  --skip-verify  link and overlay only, skip the trailing test/validate run\n' >&2
  printf '  --target DIR   install into DIR instead of %s\n' "$TARGET" >&2
  printf '  --ref TAG      detach the clone at TAG before linking (ref must\n' >&2
  printf '                 resolve, working tree must be clean)\n' >&2
  printf '  AGENT_HOME=DIR does the same as an environment variable\n' >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    --target) [ "$#" -ge 2 ] || { usage; exit 2; }; TARGET="$2"; shift 2 ;;
    --target=*) TARGET="${1#--target=}"; shift ;;
    --ref) [ "$#" -ge 2 ] || { usage; exit 2; }; REF="$2"; REF_SET=1; shift 2 ;;
    --ref=*) REF="${1#--ref=}"; REF_SET=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[ -n "$TARGET" ] || { printf 'setup.sh: empty target\n' >&2; exit 2; }

# An empty --ref would fall through the gates below and install unpinned.
[ "$REF_SET" -eq 0 ] || [ -n "$REF" ] \
  || { printf 'setup.sh: --ref requires a non-empty ref\n' >&2; exit 2; }
export AGENT_HOME="$TARGET"

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

# Floor: 3.7, set by subprocess.run(capture_output=...) in claude-code/hooks/*.py (added in 3.7).
PYTHON_FLOOR="3.7"

say ""
say "checking python3"
if ! command -v python3 >/dev/null 2>&1; then
  say "  python3 not found on PATH (every hook in claude-code/hooks/ and"
  say "  scripts/hooks/git/ shells out to it; floor: $PYTHON_FLOOR)"
  exit 1
fi

PYTHON_VERSION="$(python3 -c 'import sys; print("%d.%d.%d" % sys.version_info[:3])')" \
  || { say "  could not determine python3 version"; exit 1; }
say "  found python3 $PYTHON_VERSION (floor: $PYTHON_FLOOR)"

python3 -c "import sys; sys.exit(0 if sys.version_info >= (3, 7) else 1)" \
  || { say "  python3 $PYTHON_VERSION is below the required floor $PYTHON_FLOOR"; exit 1; }

if [ -n "$REF" ]; then
  say ""
  say "checking --ref $REF"
  if ! git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    say "  $REPO_ROOT is not a git repository, cannot pin to a ref"
    exit 1
  fi
  # `--` can't disambiguate here: for checkout it starts a pathspec, not a ref.
  case "$REF" in
    -*) say "  ref '$REF' starts with '-', refusing to pass it to git"; exit 2 ;;
  esac
  REF_COMMIT="$(git -C "$REPO_ROOT" rev-parse --verify --quiet "$REF^{commit}")" || REF_COMMIT=""
  if [ -z "$REF_COMMIT" ]; then
    say "  ref '$REF' does not resolve to a commit"
    exit 1
  fi
  if [ -n "$(git -C "$REPO_ROOT" status --porcelain)" ]; then
    say "  working tree is dirty, refusing to pin to a ref (commit or stash first)"
    exit 1
  fi
  say "  ref resolves, working tree is clean"
fi

say ""
say "installing agent config"
say "  from  $SOURCE"
say "  into  $TARGET"
say ""

if [ -n "$REF" ]; then
  say "  detaching at $REF ($REF_COMMIT)"
  run git -C "$REPO_ROOT" checkout --detach "$REF_COMMIT"
  say ""
fi

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

say ""
say "  personal overlay"
for name in settings.local.json CLAUDE.local.md; do
  local_dest="$TARGET/$name"
  if [ -e "$local_dest" ]; then
    say "    kept   $name (already present)"
  else
    say "    copy   $name.tmpl -> $name"
    run cp "$SOURCE/$name.tmpl" "$local_dest"
  fi
done

if [ "$DRY_RUN" -eq 1 ]; then
  say ""
  say "dry run complete, nothing changed"
  exit 0
fi

if [ "$SKIP_VERIFY" -eq 0 ]; then
  say ""
  python3 "$REPO_ROOT/scripts/test_hooks.py"
  python3 "$REPO_ROOT/scripts/validate.py"
fi

if [ -n "$REF" ]; then
  VERSION="$REF (pinned)"
else
  VERSION="$(cd "$REPO_ROOT" && git describe --tags --abbrev=0 2>/dev/null)" \
    || VERSION="$(cd "$REPO_ROOT" && git rev-parse --short HEAD 2>/dev/null)" \
    || VERSION="unknown"
fi
say "installed $VERSION"
say "restart Claude Code to pick up settings.json and hooks"
say ""
