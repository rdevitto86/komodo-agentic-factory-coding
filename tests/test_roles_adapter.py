import json
import os
import tempfile
import unittest

from komodo import adapters, roles, standards
from komodo.adapters import claude
from komodo.config import Config

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class RoleTests(unittest.TestCase):
    def test_every_role_parses_with_a_purpose_and_tier(self):
        for name in roles.available():
            role = roles.load(name)
            self.assertTrue(role.purpose, name)
            self.assertIn(role.tier, roles.TIERS)
            self.assertIn(role.access, roles.ACCESS)

    def test_sections_split(self):
        role = roles.load("builder")
        self.assertIn("Boundaries", role.body)
        self.assertNotIn("## Worker output", role.body)
        self.assertIn("JSON object", role.worker_output)
        self.assertIn("## Result", role.session_output)
        self.assertIn("# Output", role.system_prompt())
        self.assertNotIn("JSON object", role.session_body())

    def test_worker_only_roles_are_not_session_agents(self):
        for name in ("merger", "responder", "summarizer"):
            self.assertFalse(roles.load(name).session, name)
        for name in ("builder", "reviewer", "scout"):
            self.assertTrue(roles.load(name).session, name)

    def test_bad_tier_rejected(self):
        with self.assertRaises(roles.RoleError):
            roles.parse("---\nname: x\ntier: huge\n---\nbody\n", "x")


class AdapterTests(unittest.TestCase):
    def test_render_claude_layout(self):
        with tempfile.TemporaryDirectory() as target:
            written = adapters.render("claude", target, Config.load(REPO))
            self.assertIn("AGENTS.md", written)
            self.assertIn(os.path.join("agents", "builder.md"), written)
            self.assertNotIn(os.path.join("agents", "merger.md"), written)
            self.assertIn(os.path.join("skills", "komodo", "SKILL.md"), written)
            self.assertIn(os.path.join("skills", "standards-go", "SKILL.md"), written)
            self.assertIn(os.path.join("hooks", "guard.py"), written)
            self.assertIn(os.path.join("hooks", "context_injector.py"), written)
            self.assertEqual(os.path.join("hooks", claude.BINARY) in written, bool(claude.host_binary()))
            self.assertIn(os.path.join("standards", "rust.md"), written)
            with open(os.path.join(target, "agents", "reviewer.md"), encoding="utf-8") as handle:
                reviewer = handle.read()
            self.assertIn("model: sonnet", reviewer)
            self.assertIn("tools: Read, Grep, Glob, Bash", reviewer)
            self.assertIn("Sev | File:line", reviewer)
            self.assertNotIn("JSON object", reviewer)
            with open(os.path.join(target, "agents", "researcher.md"), encoding="utf-8") as handle:
                self.assertIn("WebFetch", handle.read())
            with open(os.path.join(target, "agents", "builder.md"), encoding="utf-8") as handle:
                self.assertNotIn("{{", handle.read())
            with open(os.path.join(target, "settings.policy.json"), encoding="utf-8") as handle:
                policy = json.load(handle)
            self.assertEqual(policy["skillOverrides"]["standards-go"], "name-only")
            self.assertEqual(policy["skillOverrides"]["komodo"], "name-only")
            self.assertIn("permissions", policy)
            self.assertEqual(set(os.listdir(os.path.join(target, "seeds"))), {"CLAUDE.local.md", "settings.local.json"})

    def test_profile_changes_rendered_models(self):
        with tempfile.TemporaryDirectory() as target:
            adapters.render("claude", target, Config.load(REPO, {"profile": "thinking"}))
            with open(os.path.join(target, "agents", "reviewer.md"), encoding="utf-8") as handle:
                self.assertIn("model: opus", handle.read())

    def test_every_standard_gets_a_skill(self):
        with tempfile.TemporaryDirectory() as target:
            adapters.render("claude", target)
            rendered = {name[len("standards-"):] for name in os.listdir(os.path.join(target, "skills")) if name.startswith("standards-")}
            self.assertEqual(rendered, set(standards.available()))

    def test_unknown_adapter(self):
        with self.assertRaises(ValueError):
            adapters.render("gemini", "/tmp/nowhere")


if __name__ == "__main__":
    unittest.main()
