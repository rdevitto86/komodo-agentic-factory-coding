#!/usr/bin/env bash
#
# test-install.sh - regression suite for scripts/install.py.
#
# Run:     bash scripts/test-install.sh
# Exit 0:  every case passed
# Exit 1:  at least one failed, with the offending reason printed
#
# Case IDs: I* install.py.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL="$REPO_ROOT/scripts/install.py"
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

PASS="$(grep -c '^PASS$' "$RESULTS" || true)"
FAIL="$(grep -c '^FAIL$' "$RESULTS" || true)"
printf '\n  %d passed, %d failed\n\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
