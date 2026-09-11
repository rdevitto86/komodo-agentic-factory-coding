#!/usr/bin/env bash
#
# test-install.sh - regression suite for scripts/install.py.
#
# Run:     bash scripts/test-install.sh
# Exit 0:  every case passed
# Exit 1:  at least one failed, with the offending reason printed
#
# Case IDs: I* install.py, S* setup.sh --ref pinning.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL="$REPO_ROOT/scripts/install.py"
SETUP="$REPO_ROOT/setup.sh"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

RESULTS="$WORKDIR/results"
: > "$RESULTS"

pass() { printf '  PASS  %s\n' "$1"; printf 'PASS\n' >> "$RESULTS"; }
fail() { printf '  FAIL  %s\n        %s\n' "$1" "$2"; printf 'FAIL\n' >> "$RESULTS"; }

REAL_PYTHON3="$(command -v python3)"

printf '\ninstall\n\n'

STUBBIN="$WORKDIR/stubbin-python-only"
mkdir -p "$STUBBIN"
ln -s "$REAL_PYTHON3" "$STUBBIN/python"

TARGET1="$WORKDIR/home1/.claude"
out="$(PATH="$STUBBIN" python "$INSTALL" --target "$TARGET1" 2>&1)"
rc=$?
if [ "$rc" -ne 0 ]; then
  fail "I1 resolves python when python3 is absent from PATH" "exit $rc: $out"
elif ! grep -q '"command": "python /' "$TARGET1/settings.json"; then
  fail "I1 resolves python when python3 is absent from PATH" "settings.json did not use the resolved 'python' interpreter"
else
  pass "I1 resolves python when python3 is absent from PATH"
fi

TARGET2="$WORKDIR/home2/.claude"
out="$("$REAL_PYTHON3" "$INSTALL" --target "$TARGET2" 2>&1)"
rc=$?
if [ "$rc" -ne 0 ]; then
  fail "I2 install with the real python3 exits 0" "exit $rc: $out"
elif grep -q '~' "$TARGET2/settings.json"; then
  fail "I2 no literal tilde in generated settings.json" "tilde found"
else
  pass "I2 no literal tilde in generated settings.json"
fi

TARGET3="$WORKDIR/home3/.claude"
out="$("$REAL_PYTHON3" "$INSTALL" --target "$TARGET3" --force-copy 2>&1)"
rc=$?
problem=""
[ "$rc" -eq 0 ] || problem="exit $rc: $out"
[ -z "$problem" ] && [ -L "$TARGET3/hooks" ] && problem="hooks was symlinked, not copied"
[ -z "$problem" ] && [ ! -d "$TARGET3/hooks" ] && problem="hooks directory missing after copy fallback"
[ -z "$problem" ] && [ ! -f "$TARGET3/hooks/git_guard.py" ] && problem="hooks/git_guard.py missing after copy fallback"
if [ -z "$problem" ] && [[ "$out" != *"Developer Mode"* ]]; then
  problem="missing Developer Mode guidance"
fi
if [ -z "$problem" ] && [[ "$out" != *"install.py"* ]]; then
  problem="missing re-sync command"
fi
if [ -n "$problem" ]; then
  fail "I3 forced symlink failure falls back to copy and prints guidance" "$problem"
else
  pass "I3 forced symlink failure falls back to copy and prints guidance"
fi

TARGET4="$WORKDIR/home4/.claude"
"$REAL_PYTHON3" "$INSTALL" --target "$TARGET4" > /dev/null 2>&1
if [ -L "$TARGET4/hooks" ] && [ -f "$TARGET4/hooks/git_guard.py" ]; then
  pass "I4 a normal install still symlinks (macOS/Linux path unaffected)"
else
  fail "I4 a normal install still symlinks (macOS/Linux path unaffected)" "hooks was not a live symlink"
fi

TARGET5="$WORKDIR/home5/.claude"
"$REAL_PYTHON3" "$INSTALL" --target "$TARGET5" > /dev/null 2>&1
problem=""
[ -L "$TARGET5/hooks" ] || problem="hooks was not a symlink"
[ -z "$problem" ] && [ ! -d "$TARGET5/hooks" ] && problem="symlinked hooks does not resolve to a directory"
[ -z "$problem" ] && [ ! -f "$TARGET5/hooks/git_guard.py" ] && problem="hooks/git_guard.py not reachable through symlinked directory"
[ -z "$problem" ] && ! grep -q 'target_is_directory' "$INSTALL" && problem="install.py no longer requests target_is_directory"
if [ -n "$problem" ]; then
  fail "I5 symlinked directory entry is created with directory semantics" "$problem"
else
  pass "I5 symlinked directory entry is created with directory semantics"
fi

TARGET6="$WORKDIR/home6\$(evil)/.claude"
mkdir -p "$(dirname "$TARGET6")"
out="$("$REAL_PYTHON3" "$INSTALL" --target "$TARGET6" 2>&1)"
rc=$?
if [ "$rc" -ne 0 ]; then
  fail "I6 hook_path with a shell metacharacter is single-quoted, not left bare" "exit $rc: $out"
elif ! grep -Eq "\"command\": \"[a-zA-Z0-9_]+ '[^']*home6\\\$\\(evil\\)[^']*git_guard\\.py'\"" "$TARGET6/settings.json"; then
  fail "I6 hook_path with a shell metacharacter is single-quoted, not left bare" "generated command did not wrap the metacharacter-bearing path in single quotes: $(grep '"command"' "$TARGET6/settings.json" | head -1)"
else
  pass "I6 hook_path with a shell metacharacter is single-quoted, not left bare"
fi

printf '\nsetup.sh --ref\n\n'

CLONE="$WORKDIR/clone"
git clone --quiet --local "$REPO_ROOT" "$CLONE"
TAG="$(git -C "$CLONE" tag | tail -1)"

TARGET_S4A="$WORKDIR/home-s4a/.claude"
TARGET_S4B="$WORKDIR/home-s4b/.claude"
baseline_out="$(AGENT_HOME="$TARGET_S4A" bash "$CLONE/setup.sh" --dry-run 2>&1 | sed "s#$TARGET_S4A#TARGET#g; s#$CLONE#REPO#g")"
current_out="$(AGENT_HOME="$TARGET_S4B" bash "$SETUP" --dry-run 2>&1 | sed "s#$TARGET_S4B#TARGET#g; s#$REPO_ROOT#REPO#g")"
if [[ "$baseline_out" == "$current_out" ]]; then
  pass "S4 plain --dry-run output is unchanged by the --ref addition"
else
  fail "S4 plain --dry-run output is unchanged by the --ref addition" "diff:
$(diff <(printf '%s\n' "$baseline_out") <(printf '%s\n' "$current_out"))"
fi

cp "$SETUP" "$CLONE/setup.sh"
git -C "$CLONE" -c user.email=test@example.com -c user.name=test commit --quiet -am "bring in the working copy's setup.sh for --ref testing"
CLONE_HEAD_BEFORE="$(git -C "$CLONE" rev-parse --abbrev-ref HEAD)"

# Only the setup that makes the tree bad differs between rejection cases.
assert_guard_rejected() {
  local label="$1" target="$2"
  shift 2
  local head_before out rc problem=""
  head_before="$(git -C "$CLONE" rev-parse --abbrev-ref HEAD)"
  out="$(AGENT_HOME="$target" bash "$CLONE/setup.sh" "$@" 2>&1)"
  rc=$?
  [ "$rc" -eq 0 ] && problem="exited 0: $out"
  [ -z "$problem" ] && [ "$head_before" != "$(git -C "$CLONE" rev-parse --abbrev-ref HEAD)" ] \
    && problem="HEAD moved"
  [ -z "$problem" ] && [ -e "$target" ] && problem="target was created before the guard failed"
  if [ -n "$problem" ]; then
    fail "$label" "$problem"
  else
    pass "$label"
  fi
}

if [ -z "$TAG" ]; then
  fail "S1 --ref TAG --dry-run previews the checkout and exits 0" "no tag found in the clone to pin to"
  fail "S3 a dirty tree exits non-zero before touching HEAD or the target" "no tag found in the clone to pin to"
else
  TAG_COMMIT="$(git -C "$CLONE" rev-parse --verify --quiet "$TAG^{commit}")"
  TARGET_S1="$WORKDIR/home-s1/.claude"
  out="$(AGENT_HOME="$TARGET_S1" bash "$CLONE/setup.sh" --ref "$TAG" --dry-run 2>&1)"
  rc=$?
  if [ "$rc" -ne 0 ]; then
    fail "S1 --ref TAG --dry-run previews the checkout and exits 0" "exit $rc: $out"
  elif [[ "$out" != *"would: git -C $CLONE checkout --detach $TAG_COMMIT"* ]]; then
    fail "S1 --ref TAG --dry-run previews the checkout and exits 0" "did not preview the detach: $out"
  else
    pass "S1 --ref TAG --dry-run previews the checkout and exits 0"
  fi
fi

assert_guard_rejected \
  "S2 an unknown ref exits non-zero before touching HEAD or the target" \
  "$WORKDIR/home-s2/.claude" --ref no-such-tag-xyz

assert_guard_rejected \
  "S5 an empty --ref is rejected rather than silently installing unpinned" \
  "$WORKDIR/home-s5/.claude" --ref=

assert_guard_rejected \
  "S6 a leading-dash ref is rejected before reaching git" \
  "$WORKDIR/home-s6/.claude" --ref --orphan=x

printf 'dirty\n' >> "$CLONE/README.md"
if [ -n "$TAG" ]; then
  assert_guard_rejected \
    "S3 a dirty tree exits non-zero before touching HEAD or the target" \
    "$WORKDIR/home-s3/.claude" --ref "$TAG"
fi

PASS="$(grep -c '^PASS$' "$RESULTS" || true)"
FAIL="$(grep -c '^FAIL$' "$RESULTS" || true)"
printf '\n  %d passed, %d failed\n\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
