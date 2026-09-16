#!/usr/bin/env python3
#
# evals.py - this repo's single entry point for running skill eval suites, the same way verify.py
# is the single entry point for the gate. Never called from verify.py, Makefile's verify target, or
# .github/workflows/verify.yml -- running a real case means a real model call and real wall-clock,
# so this stays on-demand rather than folded into the gate. See docs/design-decisions.md's
# "Skill eval coverage" section for the covered set and the gate decision.
#
#   python3 scripts/evals.py --list                 # print the covered set, exit 0, no model call
#   python3 scripts/evals.py                         # run every covered skill's suite
#   python3 scripts/evals.py workflow-loop assess-bugs   # run a subset
#   python3 scripts/evals.py -- --threshold 0.8      # forward args to `claude plugin eval`
#
# Exit codes: 0 every requested suite scored at or above its threshold, 1 otherwise or a bad arg,
# 127 the `claude` CLI is not on PATH.

import os
import subprocess
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SKILLS_DIR = os.path.join(REPO_ROOT, "claude-code", "skills")

# Mirrors docs/design-decisions.md's "Skill eval coverage" section by hand -- a skill records rules, never inventory.
COVERED_SKILLS = (
    ("workflow-loop", "the default engineering mode; AGENTS.md names it for anything bigger than a one-line fix"),
    ("backlog-modify", "owns BACKLOG.md's format, which context_injector.py regex-reads every session start"),
    ("git-pr-create", "the only path that opens a PR; owns branch/PR/protected-ref conventions git_guard.py enforces"),
    ("write-comments", "owns the nine-type comment taxonomy comments.py enforces mechanically"),
    ("assess-bugs", "runs unconditionally every band in workflow-loop's P2.3; a missed bug ships broken behavior"),
)
COVERED_NAMES = tuple(name for name, _ in COVERED_SKILLS)


def print_list():
    width = max(len(name) for name in COVERED_NAMES)
    for name, reason in COVERED_SKILLS:
        print("%-*s  %s" % (width, name, reason))


def claude_on_path():
    for directory in os.environ.get("PATH", "").split(os.pathsep):
        candidate = os.path.join(directory, "claude")
        if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
            return True
    return False


def run_suite(name, forward_args):
    skill_dir = os.path.join(SKILLS_DIR, name)
    command = [
        "claude", "plugin", "eval", skill_dir,
        "--trust-plugin", "--ablation", "with-without",
    ] + list(forward_args)
    sys.stdout.write("evals: running %s\n" % name)
    result = subprocess.run(command, cwd=REPO_ROOT)
    return result.returncode


def main(argv):
    if "--list" in argv:
        print_list()
        return 0

    forward_args = []
    requested = []
    args = iter(argv)
    for arg in args:
        if arg == "--":
            forward_args.extend(args)
            continue
        if arg.startswith("-"):
            forward_args.append(arg)
            continue
        requested.append(arg)

    targets = requested or list(COVERED_NAMES)
    unknown = [name for name in targets if name not in COVERED_NAMES]
    if unknown:
        sys.stdout.write("evals: not a covered skill: %s\n" % ", ".join(unknown))
        sys.stdout.write("evals: covered skills are %s\n" % ", ".join(COVERED_NAMES))
        return 1

    if not claude_on_path():
        sys.stdout.write("evals: the `claude` CLI is not on PATH\n")
        return 127

    for name in targets:
        returncode = run_suite(name, forward_args)
        if returncode != 0:
            sys.stdout.write("evals: %s failed\n" % name)
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
