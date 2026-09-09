#!/usr/bin/env python3
#
# verify_gate.py - Stop hook. Blocks the turn from ending while the
# repo's own verification command is failing.
#
# Opt-in per repo. It runs nothing unless the repo declares a command,
# checked in this order:
#
#   1. .claude/verify.sh   (executable)
#   2. make verify         (Makefile has a `verify:` target)
#   3. task verify         (Taskfile has a `verify:` task)
#   4. just verify         (justfile has a `verify:` recipe)
#
# No declaration means no gate, silently. A repo opts in by adding one.
#
# It also skips when `git status --porcelain` is empty, so a question-
# and-answer turn that changed nothing never pays for a test run.
#
# This hook FAILS OPEN. Every guard in this directory fails closed
# because a missed comment reaches disk; this one is the opposite. A
# verification gate that crashes must not be able to brick a session,
# so any internal error exits 0 and the turn ends normally.
#
# Claude Code stops honouring a Stop hook after 8 consecutive blocks,
# so a permanently red suite cannot trap the session either. This hook
# keeps its own approximate count of how many times in a row it has
# blocked this repo (a small file under the OS temp dir, keyed by repo
# root, cleared on any pass or skip) and appends a warning once that
# count nears the cutoff -- the fork gets a signal before it is force-
# ended, instead of the loop just going quiet.

import hashlib
import json
import os
import subprocess
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.git import repo_root

TIMEOUT_SECONDS = 300
MAX_OUTPUT_CHARS = 4000
STREAK_WARN_AT = 6


def run(args, cwd):
    return subprocess.run(
        args,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=TIMEOUT_SECONDS,
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


def tree_is_dirty(root):
    result = run(["git", "status", "--porcelain"], root)
    if result.returncode != 0:
        return False
    return bool(result.stdout.strip())


def streak_path(root):
    key = hashlib.sha256(root.encode("utf-8")).hexdigest()[:16]
    return os.path.join(tempfile.gettempdir(), "komodo-verify-gate-streak-%s" % key)


def read_streak(path):
    try:
        with open(path, encoding="utf-8") as handle:
            return int(handle.read().strip())
    except (OSError, ValueError):
        return 0


def write_streak(path, value):
    try:
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(str(value))
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


def block(label, result, streak):
    combined = tail((result.stdout or "") + (result.stderr or ""))
    lines = [
        "`%s` is failing. The task is not done." % label,
        "",
        combined,
        "",
        "Fix the cause, do not suppress the check. Re-run `%s`" % label,
        "and show the passing output as evidence.",
    ]
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

    if not tree_is_dirty(root):
        clear_streak(path)
        sys.exit(0)

    result = run(command, root)
    if result.returncode != 0:
        streak = read_streak(path) + 1
        write_streak(path, streak)
        block(label, result, streak)
    clear_streak(path)
    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
