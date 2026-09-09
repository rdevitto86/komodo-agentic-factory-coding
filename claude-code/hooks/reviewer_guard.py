#!/usr/bin/env python3
#
# reviewer_guard.py - PreToolUse hook for Edit/Write. Makes reviewer.md's
# "never edit any file other than BACKLOG.md" boundary mechanical instead
# of prose-only: an Edit or Write from the reviewer agent (the fork
# target for assess-bugs/assess-security/assess-simplify) is denied
# unless its target is BACKLOG.md or docs/BACKLOG.md.
#
# The payload's agent_type field names the running agent (present
# whenever the call originates inside a subagent); every other agent's
# Edit/Write is untouched.
#
# This hook FAILS OPEN, unlike git_guard.py. It is narrower and lower-
# stakes than git_guard's git-command interception — a crash here must
# never be able to block a legitimate reviewer edit to BACKLOG.md, so
# any internal error exits 0.

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.git import repo_root

REVIEWER_AGENT = "reviewer"
ALLOWED_RELATIVE_PATHS = ("BACKLOG.md", "docs/BACKLOG.md")


def resolve(path, base):
    absolute = path if os.path.isabs(path) else os.path.join(base, path)
    return os.path.realpath(absolute)


def is_symlinked(path, base):
    # walks every segment up to `base` since realpath dereferences ancestors too, stopping short of an OS symlink above it
    absolute = path if os.path.isabs(path) else os.path.join(base, path)
    boundary = os.path.normpath(base)
    current = os.path.normpath(absolute)
    while True:
        if os.path.islink(current):
            return True
        if current == boundary:
            return False
        parent = os.path.dirname(current)
        if parent == current:
            return False
        current = parent


def is_allowed_path(file_path, cwd):
    base = cwd or os.getcwd()
    root = repo_root(base)
    # can't confirm the repo root -- fail open rather than guess
    if root is None:
        return True
    root = os.path.realpath(root)
    # a symlinked target, BACKLOG.md, or ancestor dir collapses both realpath sides onto one inode -- checked first
    if is_symlinked(file_path, base) or any(is_symlinked(name, root) for name in ALLOWED_RELATIVE_PATHS):
        return False
    target = resolve(file_path, base)
    allowed = {resolve(name, root) for name in ALLOWED_RELATIVE_PATHS}
    return target in allowed


def respond_deny(reason):
    payload = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }
    sys.stdout.write(json.dumps(payload))
    sys.exit(0)


def main():
    payload = json.loads(sys.stdin.read())

    if payload.get("tool_name") not in ("Edit", "Write"):
        sys.exit(0)

    if payload.get("agent_type") != REVIEWER_AGENT:
        sys.exit(0)

    file_path = (payload.get("tool_input") or {}).get("file_path", "")
    if not file_path:
        sys.exit(0)

    if is_allowed_path(file_path, payload.get("cwd")):
        sys.exit(0)

    respond_deny(
        "BLOCKED. Nothing ran.\n\n"
        "    the reviewer agent edits only BACKLOG.md (or docs/BACKLOG.md) --\n"
        "    it files findings, it does not fix them. %s is out of scope."
        % file_path
    )


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
