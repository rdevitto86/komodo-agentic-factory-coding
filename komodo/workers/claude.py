"""Claude Code headless adapter: one `claude -p` process per brief, flags set from the role spec."""

from __future__ import annotations

import json
import shutil
import subprocess
from typing import Any, Dict, List

from .base import Brief, Result, Worker

DEFAULT_TOOLS = ["Read", "Edit", "Write", "Bash", "Grep", "Glob"]


def build_argv(brief: Brief, binary: str = "claude") -> List[str]:
    """The exact command line for a brief; the prompt itself travels on stdin."""
    spec = brief.spec
    argv = [
        binary, "-p",
        "--output-format", "json",
        "--setting-sources", "project",
        "--strict-mcp-config",
        "--no-session-persistence",
        "--dangerously-skip-permissions",
        "--system-prompt", brief.system,
    ]
    if spec.get("model"):
        argv += ["--model", str(spec["model"])]
    if spec.get("effort"):
        argv += ["--effort", str(spec["effort"])]
    if spec.get("max_turns"):
        argv += ["--max-turns", str(int(spec["max_turns"]))]
    if spec.get("max_budget_usd"):
        argv += ["--max-budget-usd", str(spec["max_budget_usd"])]
    tools = brief.tools if brief.tools is not None else DEFAULT_TOOLS
    argv += ["--tools", ",".join(tools)]
    if brief.schema:
        argv += ["--json-schema", json.dumps(brief.schema, separators=(",", ":"))]
    return argv


def parse_output(stdout: str) -> Result:
    """Turns the CLI's JSON envelope into a Result, preferring structured output."""
    try:
        envelope = json.loads(stdout)
    except ValueError:
        return Result(ok=False, text=stdout, error="claude returned non-JSON output")
    if isinstance(envelope, list):
        envelope = next((item for item in reversed(envelope) if isinstance(item, dict) and item.get("type") == "result"), {})
    usage = envelope.get("usage") or {}
    result = Result(
        ok=not envelope.get("is_error", False),
        text=str(envelope.get("result") or ""),
        cost_usd=float(envelope.get("total_cost_usd") or 0.0),
        input_tokens=int(usage.get("input_tokens") or 0) + int(usage.get("cache_creation_input_tokens") or 0) + int(usage.get("cache_read_input_tokens") or 0),
        output_tokens=int(usage.get("output_tokens") or 0),
        turns=int(envelope.get("num_turns") or 0),
    )
    structured = envelope.get("structured_output")
    if isinstance(structured, dict):
        result.data = structured
    elif result.text.strip().startswith("{"):
        try:
            parsed = json.loads(result.text)
            if isinstance(parsed, dict):
                result.data = parsed
        except ValueError:
            pass
    if result.ok and result.data is None and envelope.get("subtype", "").startswith("error"):
        result.ok = False
    if not result.ok and not result.error:
        result.error = str(envelope.get("subtype") or envelope.get("api_error_status") or "claude reported an error")
        if result.text:
            result.error += ": " + result.text[:400]
    return result


class ClaudeWorker(Worker):
    """Runs a brief through the installed Claude Code CLI."""

    name = "claude"

    def __init__(self, binary: str = "claude"):
        self.binary = binary

    def invoke(self, brief: Brief) -> Result:
        """Spawns the CLI with the brief's flags and parses its JSON envelope."""
        if shutil.which(self.binary) is None:
            return Result(ok=False, error="%s is not on PATH" % self.binary)
        argv = build_argv(brief, self.binary)
        try:
            completed = subprocess.run(
                argv, input=brief.prompt, cwd=brief.cwd, env=brief.env,
                capture_output=True, text=True, timeout=brief.timeout_s,
            )
        except subprocess.TimeoutExpired:
            return Result(ok=False, error="worker exceeded %ss" % brief.timeout_s)
        if completed.returncode != 0 and not completed.stdout.strip():
            return Result(ok=False, error="claude exited %d: %s" % (completed.returncode, completed.stderr.strip()[:600]))
        result = parse_output(completed.stdout)
        if completed.returncode != 0 and result.ok:
            result.ok = False
            result.error = "claude exited %d: %s" % (completed.returncode, completed.stderr.strip()[:600])
        return result
