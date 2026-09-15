#!/usr/bin/env python3
#
# context_injector.py - SessionStart hook. Puts the repo's current work
# state in front of the model before the first prompt, so a session
# resumes without re-deriving it.
#
# It emits at most a dozen lines:
#
#   1. the [IN_PROGRESS] story from BACKLOG.md, and its Done when command
#   2. how many stories are open, and how many are blocked
#   3. the current version, from the first CHANGELOG.md heading
#   4. whether this repo declares a verify gate
#
# Disk reads only. No network, no subprocess beyond `git rev-parse`,
# no probing of the bridge — a session must never wait on one.
#
# This hook FAILS OPEN, like verify_gate.py and unlike the two guards.
# Injected context is a convenience; a crash here must never be able to
# stop a session from starting, so any internal error exits 0 silently.

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.git import repo_root

MAX_STORY_CHARS = 160
BACKLOG_NAMES = ("BACKLOG.md", "docs/BACKLOG.md")
STORY = re.compile(r"^####\s*\[TSK-[\w.]+\]\s*(.+?)\s*\[P:\s*[A-Z]+\]\s*\[([A-Z_]+)\]\s*$")
VERSION = re.compile(r"^##\s*\[([^\]]+)\]")


def read_lines(path):
    if not os.path.isfile(path):
        return None
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read().splitlines()
    except OSError:
        return None


def find_backlog(root):
    for name in BACKLOG_NAMES:
        lines = read_lines(os.path.join(root, name))
        if lines is not None:
            return name, lines
    return None, None


def clip(text):
    if len(text) <= MAX_STORY_CHARS:
        return text
    return text[:MAX_STORY_CHARS].rstrip() + "..."


def scan_stories(lines):
    in_progress, blocked, total = [], 0, 0
    for line in lines:
        match = STORY.match(line)
        if match is None:
            continue
        status = match.group(2).upper()
        if status == "DONE":
            continue
        total += 1
        if status == "IN_PROGRESS":
            in_progress.append(clip(match.group(1)))
        elif status == "BLOCKED":
            blocked += 1
    return in_progress, blocked, total


def current_version(root):
    lines = read_lines(os.path.join(root, "CHANGELOG.md"))
    if lines is None:
        return None
    for line in lines:
        match = VERSION.match(line)
        if match is None:
            continue
        label = match.group(1).strip()
        if label.lower() != "unreleased":
            return label
    return None


def verify_label(root):
    script = os.path.join(root, ".claude", "verify.sh")
    if os.path.isfile(script) and os.access(script, os.X_OK):
        return ".claude/verify.sh"
    for name, pattern in (("Makefile", "verify:"), ("justfile", "verify:")):
        lines = read_lines(os.path.join(root, name))
        if lines and any(line.startswith(pattern) for line in lines):
            return "%s verify" % ("make" if name == "Makefile" else "just")
    for name in ("Taskfile.yml", "Taskfile.yaml"):
        lines = read_lines(os.path.join(root, name))
        if lines and any(line.startswith("  verify:") for line in lines):
            return "task verify"
    return None


def main():
    root = repo_root(os.getcwd())
    if root is None:
        sys.exit(0)

    name, lines = find_backlog(root)
    if lines is None:
        sys.exit(0)

    in_progress, blocked, total = scan_stories(lines)
    report = ["Work state, read from disk at session start:", ""]

    if in_progress:
        report.append("In progress (%s):" % name)
        for story in in_progress[:3]:
            report.append("  - %s" % story)
    else:
        report.append("Nothing marked [IN_PROGRESS] in %s." % name)

    tally = "%d open" % total
    if blocked:
        tally += ", %d BLOCKED" % blocked
    report.append("Backlog: %s." % tally)

    version = current_version(root)
    if version:
        report.append("Released version: %s." % version)

    gate = verify_label(root)
    if gate:
        report.append("Verify gate: %s." % gate)
    else:
        report.append(
            "WARNING: no verify gate declared -- this repo's checks, and "
            "`comments.py check` with them, never run automatically."
        )

    sys.stdout.write("\n".join(report) + "\n")
    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
