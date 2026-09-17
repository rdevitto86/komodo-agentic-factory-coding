#!/usr/bin/env python3
"""This repo's verify gate: unit tests, config validation, comment lint, and the fragment doctor, stopping at the first failure."""

import os
import subprocess
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PYTHON = sys.executable or "python3"
CHECKS = (
    ("tests", [PYTHON, "-m", "unittest", "discover", "-s", "tests", "-q"]),
    ("validate", [PYTHON, os.path.join("scripts", "validate.py")]),
    ("comments", [PYTHON, "-m", "komodo", "comments", "check"]),
    ("doctor", [PYTHON, "-m", "komodo", "doctor", "--no-git"]),
    ("hooks", [PYTHON, os.path.join("scripts", "build-hooks.py"), "--check"]),
)


def main() -> int:
    """Runs each check in order and reports the first failure."""
    for name, command in CHECKS:
        print("verify: %s" % name)
        result = subprocess.run(command, cwd=REPO_ROOT)
        if result.returncode != 0:
            print("verify: %s failed" % name)
            return 1
    print("verify: all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
