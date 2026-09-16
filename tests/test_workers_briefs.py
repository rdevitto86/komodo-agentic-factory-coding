import json
import os
import unittest
from unittest import mock

from komodo import briefs
from komodo.workers import Brief, base
from komodo.workers import claude as claude_worker

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
        self.assertEqual(briefs.tools_for("reviewer"), ["Read", "Grep", "Glob"])
        self.assertEqual(briefs.tools_for("summarizer"), [])
        self.assertIn("Edit", briefs.tools_for("builder"))


class ClaudeArgvTests(unittest.TestCase):
    def test_argv_carries_every_spec_flag(self):
        brief = Brief(role="builder", system="sys", prompt="p", cwd=".", spec={"model": "sonnet", "effort": "high", "max_turns": 7, "max_budget_usd": 1.5}, schema={"type": "object"}, tools=["Read", "Bash"])
        argv = claude_worker.build_argv(brief)
        joined = " ".join(argv)
        for flag in ("--model sonnet", "--effort high", "--max-turns 7", "--max-budget-usd 1.5", "--tools Read,Bash", "--json-schema", "--setting-sources project", "--no-session-persistence", "--dangerously-skip-permissions", "--output-format json"):
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
        result = claude_worker.parse_output(json.dumps({"is_error": True, "subtype": "error_max_turns", "result": "ran out"}))
        self.assertFalse(result.ok)
        self.assertIn("error_max_turns", result.error)

    def test_parse_output_non_json(self):
        self.assertFalse(claude_worker.parse_output("boom").ok)

    def test_invoke_uses_stdin_and_env(self):
        brief = Brief(role="builder", system="s", prompt="the prompt", cwd=".", spec={"model": "sonnet"}, env={"PATH": os.environ.get("PATH", "")})
        fake = mock.Mock(returncode=0, stdout=json.dumps({"is_error": False, "result": "{}", "structured_output": {"a": 1}}), stderr="")
        with mock.patch("shutil.which", return_value="/bin/claude"), mock.patch("subprocess.run", return_value=fake) as run:
            result = claude_worker.ClaudeWorker().run(brief)
        self.assertTrue(result.ok)
        self.assertEqual(result.provider, "claude")
        kwargs = run.call_args.kwargs
        self.assertEqual(kwargs["input"], "the prompt")
        self.assertEqual(kwargs["env"], brief.env)

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
