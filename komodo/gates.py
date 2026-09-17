"""Deterministic checks the orchestrator runs itself: verify gates, done_when, compile gates, comment lint."""

from __future__ import annotations

import os
import shlex
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from typing import List, Optional, Sequence

VERIFY_ORDER = (
    (".claude/verify.py", "python"),
    ("scripts/verify.py", "python"),
    (".claude/verify.sh", "shell"),
    ("Makefile", "make"),
    ("Taskfile.yml", "task"),
    ("Taskfile.yaml", "task"),
    ("justfile", "just"),
)


@dataclass
class CommandResult:
    """One command's outcome, with output trimmed for a report."""

    command: str
    returncode: int
    output: str
    seconds: float = 0.0

    @property
    def ok(self) -> bool:
        """Whether the command exited zero."""
        return self.returncode == 0


@dataclass
class GateResult:
    """A gate's results across every command it ran."""

    name: str
    results: List[CommandResult] = field(default_factory=list)
    skipped: str = ""

    @property
    def ok(self) -> bool:
        """Whether every command passed, or the gate was skipped."""
        return all(result.ok for result in self.results)

    def failures(self) -> List[CommandResult]:
        """The commands that failed."""
        return [result for result in self.results if not result.ok]


def run_command(command: str, cwd: str, timeout: int, env: Optional[dict] = None, max_output: int = 12000) -> CommandResult:
    """Runs one shell command and captures combined output, timing it."""
    import time

    started = time.time()
    if os.name == "nt":
        # one raw string, not a list: list2cmdline would backslash-escape the inner quotes cmd.exe needs to see verbatim
        argv = 'cmd.exe /s /c "%s"' % command
        shell = False
    else:
        argv = command
        shell = True
    try:
        completed = subprocess.run(
            argv, shell=shell, cwd=cwd, capture_output=True, text=True, timeout=timeout, env=env,
        )
        output = (completed.stdout or "") + (completed.stderr or "")
        code = completed.returncode
    except subprocess.TimeoutExpired as error:
        output = ((error.stdout or b"").decode("utf-8", "replace") if isinstance(error.stdout, bytes) else (error.stdout or "")) + "\n[timed out after %ss]" % timeout
        code = 124
    if len(output) > max_output:
        output = output[: max_output // 2] + "\n...[trimmed]...\n" + output[-max_output // 2:]
    return CommandResult(command=command, returncode=code, output=output, seconds=round(time.time() - started, 1))


def _has_target(path: str, target: str, indent: str = "") -> bool:
    """Whether a build file declares a target line."""
    try:
        with open(path, encoding="utf-8", errors="ignore") as handle:
            return any(line.startswith(indent + target + ":") for line in handle)
    except OSError:
        return False


def resolve_verify(root: str) -> Optional[str]:
    """The repo's verify command in the shared discovery order, or None when it opts out."""
    python = shlex.quote(sys.executable or "python3")
    for relative, kind in VERIFY_ORDER:
        path = os.path.join(root, relative)
        if not os.path.isfile(path):
            continue
        if kind == "python":
            return "%s %s" % (python, relative)
        if kind == "shell":
            return "bash %s" % relative
        if kind == "make" and _has_target(path, "verify"):
            return "make verify"
        if kind == "task" and _has_target(path, "verify", "  "):
            return "task verify"
        if kind == "just" and _has_target(path, "verify"):
            return "just verify"
    return None


def compile_commands(root: str) -> List[str]:
    """Cheap whole-tree compile or typecheck commands, chosen from the manifests present."""
    commands: List[str] = []
    if os.path.isfile(os.path.join(root, "go.mod")):
        commands.append("go build ./... && go vet ./...")
    if os.path.isfile(os.path.join(root, "package.json")):
        if os.path.isfile(os.path.join(root, "tsconfig.json")) and (shutil.which("npx") or shutil.which("tsc")):
            commands.append("npx tsc --noEmit -p .")
    if os.path.isfile(os.path.join(root, "pyproject.toml")) or os.path.isfile(os.path.join(root, "setup.py")):
        commands.append("%s -m compileall -q ." % shlex.quote(sys.executable or "python3"))
    return commands


def run_gate(name: str, commands: Sequence[str], cwd: str, timeout: int, env: Optional[dict] = None) -> GateResult:
    """Runs commands in order, stopping at the first failure."""
    gate = GateResult(name=name)
    if not commands:
        gate.skipped = "nothing to run"
        return gate
    for command in commands:
        result = run_command(command, cwd, timeout, env)
        gate.results.append(result)
        if not result.ok:
            break
    return gate


def comments_check_command(root: str, paths: Sequence[str] = (), base: str = "HEAD") -> str:
    """The comment lint invocation for changed lines against base."""
    python = shlex.quote(sys.executable or "python3")
    parts = [python, "-m", "komodo", "comments", "check", "--base", shlex.quote(base)]
    parts.extend(shlex.quote(path) for path in paths)
    return " ".join(parts)
