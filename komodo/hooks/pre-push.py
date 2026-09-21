#!/usr/bin/env python3
"""pre-push: refuse a protected ref, a delete, or a force update, then run the repo's verify gate."""

from __future__ import annotations

import fnmatch
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(os.path.dirname(HERE)))

ZERO = "0" * 40


def git(*args: str) -> str:
    """Runs git in the current repo and returns stdout."""
    return subprocess.run(["git", *args], capture_output=True, text=True).stdout


def protected_patterns(root: str) -> list:
    """Patterns from config, else the built-in set."""
    try:
        from komodo.config import Config

        return Config.load(root).protected
    except Exception:
        return ["main", "master", "trunk", "prod", "production", "release/*", "hotfix/*"]


def is_force(local_sha: str, remote_sha: str) -> bool:
    """Whether the update is not a fast-forward of the remote ref."""
    if remote_sha == ZERO or local_sha == ZERO:
        return False
    return subprocess.run(["git", "merge-base", "--is-ancestor", remote_sha, local_sha], capture_output=True).returncode != 0


def main() -> int:
    """Reads the ref updates git passes on stdin and refuses the dangerous ones before running verify."""
    root = git("rev-parse", "--show-toplevel").strip()
    if not root:
        return 0
    os.chdir(root)
    patterns = protected_patterns(root)
    problems = []
    for line in sys.stdin.read().splitlines():
        parts = line.split()
        if len(parts) != 4:
            continue
        local_ref, local_sha, remote_ref, remote_sha = parts
        name = remote_ref[len("refs/heads/"):] if remote_ref.startswith("refs/heads/") else remote_ref
        if any(fnmatch.fnmatchcase(name, pattern) for pattern in patterns):
            problems.append("push to protected ref %s; open a pull request instead" % name)
        if local_sha == ZERO:
            problems.append("deleting remote ref %s from a hook-guarded push" % name)
        if is_force(local_sha, remote_sha):
            problems.append("non-fast-forward update of %s; rewriting published history is refused" % name)
    if problems:
        sys.stderr.write("pre-push: blocked\n")
        for problem in problems:
            sys.stderr.write("  %s\n" % problem)
        return 1

    try:
        from komodo.gates import resolve_verify, run_command
        from komodo.gitops import clean_env
    except Exception:
        return 0
    command = resolve_verify(root)
    if command is None:
        return 0
    timeout = int(os.environ.get("KOMODO_VERIFY_TIMEOUT", "600") or 600)
    result = run_command(command, root, timeout, env=clean_env())
    if not result.ok:
        sys.stderr.write(result.output)
        sys.stderr.write("\npre-push: %s failed; fix it or push with --no-verify if you must\n" % command)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
