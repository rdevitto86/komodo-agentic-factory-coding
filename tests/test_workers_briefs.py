import io
import json
import os
import threading
import unittest
from unittest import mock

from komodo import account, briefs
from komodo.workers import Brief, base
from komodo.workers import claude as claude_worker


def envelope_json(envelope):
    """One stream-json result event as the text a whole-output parse receives."""
    return json.dumps(envelope)


class FakeProcess:
    """A Popen stand-in that yields canned stream-json lines, optionally dying before the result event."""

    def __init__(self, events, returncode=0, kill_before_result=False, stderr=""):
        self.stdin = io.StringIO()
        self.stderr = iter([stderr] if stderr else [])
        self.returncode = returncode
        self._final = returncode
        self._killed = threading.Event()
        self._hang = kill_before_result
        self.stdout = self._emit([json.dumps(event) + "\n" for event in events])

    def _emit(self, lines):
        """Yields the canned lines, then hangs until the watchdog kills it when the run is meant to time out."""
        for line in lines:
            yield line
        if self._hang:
            self._killed.wait(30)

    def wait(self, timeout=None):
        """Settles the exit status once the caller has drained stdout."""
        self.returncode = -9 if self._killed.is_set() else self._final
        return self.returncode

    def kill(self):
        """Releases the hung stream the way a real kill closes the pipe."""
        self._killed.set()

BUILDER_SLOTS = {
    "task_id": "TSK-1", "title": "Do it", "task_block": "files: [a.go]", "repo_rules": "none",
    "context": "n/a", "files": "a.go: ...", "standards": "go rules", "done_when": "- go test ./...",
    "failure": "", "comment_convention": "Go: godoc on exported identifiers.",
}


class BriefTests(unittest.TestCase):
    def test_every_role_template_renders(self):
        slots = {
            "builder": BUILDER_SLOTS,
            "reviewer": {"group_id": "TG-1", "title": "t", "tasks": "- x", "standards": "s", "base": "main", "diff": "+a"},
            "planner": {"goal": "g", "layout": "l", "spec": "s", "existing": "none"},
            "merger": {"incoming": "main", "branch": "feat/x", "files": "a.go", "ours": "o", "theirs": "t"},
            "responder": {"pr_number": 1, "path": "a.go", "line": 3, "thread": "th", "excerpt": "code", "done_when": "- go test"},
            "summarizer": {"what": "a changelog entry", "material": "m"},
        }
        for role, values in slots.items():
            system, prompt = briefs.render(role, values)
            self.assertTrue(system.strip(), role)
            self.assertTrue(prompt.strip(), role)
            self.assertNotIn("{{", system + prompt, role)

    def test_missing_slot_raises(self):
        with self.assertRaises(briefs.BriefError):
            briefs.render("reviewer", {"group_id": "x"})

    def test_system_prompts_stay_small(self):
        from komodo import roles

        for name in roles.available():
            size = len(roles.load(name).system_prompt())
            self.assertLess(size // 4, 900, "%s system prompt is %d tokens" % (name, size // 4))

    def test_clip_marks_truncation(self):
        text = "x" * 1000
        clipped = briefs.clip(text, 100, "file")
        self.assertIn("truncated", clipped)
        self.assertLess(len(clipped), 400)
        self.assertEqual(briefs.clip("short", 100), "short")

    def test_schema_and_tools(self):
        self.assertIn("findings", briefs.schema_for("reviewer")["properties"])
        self.assertIn("blast_radius", briefs.schema_for("reviewer")["required"])
        self.assertEqual(briefs.tools_for("reviewer"), ["Read", "Grep", "Glob"])
        self.assertEqual(briefs.tools_for("summarizer"), [])
        self.assertIn("Edit", briefs.tools_for("builder"))


class ClaudeArgvTests(unittest.TestCase):
    def test_argv_carries_every_spec_flag(self):
        brief = Brief(role="builder", system="sys", prompt="p", cwd=".", spec={"model": "sonnet", "effort": "high", "max_turns": 7, "max_budget_usd": 1.5}, schema={"type": "object"}, tools=["Read", "Bash"])
        argv = claude_worker.build_argv(brief)
        joined = " ".join(argv)
        for flag in ("--model sonnet", "--effort high", "--max-turns 7", "--max-budget-usd 1.5", "--tools Read,Bash", "--json-schema", "--setting-sources project", "--no-session-persistence", "--dangerously-skip-permissions", "--output-format stream-json", "--verbose"):
            self.assertIn(flag, joined)
        self.assertNotIn("p", argv[2:3])

    def test_parse_output_prefers_structured(self):
        envelope = {"is_error": False, "result": "text", "structured_output": {"result": "DONE"}, "total_cost_usd": 0.5, "usage": {"input_tokens": 10, "cache_read_input_tokens": 5, "output_tokens": 3}, "num_turns": 4}
        result = claude_worker.parse_output(json.dumps(envelope))
        self.assertTrue(result.ok)
        self.assertEqual(result.data, {"result": "DONE"})
        self.assertEqual(result.input_tokens, 15)
        self.assertEqual(result.cost_usd, 0.5)
        self.assertEqual(result.turns, 4)

    def test_parse_output_error_envelope(self):
        envelope = {"is_error": True, "subtype": "error_max_turns", "result": "ran out", "num_turns": 101,
                    "total_cost_usd": 3.75, "usage": {"input_tokens": 12_000_000}}
        result = claude_worker.parse_output(envelope_json(envelope), {"max_turns": 100})
        self.assertFalse(result.ok)
        self.assertIn("error_max_turns", result.error)
        self.assertIn("max_turns bound at 100", result.error)
        self.assertIn("101 turn(s)", result.error)
        self.assertIn("12.0M input tokens", result.error)

    def test_argv_drops_the_dollar_cap_a_subscription_does_not_meter(self):
        subscription = account.Account(logged_in=True, auth_method="claude.ai", api_provider="firstParty", plan="pro", detected=True)
        api_key = account.Account(logged_in=True, auth_method="apiKey", api_provider="firstParty", detected=True)
        undetected = account.Account()
        base_spec = {"provider": "claude", "model": "sonnet", "max_budget_usd": 2.0}
        for acc, expected in ((subscription, False), (api_key, True), (undetected, True)):
            spec = account.apply_to_spec(base_spec, "standard", acc)
            brief = Brief(role="builder", system="s", prompt="p", cwd=".", spec=spec)
            joined = " ".join(claude_worker.build_argv(brief))
            self.assertEqual("--max-budget-usd" in joined, expected, acc.metered)
            self.assertIn("--max-turns", joined)

    def test_parse_output_non_json(self):
        self.assertFalse(claude_worker.parse_output("boom").ok)

    def test_invoke_uses_stdin_and_env(self):
        brief = Brief(role="builder", system="s", prompt="the prompt", cwd=".", spec={"model": "sonnet"}, env={"PATH": os.environ.get("PATH", "")})
        events = [
            {"type": "assistant", "message": {"usage": {"input_tokens": 10, "output_tokens": 2}}},
            {"type": "result", "is_error": False, "result": "{}", "structured_output": {"a": 1}},
        ]
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.Popen", return_value=FakeProcess(events)) as popen:
            result = claude_worker.ClaudeWorker().run(brief)
        self.assertTrue(result.ok)
        self.assertEqual(result.provider, "claude")
        self.assertEqual(popen.call_args.kwargs["env"], brief.env)
        self.assertEqual(popen.call_args.kwargs["cwd"], ".")

    def test_a_killed_worker_keeps_its_partial_accounting(self):
        brief = Brief(role="builder", system="s", prompt="p", cwd=".", spec={"model": "sonnet"}, timeout_s=1)
        events = [
            {"type": "assistant", "message": {"usage": {"input_tokens": 400_000, "cache_read_input_tokens": 100_000, "output_tokens": 900}}},
            {"type": "assistant", "message": {"usage": {"input_tokens": 600_000, "output_tokens": 1_100}}},
            {"type": "rate_limit_event", "rate_limit_info": {"unifiedWindows": {"five_hour": {"utilization": 0.4, "resetsAt": 1}}}},
        ]
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.Popen", return_value=FakeProcess(events, kill_before_result=True)):
            result = claude_worker.ClaudeWorker().run(brief)
        self.assertFalse(result.ok)
        self.assertEqual(result.turns, 2)
        self.assertEqual(result.input_tokens, 1_100_000)
        self.assertEqual(result.output_tokens, 2_000)
        self.assertIn("1.1M input tokens", result.error)
        self.assertEqual(result.rate_limit["unifiedWindows"]["five_hour"]["utilization"], 0.4)

    def test_rate_limit_event_reaches_the_result(self):
        brief = Brief(role="builder", system="s", prompt="p", cwd=".", spec={"model": "sonnet"})
        events = [
            {"type": "rate_limit_event", "rate_limit_info": {"status": "allowed_warning", "unifiedWindows": {"seven_day": {"utilization": 0.82}}}},
            {"type": "result", "is_error": False, "result": "done", "num_turns": 3},
        ]
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.Popen", return_value=FakeProcess(events)):
            result = claude_worker.ClaudeWorker().run(brief)
        self.assertTrue(result.ok)
        self.assertEqual(result.rate_limit["status"], "allowed_warning")

    def test_missing_binary_is_soft_failure(self):
        brief = Brief(role="builder", system="s", prompt="p", cwd=".", spec={})
        with mock.patch("shutil.which", return_value=None):
            result = claude_worker.ClaudeWorker().run(brief)
        self.assertFalse(result.ok)
        self.assertIn("not on PATH", result.error)

    def test_worker_for_unknown_provider(self):
        with self.assertRaises(ValueError):
            base.worker_for("gemini")


if __name__ == "__main__":
    unittest.main()
