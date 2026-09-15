#!/usr/bin/env python3
#
# verify.py - this repo's verify gate. Runs the hook regression suite, the config validator, and the
# comment lint, in that order, stopping at the first failure.
#
#   python3 scripts/verify.py
#
# Exit codes: 0 every check passed, 1 a check failed.

import os
import subprocess
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

CHECKS = (
    ("test", os.path.join("scripts", "test_hooks.py"), []),
    ("validate", os.path.join("scripts", "validate.py"), []),
    ("comments", os.path.join("claude-code", "hooks", "comments.py"), ["check"]),
)


def interpreter():
    return sys.executable or "python3"


def main():
    for name, script, args in CHECKS:
        result = subprocess.run(
            [interpreter(), os.path.join(REPO_ROOT, script)] + args,
            cwd=REPO_ROOT,
        )
        if result.returncode != 0:
            sys.stdout.write("verify: %s failed\n" % name)
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
