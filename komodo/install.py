"""Installs the thin Claude Code adapter into ~/.claude by copy, merging policy into a personal settings.json."""

from __future__ import annotations

import datetime
import json
import os
import shlex
import shutil
import subprocess
import sys
from typing import Any, Dict, List, Optional, Tuple

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SOURCE = os.path.join(REPO_ROOT, "claude-code")
COPY_ENTRIES = ("AGENTS.md", "CLAUDE.md", "agents", "hooks", "skills")
POLICY_KEYS = ("permissions", "hooks", "skillOverrides")
PYTHON_FLOOR = (3, 9)


def resolve_interpreter() -> List[str]:
    """The python the hook commands will run under: python3, python, or the py launcher."""
    for candidate in (["python3"], ["python"], ["py", "-3"]):
        if shutil.which(candidate[0]) is None:
            continue
        try:
            result = subprocess.run(candidate + ["-c", "import sys; print(sys.version_info[0], sys.version_info[1])"], capture_output=True, text=True, timeout=10)
        except (OSError, subprocess.SubprocessError):
            continue
        parts = result.stdout.split()
        if result.returncode == 0 and len(parts) == 2 and (int(parts[0]), int(parts[1])) >= PYTHON_FLOOR:
            return candidate
    return []


def rewrite_hook_commands(policy: Dict[str, Any], hooks_dir: str, interpreter: List[str]) -> Dict[str, Any]:
    """Replaces `python3 ~/.claude/hooks/x.py` with the resolved interpreter and an absolute, tilde-free path."""
    for entries in (policy.get("hooks") or {}).values():
        for entry in entries:
            for hook in entry.get("hooks", []):
                tokens = shlex.split(str(hook.get("command", "")))
                index = next((i for i, token in enumerate(tokens) if token.endswith(".py")), None)
                if index is None:
                    continue
                script = os.path.join(hooks_dir, os.path.basename(tokens[index]))
                hook["command"] = " ".join(shlex.quote(part) for part in interpreter + [script] + tokens[index + 1:])
    return policy


def merge_settings(existing: Dict[str, Any], policy: Dict[str, Any]) -> Dict[str, Any]:
    """Policy keys replace, every other key in the existing file survives."""
    merged = dict(existing)
    for key in POLICY_KEYS:
        if key in policy:
            merged[key] = policy[key]
    return merged


def _remove(path: str, dry_run: bool) -> str:
    """Removes a symlink, or backs up a real file or directory; returns what happened."""
    if os.path.islink(path):
        if not dry_run:
            os.unlink(path)
        return "unlinked old symlink"
    if os.path.exists(path):
        stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
        backup = "%s.bak-%s" % (path, stamp)
        if not dry_run:
            os.rename(path, backup)
        return "backed up to %s" % os.path.basename(backup)
    return "new"


def install(target: Optional[str] = None, dry_run: bool = False, log=print) -> int:
    """Copies the adapter, generates settings.json, and points hooks at the copy."""
    target = os.path.abspath(os.path.expanduser(target or os.environ.get("AGENT_HOME") or os.path.join("~", ".claude")))
    interpreter = resolve_interpreter()
    if not interpreter:
        log("no python >= %d.%d found on PATH; hooks would not run" % PYTHON_FLOOR)
        return 1
    log("installing claude-code/ into %s (copy, not symlink) using %s" % (target, " ".join(interpreter)))
    if not dry_run:
        os.makedirs(target, exist_ok=True)
    for name in COPY_ENTRIES:
        source = os.path.join(SOURCE, name)
        dest = os.path.join(target, name)
        if not os.path.exists(source):
            continue
        note = _remove(dest, dry_run)
        log("  copy %-10s (%s)" % (name, note))
        if dry_run:
            continue
        if os.path.isdir(source):
            shutil.copytree(source, dest, ignore=shutil.ignore_patterns("__pycache__", "synced", "*.pyc"))
        else:
            shutil.copy2(source, dest)
    standards_src = os.path.join(REPO_ROOT, "komodo", "standards")
    standards_dest = os.path.join(target, "standards")
    note = _remove(standards_dest, dry_run)
    log("  copy %-10s (%s, from komodo/standards)" % ("standards", note))
    if not dry_run:
        shutil.copytree(standards_src, standards_dest, ignore=shutil.ignore_patterns("*.py", "__pycache__"))
    policy_path = os.path.join(SOURCE, "settings.policy.json")
    policy = json.load(open(policy_path, encoding="utf-8"))
    policy = rewrite_hook_commands(policy, os.path.join(target, "hooks"), interpreter)
    settings_path = os.path.join(target, "settings.json")
    existing: Dict[str, Any] = {}
    if os.path.islink(settings_path):
        try:
            existing = json.load(open(settings_path, encoding="utf-8"))
        except (OSError, ValueError):
            existing = {}
        personal = {key: value for key, value in existing.items() if key not in POLICY_KEYS}
        log("  settings.json was a symlink; keeping %d personal key(s) from it" % len(personal))
        existing = personal
        if not dry_run:
            os.unlink(settings_path)
    elif os.path.isfile(settings_path):
        try:
            existing = json.load(open(settings_path, encoding="utf-8"))
        except ValueError:
            log("  settings.json is not valid JSON; backing it up")
            _remove(settings_path, dry_run)
            existing = {}
    merged = merge_settings(existing, policy)
    log("  write settings.json (policy keys: %s; personal keys kept: %d)" % (", ".join(k for k in POLICY_KEYS if k in policy), len([k for k in merged if k not in POLICY_KEYS])))
    if not dry_run:
        with open(settings_path, "w", encoding="utf-8") as handle:
            json.dump(merged, handle, indent=2)
            handle.write("\n")
    for name in ("settings.local.json", "CLAUDE.local.md"):
        dest = os.path.join(target, name)
        template = os.path.join(SOURCE, name + ".tmpl")
        if os.path.exists(dest) or not os.path.exists(template):
            continue
        log("  seed %s from template" % name)
        if not dry_run:
            shutil.copy2(template, dest)
    for stale in ("settings.local.json.tmpl", "CLAUDE.local.md.tmpl", "standards", "modes", "templates"):
        path = os.path.join(target, stale)
        if os.path.islink(path):
            log("  unlink stale %s" % stale)
            if not dry_run:
                os.unlink(path)
    log("done. Re-run after editing claude-code/; the install is a copy. Restart Claude Code to load it.")
    return 0
