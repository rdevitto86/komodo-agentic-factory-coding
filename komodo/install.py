"""Renders an adapter and installs it by copy, merging its settings policy into the personal settings file."""

from __future__ import annotations

import datetime
import json
import os
import shlex
import shutil
import subprocess
import tempfile
from typing import Any, Dict, List, Optional

from . import adapters

COPY_ENTRIES = ("AGENTS.md", "CLAUDE.md", "agents", "hooks", "skills", "standards")
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


MARKER = ".komodo-rendered"


def _remove(path: str, dry_run: bool, rendered: bool = False) -> str:
    """Removes a symlink or a previous render outright; backs up anything a person may have put there."""
    if os.path.islink(path):
        if not dry_run:
            os.unlink(path)
        return "unlinked old symlink"
    previous_render = rendered or (os.path.isdir(path) and os.path.exists(os.path.join(path, MARKER)))
    if os.path.exists(path) and previous_render:
        if not dry_run:
            if os.path.isdir(path):
                shutil.rmtree(path)
            else:
                os.remove(path)
        return "replaced previous render"
    if os.path.exists(path):
        stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
        backup = "%s.bak-%s" % (path, stamp)
        if not dry_run:
            os.rename(path, backup)
        return "backed up to %s" % os.path.basename(backup)
    return "new"


def install(target: Optional[str] = None, dry_run: bool = False, log=print, adapter: str = "claude", config=None) -> int:
    """Renders the adapter to a scratch directory, copies it into target, and generates settings.json."""
    target = os.path.abspath(os.path.expanduser(target or os.environ.get("AGENT_HOME") or os.path.join("~", ".claude")))
    interpreter = resolve_interpreter()
    if not interpreter:
        log("no python >= %d.%d found on PATH; hooks would not run" % PYTHON_FLOOR)
        return 1
    with tempfile.TemporaryDirectory() as scratch:
        written = adapters.render(adapter, scratch, config)
        log("rendered %s adapter: %d files" % (adapter, len(written)))
        log("installing into %s (copy, not symlink) using %s" % (target, " ".join(interpreter)))
        root_marker = os.path.join(target, MARKER)
        rendered_before = os.path.exists(root_marker)
        if not dry_run:
            os.makedirs(target, exist_ok=True)
        for name in COPY_ENTRIES:
            source = os.path.join(scratch, name)
            dest = os.path.join(target, name)
            if not os.path.exists(source):
                continue
            note = _remove(dest, dry_run, rendered_before)
            log("  copy %-10s (%s)" % (name, note))
            if dry_run:
                continue
            if os.path.isdir(source):
                shutil.copytree(source, dest)
                with open(os.path.join(dest, MARKER), "w", encoding="utf-8") as handle:
                    handle.write("rendered by komodo install; safe to replace\n")
            else:
                shutil.copy2(source, dest)
        with open(os.path.join(scratch, "settings.policy.json"), encoding="utf-8") as handle:
            policy = json.load(handle)
        policy = rewrite_hook_commands(policy, os.path.join(target, "hooks"), interpreter)
        settings_path = os.path.join(target, "settings.json")
        existing: Dict[str, Any] = {}
        if os.path.islink(settings_path):
            try:
                with open(settings_path, encoding="utf-8") as handle:
                    existing = json.load(handle)
            except (OSError, ValueError):
                existing = {}
            existing = {key: value for key, value in existing.items() if key not in POLICY_KEYS}
            log("  settings.json was a symlink; keeping %d personal key(s) from it" % len(existing))
            if not dry_run:
                os.unlink(settings_path)
        elif os.path.isfile(settings_path):
            try:
                with open(settings_path, encoding="utf-8") as handle:
                    existing = json.load(handle)
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
        seeds = os.path.join(scratch, "seeds")
        for name in sorted(os.listdir(seeds)) if os.path.isdir(seeds) else []:
            dest = os.path.join(target, name)
            if os.path.exists(dest):
                continue
            log("  seed %s" % name)
            if not dry_run:
                shutil.copy2(os.path.join(seeds, name), dest)
    if not dry_run:
        with open(os.path.join(target, MARKER), "w", encoding="utf-8") as handle:
            handle.write("rendered by komodo install; AGENTS.md, CLAUDE.md, agents, hooks, skills, standards are replaced on the next run\n")
    for stale in ("settings.local.json.tmpl", "CLAUDE.local.md.tmpl", "standards.bak", "modes", "templates"):
        path = os.path.join(target, stale)
        if os.path.islink(path):
            log("  unlink stale %s" % stale)
            if not dry_run:
                os.unlink(path)
    log("done. Re-run after editing komodo/rules, roles, standards, or adapters; the install is a copy. Restart the client to load it.")
    return 0
