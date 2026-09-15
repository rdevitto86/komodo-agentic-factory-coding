#!/usr/bin/env python3
from __future__ import annotations

import os
import re
import subprocess
import sys

VERSION_HEADING = re.compile(r"^## \[([0-9]+\.[0-9]+\.[0-9]+)\]", re.M)


def run(args: list, repo_root: str) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-C", repo_root] + args, capture_output=True)


def read_version(repo_root: str) -> str:
    path = os.path.join(repo_root, "CHANGELOG.md")
    try:
        with open(path, encoding="utf-8") as handle:
            body = handle.read()
    except OSError:
        return ""
    match = VERSION_HEADING.search(body)
    return match.group(1) if match else ""


def main() -> int:
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

    status = run(["status", "--porcelain"], repo_root)
    if status.returncode != 0:
        sys.stderr.write(status.stderr.decode("utf-8", "replace"))
        return status.returncode
    if status.stdout.decode("utf-8", "replace").strip():
        sys.stderr.write("error: working tree is dirty — commit all changes before releasing\n")
        return 1

    version = read_version(repo_root)
    if not version:
        sys.stderr.write('error: CHANGELOG.md has no "## [x.y.z]" heading — add one before releasing\n')
        return 1

    tag = "v" + version
    if run(["rev-parse", tag], repo_root).returncode == 0:
        sys.stderr.write(
            "error: tag %s already exists — bump the CHANGELOG.md heading to the new version first\n" % tag
        )
        return 1

    created = subprocess.run(["git", "-C", repo_root, "tag", tag])
    if created.returncode != 0:
        return created.returncode
    pushed = subprocess.run(["git", "-C", repo_root, "push", "origin", tag])
    if pushed.returncode != 0:
        return pushed.returncode

    print("released %s" % tag)
    return 0


if __name__ == "__main__":
    sys.exit(main())
