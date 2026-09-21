"""Claude Code headless adapter: one streamed `claude -p` process per brief, flags set from the role spec."""

from __future__ import annotations

import json
import shutil
import subprocess
import threading
from typing import Any, Dict, List, Optional, Tuple

from .base import Brief, Result, Worker

DEFAULT_TOOLS = ["Read", "Edit", "Write", "Bash", "Grep", "Glob"]
# Envelope subtypes that mean a ceiling stopped the worker, mapped to the setting that fixed it.
LIMIT_SUBTYPES = {
    "error_max_turns": "max_turns",
    "error_max_budget_usd": "max_budget_usd",
    "error_max_structured_output_retries": "max_structured_output_retries",
}


def build_argv(brief: Brief, binary: str = "claude") -> List[str]:
    """The exact command line for a brief; the prompt itself travels on stdin."""
    spec = brief.spec
    argv = [
        binary, "-p",
        "--output-format", "stream-json",
        "--verbose",
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


def _usage_tokens(usage: Dict[str, Any]) -> Tuple[int, int]:
    """Input and output tokens from one usage block, counting cache reads as the input they replace."""
    inputs = int(usage.get("input_tokens") or 0) + int(usage.get("cache_creation_input_tokens") or 0) + int(usage.get("cache_read_input_tokens") or 0)
    return inputs, int(usage.get("output_tokens") or 0)


def _tokens(count: int) -> str:
    """A token count in the unit that reads at a glance, so a small burn is not rounded away to zero."""
    if count >= 1_000_000:
        return "%.1fM" % (count / 1_000_000.0)
    if count >= 1_000:
        return "%.1fK" % (count / 1_000.0)
    return str(count)


class Stream:
    """Accumulates a stream-json run, so accounting survives a worker that never reaches its final envelope."""

    def __init__(self) -> None:
        self.envelope: Optional[Dict[str, Any]] = None
        self.rate_limit: Optional[Dict[str, Any]] = None
        self.turns = 0
        self.input_tokens = 0
        self.output_tokens = 0
        self.text = ""

    def feed(self, line: str) -> None:
        """Folds one newline-delimited event into the running totals."""
        line = line.strip()
        if not line or not line.startswith("{"):
            return
        try:
            event = json.loads(line)
        except ValueError:
            return
        if not isinstance(event, dict):
            return
        kind = event.get("type")
        if kind == "assistant":
            usage = (event.get("message") or {}).get("usage") or {}
            if usage:
                self.turns += 1
                inputs, outputs = _usage_tokens(usage)
                self.input_tokens += inputs
                self.output_tokens += outputs
        elif kind == "rate_limit_event":
            info = event.get("rate_limit_info")
            if isinstance(info, dict):
                self.rate_limit = info
        elif kind == "result":
            self.envelope = event

    def partial(self, error: str) -> Result:
        """What a killed worker still did: its turns and tokens, with no final cost to report."""
        return Result(
            ok=False, error=error, turns=self.turns,
            input_tokens=self.input_tokens, output_tokens=self.output_tokens,
            rate_limit=self.rate_limit,
        )


def parse_output(stdout: str, spec: Optional[Dict[str, Any]] = None) -> Result:
    """Turns the CLI's JSON envelope into a Result, preferring structured output."""
    try:
        envelope = json.loads(stdout)
    except ValueError:
        return Result(ok=False, text=stdout, error="claude returned non-JSON output")
    if isinstance(envelope, list):
        envelope = next((item for item in reversed(envelope) if isinstance(item, dict) and item.get("type") == "result"), {})
    return result_from_envelope(envelope, spec)


def result_from_envelope(envelope: Dict[str, Any], spec: Optional[Dict[str, Any]] = None) -> Result:
    """Builds the Result for a finished run, naming the ceiling when one stopped it."""
    usage = envelope.get("usage") or {}
    inputs, outputs = _usage_tokens(usage)
    result = Result(
        ok=not envelope.get("is_error", False),
        text=str(envelope.get("result") or ""),
        cost_usd=float(envelope.get("total_cost_usd") or 0.0),
        input_tokens=inputs,
        output_tokens=outputs,
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
    subtype = str(envelope.get("subtype") or "")
    if result.ok and result.data is None and subtype.startswith("error"):
        result.ok = False
    if not result.ok and not result.error:
        result.error = limit_error(envelope, spec) or subtype or str(envelope.get("api_error_status") or "claude reported an error")
        if result.text and subtype not in LIMIT_SUBTYPES:
            result.error += ": " + result.text[:400]
    return result


def limit_error(envelope: Dict[str, Any], spec: Optional[Dict[str, Any]] = None) -> str:
    """Says which ceiling bound and what the worker had spent when it did, so the next raise is not a guess."""
    subtype = str(envelope.get("subtype") or "")
    key = LIMIT_SUBTYPES.get(subtype)
    if key is None:
        return ""
    setting = (spec or {}).get(key)
    usage = envelope.get("usage") or {}
    inputs, _ = _usage_tokens(usage)
    spent = "%d turn(s), %s input tokens, $%.2f list-price" % (
        int(envelope.get("num_turns") or 0), _tokens(inputs), float(envelope.get("total_cost_usd") or 0.0))
    at = " at %s" % setting if setting is not None else ""
    return "%s: %s bound%s after %s. Raise that tier's headroom or split the task; every other cap will bind next." % (subtype, key, at, spent)


class ClaudeWorker(Worker):
    """Runs a brief through the installed Claude Code CLI."""

    name = "claude"

    def __init__(self, binary: str = "claude"):
        self.binary = binary

    def invoke(self, brief: Brief) -> Result:
        """Streams the CLI's events so a killed worker still reports the turns and tokens it burned."""
        if shutil.which(self.binary) is None:
            return Result(ok=False, error="%s is not on PATH" % self.binary)
        argv = build_argv(brief, self.binary)
        stream = Stream()
        try:
            process = subprocess.Popen(
                argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                text=True, cwd=brief.cwd, env=brief.env,
            )
        except OSError as error:
            return Result(ok=False, error="claude failed to start: %s" % error)
        killed = threading.Event()

        def expire() -> None:
            """Marks the run as timed out and kills the process, ending the stream read."""
            killed.set()
            process.kill()

        watchdog = threading.Timer(max(1, int(brief.timeout_s)), expire)
        watchdog.daemon = True
        watchdog.start()
        errors: List[str] = []
        writer = threading.Thread(target=_write_stdin, args=(process, brief.prompt), daemon=True)
        reader = threading.Thread(target=_drain, args=(process.stderr, errors), daemon=True)
        writer.start()
        reader.start()
        try:
            assert process.stdout is not None
            for line in process.stdout:
                stream.feed(line)
            process.wait()
        finally:
            watchdog.cancel()
            reader.join(timeout=5)
        stderr = "".join(errors).strip()
        if killed.is_set():
            return stream.partial("worker exceeded %ss after %d turn(s) and %s input tokens" % (
                brief.timeout_s, stream.turns, _tokens(stream.input_tokens)))
        if stream.envelope is None:
            return Result(ok=False, error="claude exited %d with no result event: %s" % (process.returncode, stderr[:600]),
                          turns=stream.turns, input_tokens=stream.input_tokens, output_tokens=stream.output_tokens,
                          rate_limit=stream.rate_limit)
        result = result_from_envelope(stream.envelope, brief.spec)
        result.rate_limit = stream.rate_limit
        if process.returncode != 0 and result.ok:
            result.ok = False
            result.error = "claude exited %d: %s" % (process.returncode, stderr[:600])
        return result


def _write_stdin(process: "subprocess.Popen[str]", prompt: str) -> None:
    """Feeds the prompt on its own thread, so a prompt past the pipe buffer cannot deadlock the read."""
    try:
        assert process.stdin is not None
        process.stdin.write(prompt)
        process.stdin.close()
    except (OSError, ValueError):
        pass


def _drain(pipe: Any, sink: List[str]) -> None:
    """Reads a pipe to EOF into sink, so a chatty stderr cannot fill and block."""
    try:
        for chunk in pipe:
            sink.append(chunk)
    except (OSError, ValueError):
        pass
