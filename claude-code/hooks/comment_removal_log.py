#!/usr/bin/env python3
#
# comment_removal_log.py - PostToolUse hook for Edit/Write/MultiEdit/
# NotebookEdit. Records approved comment removals for a later pass.
#
# This hook FAILS OPEN, like auto_format.py.

import json
import os
import subprocess
import sys
import time

from lib.comment_rules import FAMILY_SYNTAX, normalize, resolve_family, scan_comments

WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")
LOG_RELATIVE_PATH = os.path.join(".claude", "state", "removed-comments.jsonl")


def repo_root(start):
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            cwd=start,
            capture_output=True,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    return result.stdout.strip() or None


def find_line(before_text, family, body):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return None
    for lineno, line in enumerate(before_text.splitlines(), start=1):
        stripped = line.strip()
        if stripped.startswith(line_marker) and normalize(stripped) == body:
            return lineno
    return None


def removed_comments(before_text, after_text, family, ext):
    before_counts = {}
    for body, _, _ in scan_comments(before_text, family, ext):
        before_counts[body] = before_counts.get(body, 0) + 1
    after_counts = {}
    for body, _, _ in scan_comments(after_text, family, ext):
        after_counts[body] = after_counts.get(body, 0) + 1

    removed = []
    for body, count in before_counts.items():
        extra = count - after_counts.get(body, 0)
        for _ in range(max(extra, 0)):
            removed.append((body, find_line(before_text, family, body)))
    return removed


def append_entries(root, path, entries):
    log_dir = os.path.join(root, ".claude", "state")
    os.makedirs(log_dir, exist_ok=True)
    log_path = os.path.join(root, LOG_RELATIVE_PATH)
    with open(log_path, "a", encoding="utf-8") as handle:
        for body, lineno in entries:
            handle.write(json.dumps({
                "file": path,
                "line": lineno,
                "text": body,
                "timestamp": time.time(),
            }) + "\n")


def handle_post(payload):
    tool_name = payload.get("tool_name", "")
    if tool_name not in WRITE_TOOLS:
        sys.exit(0)

    tool_input = payload.get("tool_input") or {}
    path = tool_input.get("file_path") or tool_input.get("notebook_path", "")
    family = resolve_family(path)
    if not family:
        sys.exit(0)
    ext = os.path.splitext(os.path.basename(path))[1].lower()

    edits = tool_input.get("edits") or []
    before_text = tool_input.get("old_string") or "\n".join(
        e.get("old_string", "") for e in edits
    )
    if not before_text.strip():
        sys.exit(0)

    if not os.path.isfile(path):
        sys.exit(0)
    with open(path, "r", encoding="utf-8", errors="ignore") as handle:
        after_text = handle.read()

    entries = removed_comments(before_text, after_text, family, ext)
    if not entries:
        sys.exit(0)

    root = repo_root(os.path.dirname(os.path.abspath(path))) or repo_root(os.getcwd())
    if root is None:
        sys.exit(0)

    append_entries(root, path, entries)
    sys.exit(0)


if __name__ == "__main__":
    try:
        raw_input = sys.stdin.read()
        if raw_input:
            payload = json.loads(raw_input)
            if payload.get("hook_event_name") == "PostToolUse":
                handle_post(payload)
    except SystemExit:
        raise
    except BaseException:
        pass
    sys.exit(0)
