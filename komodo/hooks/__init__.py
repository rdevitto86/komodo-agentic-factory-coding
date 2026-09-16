"""Git hooks installed via core.hooksPath, plus the installer that points repos at them."""

from __future__ import annotations

import os
import stat
import subprocess
from typing import List, Tuple

HOOKS_DIR = os.path.dirname(os.path.abspath(__file__))
STUBS = ("pre-commit", "pre-push")


def ensure_executable() -> None:
    """Sets the executable bit on the shell stubs where the filesystem has one."""
    for name in STUBS:
        path = os.path.join(HOOKS_DIR, name)
        try:
            mode = os.stat(path).st_mode
            os.chmod(path, mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        except OSError:
            pass


def install(repos: List[str], status_only: bool = False) -> List[Tuple[str, str]]:
    """Points each repo's core.hooksPath at this directory; returns (repo, outcome) pairs."""
    ensure_executable()
    outcomes = []
    for repo in repos:
        if subprocess.run(["git", "-C", repo, "rev-parse", "--git-dir"], capture_output=True).returncode != 0:
            outcomes.append((repo, "skipped: not a git repo"))
            continue
        current = subprocess.run(["git", "-C", repo, "config", "--local", "--get", "core.hooksPath"], capture_output=True, text=True).stdout.strip()
        if os.path.normcase(os.path.realpath(current)) == os.path.normcase(os.path.realpath(HOOKS_DIR)) if current else False:
            outcomes.append((repo, "already installed"))
            continue
        if status_only:
            outcomes.append((repo, "hooksPath=%s" % (current or "<unset>")))
            continue
        legacy = subprocess.run(["git", "-C", repo, "rev-parse", "--git-path", "hooks"], capture_output=True, text=True).stdout.strip()
        legacy_dir = os.path.join(repo, legacy) if not os.path.isabs(legacy) else legacy
        orphans = []
        if os.path.isdir(legacy_dir):
            orphans = [name for name in os.listdir(legacy_dir) if not name.endswith(".sample") and os.path.isfile(os.path.join(legacy_dir, name))]
        subprocess.run(["git", "-C", repo, "config", "--local", "core.hooksPath", HOOKS_DIR], check=True)
        note = "installed"
        if orphans:
            note += " (these .git/hooks stop running: %s)" % ", ".join(sorted(orphans)[:5])
        outcomes.append((repo, note))
    return outcomes
