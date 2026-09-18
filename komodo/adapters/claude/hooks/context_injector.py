#!/usr/bin/env python3
"""SessionStart: a few lines of repo work state so a session resumes without re-deriving it. Fails open."""

from __future__ import annotations

import os
import re
import subprocess
import sys

TASK = re.compile(r"^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*[A-Z]\]\s*\[([A-Z_]+)\]\s*$")
GROUP = re.compile(r"^###\s+\[(TG-[\w.]+)\]\s*(.*?)\s*$")
VERSION = re.compile(r"^##\s*\[([^\]]+)\]")


def repo_root() -> str:
    """The git toplevel, or empty."""
    try:
        return subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, timeout=3).stdout.strip()
    except Exception:
        return ""


def read(path: str) -> list:
    """Lines of a file, or an empty list."""
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read().splitlines()
    except OSError:
        return []


def main() -> None:
    """Prints the work-state summary."""
    root = repo_root()
    if not root:
        return
    backlog = None
    for relative in ("BACKLOG.md", os.path.join("docs", "BACKLOG.md")):
        if os.path.isfile(os.path.join(root, relative)):
            backlog = os.path.join(root, relative)
            break
    lines = ["Work state, read from disk at session start:"]
    if backlog:
        in_progress, blocked, refinement, open_count, next_group = [], 0, 0, 0, None
        current = None
        for line in read(backlog):
            group = GROUP.match(line)
            if group:
                current = group.group(1)
                continue
            match = TASK.match(line)
            if not match:
                continue
            status = match.group(3)
            if status == "DONE":
                continue
            open_count += 1
            if status == "BLOCKED":
                blocked += 1
            elif status == "REFINEMENT":
                refinement += 1
            elif status in ("IN_PROGRESS", "WIP"):
                in_progress.append("%s %s" % (match.group(1), match.group(2)[:100]))
            elif next_group is None:
                next_group = current
        if in_progress:
            lines.append("In progress: " + "; ".join(in_progress[:3]))
        counts = "%d open" % open_count
        if blocked:
            counts += ", %d blocked" % blocked
        if refinement:
            counts += ", %d in refinement" % refinement
        lines.append("Backlog: %s. Next group: %s." % (counts, next_group or "none"))
    else:
        lines.append("No BACKLOG.md; the harness has nothing to run here.")
    for line in read(os.path.join(root, "CHANGELOG.md")):
        match = VERSION.match(line)
        if match:
            lines.append("Released version: %s." % match.group(1))
            break
    for relative in (".claude/verify.py", "scripts/verify.py", ".claude/verify.sh", "Makefile"):
        if os.path.isfile(os.path.join(root, relative)):
            lines.append("Verify gate: %s." % ("make verify" if relative == "Makefile" else relative))
            break
    else:
        lines.append("No verify gate declared.")
    lines.append("Run a task group: python3 -m komodo run [group] [--dry-run].")
    sys.stdout.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        pass
