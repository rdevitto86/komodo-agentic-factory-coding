"""Comment lint over changed lines: MISSING function docs and INVALID comments."""

from __future__ import annotations

import os
import subprocess
from typing import Dict, Iterable, List, Optional, Set

from . import comment_rules

SKIP_DIRS = (".git", "node_modules", "vendor", ".venv", "dist", "build", ".komodo")


def changed_lines(root: str, base: str = "HEAD", staged: bool = False) -> Optional[Dict[str, Set[int]]]:
    """Added line numbers per path from git diff, plus whole untracked files; None when git is unavailable."""
    args = ["git", "diff", "--unified=0", "--no-color"]
    if staged:
        args.append("--cached")
    else:
        args.append(base)
    try:
        result = subprocess.run(args + ["--"], cwd=root, capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    changed: Dict[str, Set[int]] = {}
    path = None
    lineno = 0
    for line in result.stdout.splitlines():
        if line.startswith("+++ b/"):
            path = line[6:]
            changed.setdefault(path, set())
        elif line.startswith("@@") and path is not None:
            marker = line.split("+", 1)
            if len(marker) > 1:
                span = marker[1].split("@@")[0].strip().split(",")
                try:
                    lineno = int(span[0])
                except ValueError:
                    lineno = 0
        elif line.startswith("+") and not line.startswith("+++") and path is not None:
            changed[path].add(lineno)
            lineno += 1
    if not staged:
        try:
            untracked = subprocess.run(["git", "ls-files", "--others", "--exclude-standard"], cwd=root, capture_output=True, text=True, timeout=60)
        except (OSError, subprocess.SubprocessError):
            untracked = None
        if untracked and untracked.returncode == 0:
            for relative in untracked.stdout.splitlines():
                full = os.path.join(root, relative)
                try:
                    with open(full, encoding="utf-8", errors="ignore") as handle:
                        count = sum(1 for _ in handle)
                except OSError:
                    continue
                changed[relative] = set(range(1, count + 1))
    return changed


def collect_files(root: str, paths: Iterable[str]) -> List[str]:
    """Files under paths (or the whole root) that the lint has a family for."""
    found: List[str] = []
    for target in list(paths) or [root]:
        full = target if os.path.isabs(target) else os.path.join(root, target)
        if os.path.isfile(full):
            found.append(full)
            continue
        for directory, dirs, names in os.walk(full):
            dirs[:] = [name for name in dirs if name not in SKIP_DIRS]
            found.extend(os.path.join(directory, name) for name in names)
    return [path for path in found if comment_rules.resolve_family(path)]


def check(root: str, paths: Iterable[str] = (), base: str = "HEAD", all_lines: bool = False, staged: bool = False, require: str = "nonobvious", trivial_lines: int = 8) -> List[Dict[str, object]]:
    """Findings over changed lines (or whole files with all_lines), sorted by file and line."""
    changed = None if all_lines else changed_lines(root, base, staged)
    findings: List[Dict[str, object]] = []
    for full in collect_files(root, paths):
        relative = os.path.relpath(full, root).replace("\\", "/")
        only: Optional[Set[int]] = None
        if changed is not None:
            only = changed.get(relative)
            if not only:
                continue
        try:
            with open(full, encoding="utf-8", errors="ignore") as handle:
                text = handle.read()
        except OSError:
            continue
        for lineno, name in comment_rules.undocumented_functions(text, relative, require, trivial_lines):
            if only is None or lineno in only:
                findings.append({"file": relative, "line": lineno, "kind": "MISSING", "rule": "FUNC_UNDOCUMENTED", "detail": "%s needs a one-line comment saying what it does" % name})
        for lineno, rule, detail in comment_rules.invalid_comments(text, relative, only):
            findings.append({"file": relative, "line": lineno, "kind": "INVALID", "rule": rule, "detail": detail})
    findings.sort(key=lambda item: (str(item["file"]), int(item["line"])))
    return findings


def format_findings(findings: List[Dict[str, object]]) -> str:
    """One line per finding plus a count."""
    lines = ["%s:%s: %s %s -- %s" % (f["file"], f["line"], f["kind"], f["rule"], f["detail"]) for f in findings]
    lines.append("%d comment finding(s)" % len(findings))
    return "\n".join(lines)
