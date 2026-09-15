#!/usr/bin/env python3
#
# verify_gate.py - Stop hook. Blocks the turn from ending while the
# repo's own verification command is failing.
#
# Opt-in per repo. Checked in order: .claude/verify.sh (executable),
# then a `verify:` target in Makefile / Taskfile / justfile. None of
# those declared means no gate, silently.
#
# Skips a clean working tree, and skips -- without running the check
# at all -- a tree whose only dirty paths are records-only files
# (BACKLOG.md, docs/BACKLOG.md, CHANGELOG.md, README.md).
#
# Skips the full suite -- without running it -- while the repo carries a
# band-gate deferral marker at .claude/state/band-gate: one line holding
# an integer epoch-seconds deadline, written by the orchestrator when a
# band of more than one task starts, removed before the band's single
# gate run at P2.2. This suppression fails CLOSED toward running the
# gate: an absent, unreadable, malformed, expired, or implausibly distant
# (past MAX_DEFER_SECONDS from now) marker runs the suite as usual.
#
# The verify command is bounded by KOMODO_VERIFY_TIMEOUT seconds
# (default 300, an invalid value falls back to 300); a timeout is a
# deliberate block naming the limit, not a silent pass-through. The
# git-status probe backing the records-only skip has its own fixed
# 5s timeout; a probe timeout falls through to a normal verify run.
#
# Otherwise this hook FAILS OPEN: an internal error outside the cases
# above exits 0, because a broken gate must not be able to brick a
# session.
#
# Claude Code stops honouring a Stop hook after 8 consecutive blocks,
# so this hook tracks its own approximate streak of consecutive blocks
# against a repo (a small file under the OS temp dir, keyed by repo
# root, cleared on any pass or skip) and warns once that streak nears
# the cutoff. It also hashes each failure's combined output: three
# consecutive identical hashes stop the fork itself (exit 0 with a
# systemMessage, no block) rather than grinding toward that 8-block
# cutoff on a failure that isn't changing.

import hashlib
import json
import os
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.git import repo_root

DEFAULT_TIMEOUT_SECONDS = 300
MAX_OUTPUT_CHARS = 4000
STREAK_WARN_AT = 6
IDENTICAL_STOP_AT = 3
RECORDS_ONLY_PATHS = frozenset(
    ("BACKLOG.md", "docs/BACKLOG.md", "CHANGELOG.md", "README.md")
)
BAND_MARKER_PARTS = (".claude", "state", "band-gate")
MAX_DEFER_SECONDS = 4 * 60 * 60


def timeout_seconds():
    raw = os.environ.get("KOMODO_VERIFY_TIMEOUT")
    if raw is None:
        return DEFAULT_TIMEOUT_SECONDS
    try:
        value = int(raw.strip())
    except ValueError:
        return DEFAULT_TIMEOUT_SECONDS
    return value if value > 0 else DEFAULT_TIMEOUT_SECONDS


def run(args, cwd, timeout):
    return subprocess.run(
        args,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=timeout,
    )


def target_exists(path, pattern):
    if not os.path.isfile(path):
        return False
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            for line in handle:
                if line.startswith(pattern):
                    return True
    except OSError:
        return False
    return False


def discover(root):
    script = os.path.join(root, ".claude", "verify.sh")
    if os.path.isfile(script) and os.access(script, os.X_OK):
        return [script], ".claude/verify.sh"
    if target_exists(os.path.join(root, "Makefile"), "verify:"):
        return ["make", "verify"], "make verify"
    for name in ("Taskfile.yml", "Taskfile.yaml"):
        if target_exists(os.path.join(root, name), "  verify:"):
            return ["task", "verify"], "task verify"
    if target_exists(os.path.join(root, "justfile"), "verify:"):
        return ["just", "verify"], "just verify"
    return None, None


def dirty_paths(root):
    # A timed-out probe is inconclusive, not clean: fall through to a normal verify run.
    try:
        result = run(["git", "status", "--porcelain"], root, 5)
    except subprocess.TimeoutExpired:
        return None
    if result.returncode != 0:
        return []
    paths = []
    for line in result.stdout.splitlines():
        if not line:
            continue
        rest = line[3:] if len(line) > 3 else line.strip()
        if " -> " in rest:
            rest = rest.split(" -> ", 1)[1]
        rest = rest.strip()
        if rest:
            paths.append(rest)
    return paths


def is_records_only(paths):
    return bool(paths) and all(p in RECORDS_ONLY_PATHS for p in paths)


def band_gate_deferred(root):
    # Absent, malformed, expired, implausibly distant: every one runs the gate.
    try:
        with open(
            os.path.join(root, *BAND_MARKER_PARTS), encoding="utf-8"
        ) as handle:
            lines = handle.read(256).splitlines()
        if not lines:
            return False
        deadline = int(lines[0].strip())
    except Exception:
        return False
    now = time.time()
    return now < deadline <= now + MAX_DEFER_SECONDS


def streak_path(root):
    key = hashlib.sha256(root.encode("utf-8")).hexdigest()[:16]
    return os.path.join(tempfile.gettempdir(), "komodo-verify-gate-streak-%s" % key)


def read_state(path):
    try:
        with open(path, encoding="utf-8") as handle:
            lines = handle.read().splitlines()
    except OSError:
        return 0, "", 0
    streak = 0
    prev_hash = ""
    identical = 0
    if len(lines) >= 1:
        try:
            streak = int(lines[0].strip())
        except ValueError:
            streak = 0
    if len(lines) >= 2:
        prev_hash = lines[1].strip()
    if len(lines) >= 3:
        try:
            identical = int(lines[2].strip())
        except ValueError:
            identical = 0
    return streak, prev_hash, identical


def write_state(path, streak, hash_hex, identical):
    try:
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("%d\n%s\n%d\n" % (streak, hash_hex, identical))
    except OSError:
        pass


def clear_streak(path):
    try:
        os.remove(path)
    except OSError:
        pass


def tail(text):
    text = text.strip()
    if len(text) <= MAX_OUTPUT_CHARS:
        return text
    return "...(truncated)...\n" + text[-MAX_OUTPUT_CHARS:]


def stop_fork(path):
    clear_streak(path)
    message = (
        "verify_gate: identical failure three times -- returning so the "
        "fork can report BLOCKED"
    )
    sys.stdout.write(json.dumps({"systemMessage": message}))
    sys.exit(0)


def defer_to_band(path):
    clear_streak(path)
    message = (
        "verify_gate: full suite deferred to the band gate at P2.2 -- this "
        "fork's own `Done when` commands stay its proof"
    )
    sys.stdout.write(json.dumps({"systemMessage": message}))
    sys.exit(0)


def record_failure(path, hash_source):
    prev_streak, prev_hash, prev_identical = read_state(path)
    hash_hex = hashlib.sha256(hash_source.encode("utf-8")).hexdigest()
    new_streak = prev_streak + 1
    if prev_hash and hash_hex == prev_hash:
        identical = prev_identical + 1
    else:
        identical = 1
    return new_streak, identical, hash_hex, (identical >= 2)


def emit_block(path, lines, hash_source):
    streak, identical, hash_hex, repeats = record_failure(path, hash_source)
    if identical >= IDENTICAL_STOP_AT:
        stop_fork(path)
        return
    write_state(path, streak, hash_hex, identical)
    if repeats:
        lines += ["", "Same failure as the previous block."]
    if streak >= STREAK_WARN_AT:
        lines += [
            "",
            "This is block %d in a row on this tree. Claude Code stops "
            "honoring a Stop hook after 8 consecutive blocks -- past that "
            "this fork force-ends with no further warning. If the same "
            "failure persists, stop and escalate instead of retrying." % streak,
        ]
    reason = "\n".join(lines)
    sys.stdout.write(json.dumps({"decision": "block", "reason": reason}))
    sys.exit(0)


def block(path, label, result):
    combined = tail((result.stdout or "") + (result.stderr or ""))
    lines = [
        "`%s` is failing. The task is not done." % label,
        "",
        combined,
        "",
        "Fix the cause, do not suppress the check. Re-run `%s`" % label,
        "and show the passing output as evidence.",
    ]
    emit_block(path, lines, combined)


def block_timeout(path, label, limit):
    reason = (
        "`%s` exceeded %d s -- the task is not done; a suite this slow "
        "needs a narrower .claude/verify.sh" % (label, limit)
    )
    emit_block(path, [reason], reason)


def main():
    payload = json.loads(sys.stdin.read())

    # already continuing from a previous block; let the turn end
    if payload.get("stop_hook_active"):
        sys.exit(0)

    start = payload.get("cwd") or os.getcwd()
    root = repo_root(start)
    if root is None:
        sys.exit(0)

    path = streak_path(root)

    command, label = discover(root)
    if command is None:
        clear_streak(path)
        sys.exit(0)

    if band_gate_deferred(root):
        defer_to_band(path)
        return

    paths = dirty_paths(root)
    if paths is not None:
        if not paths:
            clear_streak(path)
            sys.exit(0)

        if is_records_only(paths):
            clear_streak(path)
            sys.exit(0)

    limit = timeout_seconds()
    try:
        result = run(command, root, limit)
    except subprocess.TimeoutExpired:
        block_timeout(path, label, limit)
        return

    if result.returncode != 0:
        block(path, label, result)
        return
    clear_streak(path)
    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
