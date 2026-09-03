#!/usr/bin/env python3
#
# auto_format.py - PostToolUse hook for Edit/Write. Runs the touched
# file through its language's formatter, so a formatting-only lint
# failure never costs a verify-gate round trip.
#
# gofmt for .go, prettier for the JS/TS/CSS/etc family it covers.
# Neither is required: if the formatter isn't on PATH, this no-ops.
#
# This hook FAILS OPEN, like verify_gate.py and context_injector.py.
# It is a convenience, not a guard — a crash or a missing formatter
# must never be able to block a write that comment_guard.py and
# git_guard.py already allowed through.

import json
import os
import shutil
import subprocess
import sys

GOFMT_EXTENSIONS = {".go"}

PRETTIER_EXTENSIONS = {
    ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".json", ".css", ".scss",
    ".html", ".vue", ".svelte", ".md", ".yaml", ".yml",
}


def formatter_for(path):
    _, ext = os.path.splitext(path)
    ext = ext.lower()
    if ext in GOFMT_EXTENSIONS:
        return "gofmt", ["gofmt", "-w", path]
    if ext in PRETTIER_EXTENSIONS:
        return "prettier", ["prettier", "--write", path]
    return None, None


def run_formatter(path):
    binary, command = formatter_for(path)
    if binary is None:
        return
    if shutil.which(binary) is None:
        return

    try:
        subprocess.run(command, capture_output=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        pass


def main():
    payload = json.loads(sys.stdin.read())
    if payload.get("tool_name") not in ("Edit", "Write"):
        sys.exit(0)

    file_path = (payload.get("tool_input") or {}).get("file_path", "")
    if not file_path or not os.path.isfile(file_path):
        sys.exit(0)

    run_formatter(file_path)

    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
